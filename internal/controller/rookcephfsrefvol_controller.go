/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RookCephFSRefVolReconciler reconciles a RookCephFSRefVol object
type RookCephFSRefVolReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var (
	finalizerName = "operator.dotv.home.arpa/finalizer"
	// pvOwnerKey     = ".metadata.controller"
	controllerName = "RookCephFSController"
)

// +kubebuilder:rbac:groups=operator.dotv.home.arpa,resources=rookcephfsrefvols,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.dotv.home.arpa,resources=rookcephfsrefvols/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.dotv.home.arpa,resources=rookcephfsrefvols/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumes/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RookCephFSRefVol object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
//   - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
//     This function is where we define how our Operator should react to these events
//
// a nd take corrective measures to ensure the actual state matches the desired state defined in the object

// ctx context.Context: được dùng phổ biến trong go để kiểm soát các hàm cần nhiều thời gian để xử lý. có thể dùng để handle timeout
//
//	hoặc huỷ các task chạy lâu
//
// req ctrl.Request	  : chưa thông về đối tượng chưa thông tin về sự kiện mà trigger cái event cho
func (r *RookCephFSRefVolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("RookCephFSRefVolReconciler")
	rookcephfsrefvol := &operatorv1.RookCephFSRefVol{}

	// fetch rookcephfsrefvol
	if err := r.Get(ctx, req.NamespacedName, rookcephfsrefvol); err != nil {

		if apierrors.IsNotFound(err) {
			log.Info("[RookCephFSRefVol] Not found", "Name", rookcephfsrefvol.Name)
			return ctrl.Result{}, nil
		}
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		log.Error(err, "[RookCephFSRefVol] failed to get RookCephFSRefVol")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle delete ###
	// examine DeletionTimestamp to determine if object is under deletion
	if rookcephfsrefvol.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(rookcephfsrefvol, finalizerName) {
			controllerutil.AddFinalizer(rookcephfsrefvol, finalizerName)
			if err := r.Update(ctx, rookcephfsrefvol); err != nil {
				log.Error(err, "Gặp lỗi trong quá trình thêm finalizers của tài nguyên RookCephFSVol")
				return ctrl.Result{}, err
			}
		}
	}

	refVolume, err := r.getRefVolume(ctx, log, rookcephfsrefvol)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.updateStatus(ctx, log, rookcephfsrefvol, refVolume)
	// handle deletetion
	if !rookcephfsrefvol.ObjectMeta.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(rookcephfsrefvol, finalizerName) {
		// if the object is being deleted
		log.Info("Có yêu cầu xoá! Proceeding to cleanup the finalizers...")
		if controllerutil.ContainsFinalizer(rookcephfsrefvol, finalizerName) {
			// Thực hiện xoá
			if err := r.onDelete(ctx, log, rookcephfsrefvol, refVolume); err != nil {
				log.Error(err, "Gặp lỗi trong quá trình  tài nguyên RookCephFSVol")
				return ctrl.Result{Requeue: true}, err
			}

			if err := r.Update(ctx, rookcephfsrefvol); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Tạo PV nếu chưa tạo
	if rookcephfsrefvol.Status.State == operatorv1.Missing {
		fmt.Print(rookcephfsrefvol.Status.State)
		log.Info("Bắt đầu quá trình thực hiện tạo Pv cho RookCephFSRefVol")
		if err := r.createRefVolume(ctx, log, rookcephfsrefvol); err != nil {
			if err := r.writeInstance(ctx, log, rookcephfsrefvol); err != nil {
				log.Error(err, "while setting rookcephfsrefvol state", "state", operatorv1.Missing, "reason", err)
			}
			return ctrl.Result{}, err
		}
	} // update reconcile code

	// UPDATE CR Status

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RookCephFSRefVolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	updatePred := predicate.Funcs{
		// Only allow updates when the spec.size of the Busybox resource changes
		UpdateFunc: func(e event.UpdateEvent) bool {
			// oldObj := e.ObjectOld.(*examplecomv1alpha1.Busybox)
			// newObj := e.ObjectNew.(*examplecomv1alpha1.Busybox)

			// Trigger reconciliation only if the spec.size field has changed
			val, ok := e.ObjectNew.GetAnnotations()[operatorv1.IsParent]
			println(e.ObjectNew.GetDeletionTimestamp().IsZero())
			println(ok && val == "true")
			println(ok && val == "true" && !e.ObjectNew.GetDeletionTimestamp().IsZero())
			return ok && val == "true" && !e.ObjectNew.GetDeletionTimestamp().IsZero()
		},

		// Allow create events
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},

		// Allow delete events
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
			// pv := e.Object.GetAnnotations()

		},

		// Allow generic events (e.g., external triggers)
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.RookCephFSRefVol{}). //specifies the type of resource to watch
		Owns(&corev1.PersistentVolume{}).
		Watches(
			&corev1.PersistentVolume{},
			handler.EnqueueRequestsFromMapFunc(r.filterParent),
			builder.WithPredicates(updatePred),
		).
		// Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

func (r *RookCephFSRefVolReconciler) filterParent(ctx context.Context, pv client.Object) []ctrl.Request {
	childrenList, err := r.fetchRefVolumeList(ctx, pv.GetName())

	if err != nil && apierrors.IsNotFound(err) {
		return []ctrl.Request{}
	}

	reqs := make([]ctrl.Request, 0, len(childrenList))
	println("filterParent triggered - PV deleted:", pv.GetName()) // Debug log
	// fmt -- trig
	for _, item := range childrenList {
		reqs = append(reqs, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name: item.GetName(),
			},
		})
	}
	return reqs
}

func (r *RookCephFSRefVolReconciler) fetchRefVolumeList(ctx context.Context, parentPvName string) ([]operatorv1.RookCephFSRefVol, error) {
	var matchingCRs []operatorv1.RookCephFSRefVol

	var rookCephFSRefVolList operatorv1.RookCephFSRefVolList
	if err := r.List(ctx, &rookCephFSRefVolList); err != nil {
		return nil, err
	}

	for _, cr := range rookCephFSRefVolList.Items {
		if cr.Status.Parent == parentPvName { // Kiểm tra điều kiện trong status
			matchingCRs = append(matchingCRs, cr)
		}
	}
	return matchingCRs, nil
}

// func generateRandomString() string {
// 	rand.Seed(uint64(time.Now().UnixNano()))
// 	b := make([]byte, 6)
// 	for i := range b {
// 		b[i] = charset[rand.Intn(len(charset))]
// 	}
// 	return string(b)
// }

func (r *RookCephFSRefVolReconciler) buildRefVolumeManifest(originalPv *corev1.PersistentVolume, rookCephFSRefVol *operatorv1.RookCephFSRefVol) *corev1.PersistentVolume {
	newPvPrefix := "-" + rookCephFSRefVol.ObjectMeta.Name + "-" + rookCephFSRefVol.ObjectMeta.Namespace
	newPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rookCephFSRefVol.Name + "-" + rookCephFSRefVol.Spec.Namespace + "-" + rookCephFSRefVol.Spec.PvcName,
			Annotations: make(map[string]string),
			Labels:      make(map[string]string),
			// {
			// 	"parent":     originalPv.Name,
			// 	"created-by": controllerName,
			// },
		},
		Spec: *originalPv.Spec.DeepCopy(),
	}
	if newPV.Spec.PersistentVolumeSource.CSI != nil {
		newPV.Spec.ClaimRef = nil
		newPV.Spec.PersistentVolumeSource.CSI.NodeStageSecretRef.Name = rookCephFSRefVol.Spec.CephFsUserSecretName
		newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["staticVolume"] = "true"
		newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["rootPath"] = newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["subvolumePath"]
		newPV.Spec.PersistentVolumeSource.CSI.VolumeHandle = newPV.Spec.PersistentVolumeSource.CSI.VolumeHandle + newPvPrefix
		newPV.Spec.PersistentVolumeReclaimPolicy = "Retain"
	}

	newPV.ObjectMeta.Annotations = annotationMapping(originalPv, rookCephFSRefVol)
	for k, v := range originalPv.GetLabels() {
		newPV.ObjectMeta.Labels[k] = v
	}
	// set parent child ownership
	// fmt.Print(newPV)
	controllerutil.SetControllerReference(rookCephFSRefVol, newPV, r.Scheme, controllerutil.WithBlockOwnerDeletion(true))
	// controllerutil.SetOwnerReference(rookCephFSRefVol, newPV, r.Scheme, controllerutil.WithBlockOwnerDeletion(true))
	return newPV
}

func (r *RookCephFSRefVolReconciler) ownObject(ctx context.Context, cr *operatorv1.RookCephFSRefVol, obj client.Object) error {

	err := ctrl.SetControllerReference(cr, obj, r.Scheme)
	if err != nil {
		return err
	}
	return nil
}

func (r *RookCephFSRefVolReconciler) createRefVolume(ctx context.Context, log logr.Logger, rookCephFsRefVol *operatorv1.RookCephFSRefVol) error {
	pv, shouldCreate := r.shouldCreateRefVol(ctx, log, rookCephFsRefVol)

	if shouldCreate {
		desiredPv := r.buildRefVolumeManifest(pv, rookCephFsRefVol)
		err := r.ownObject(ctx, rookCephFsRefVol, desiredPv)

		if err != nil {
			log.Error(err, "Err While setting controller ref")
			return err
		}
		// log.Info("Creating RefVolume", rookCephFsRefVol.Name)
		if err := r.Create(ctx, desiredPv); err != nil {
			log.Error(err, "Err While creating Refvolume")
			rookCephFsRefVol.Status.State = operatorv1.Missing
			return r.Status().Update(ctx, rookCephFsRefVol)
		}
	}
	rookCephFsRefVol.Status.State = operatorv1.Ok
	rookCephFsRefVol.Status.Parent = pv.GetName()
	pv.GetAnnotations()[operatorv1.IsParent] = "true"
	// add finalizer in case cascading deleting or should I ? because, the data is being shared between PVs fate should be
	// but for save we will add it for now
	controllerutil.AddFinalizer(pv, finalizerName)
	r.Update(ctx, pv)

	return r.Status().Update(ctx, rookCephFsRefVol)
}

func annotationMapping(originalPv *corev1.PersistentVolume, sourceRookCephRefVolObj *operatorv1.RookCephFSRefVol) map[string]string {
	annotations := make(map[string]string)
	for k, v := range originalPv.GetAnnotations() {
		if !strings.HasPrefix(k, operatorv1.MetaGroup) {
			annotations[k] = v
		}

	}
	annotations[operatorv1.CreatedBy] = sourceRookCephRefVolObj.Name
	annotations[operatorv1.Parent] = originalPv.Name

	return annotations
}

func (r *RookCephFSRefVolReconciler) onDelete(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolume) error {

	// IsDeletingCRD, errr := crd.
	switch {
	case r.shouldDeleteRefVol(ctx, log, rookCephFSRefVol, refVolume):
		log.Info("Deleting refVolume due to CR being deleted")
		return r.deletePersistentVolume(ctx, log, refVolume)
	case r.shouldRemoveFinalizer(log, rookCephFSRefVol, refVolume):
		log.Info("Remove finalizer from Crs")
		controllerutil.RemoveFinalizer(rookCephFSRefVol, finalizerName)
		return r.writeInstance(ctx, log, rookCephFSRefVol)
	default:
		if controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
			log.Info("Waiting for refVolume to be fully purged before letting the CR be deleted")
		} else {
			// I doubt we'll ever get here but I suppose it's possible
			log.Info("Waiting for K8s to delete this CR (all finalizers are removed)")
		}
		return nil
		// case operatorv1.Conflict:
		// case operatorv1.Missing:
	}

}

func (r *RookCephFSRefVolReconciler) shouldDeleteRefVol(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolume) bool {
	switch rookCephFSRefVol.Status.State {
	case operatorv1.Ok:
		log.Info("enter check delete")
		// RefVol và Crs được bound với nhau, ok để xoá
		// 1. Nếu nó đang xoá ==> ko cần xoá lại,
		// 2. Nếu CRs không có finalizer --> PV đã xoá thành công --> k cần phải xoá lại
		if !refVolume.DeletionTimestamp.IsZero() {
			log.Info("The RefVolume is being deleted, no need to delete it again", rookCephFSRefVol.Name)
			return false
		}

		if !controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
			log.Info("The Crs is already finalized, no need to delete again", rookCephFSRefVol.Name)
			return false
		}
		// isParentDeleting := r.shouldTriggerDelete(ctx, refVolume)
		return true
	case operatorv1.Conflict:
		// Trường hợp confilct, không có pv nào đc bound --> k xoá
		log.Info("This anchor is in conflict state ---> no PV will be deleted", rookCephFSRefVol.Name)
		return false
	case operatorv1.Missing:
		log.Info("PV is deleted, no need to delete again")
		return false
	default:
		return false
	}

}

func (r *RookCephFSRefVolReconciler) shouldCreateRefVol(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol) (*corev1.PersistentVolume, bool) {
	persistentVolumeClaim, err := r.getPersistentVolumeClaim(ctx, log, rookCephFSRefVol)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Err when get PersistentClaim")
			return nil, false
		}
		log.Info("PeristenVolumeClaim", rookCephFSRefVol.Spec.PvcName, "Namespace", rookCephFSRefVol.Namespace, "is not existed")
		return nil, false
	}
	pv, err := r.getSourcePersistentVolume(ctx, log, persistentVolumeClaim.Spec.VolumeName)

	if err != nil {
		log.Info("Err when get PersistentVolume", rookCephFSRefVol.Spec.PvcName, "Namespace", rookCephFSRefVol.Namespace, "is not existed")
		return nil, false
	}

	if !pv.DeletionTimestamp.IsZero() {
		log.Info("ParentPv is deleting", rookCephFSRefVol.Status.Parent, "Namespace", rookCephFSRefVol.Namespace, "is not existed")

		return nil, false
	}

	return pv, true
}

func (r *RookCephFSRefVolReconciler) shouldRemoveFinalizer(log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolume) bool {
	// If the anchor is already finalized, there's no need to do it again.
	if !controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
		return false
	}
	switch rookCephFSRefVol.Status.State {
	case operatorv1.Ok:
		// RefVol và Crs được bound với nhau
		// Do đã gọi hàm shouldDeleteRefVol
		if refVolume.DeletionTimestamp.IsZero() {
			log.Info("refVolume will not be deleted, allow cr to be finalized")
			return true
		}
		log.Info("RefVolume is being deleted; cannot finalize cr yet")
		return false
	case operatorv1.Conflict:
		// Trường hợp confilct, không có pv nào đc bound --> k xoá
		log.Info("This anchor is in conflict state ---> no PV will be deleted, RookCephFsRefVol:")
		return true
	case operatorv1.Missing:
		log.Info("PV is deleted, no need to delete again")
		return true
	default:
		return true
	}
}
func (r *RookCephFSRefVolReconciler) getPersistentVolumeClaim(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol) (*corev1.PersistentVolumeClaim, error) {
	persistentVolumeClaim := &corev1.PersistentVolumeClaim{}

	// var PersistentVolume corev1.PersistentVolume

	if err := r.Get(ctx, client.ObjectKey{
		Namespace: rookCephFSRefVol.Spec.Namespace,
		Name:      rookCephFSRefVol.Spec.PvcName,
	}, persistentVolumeClaim); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return &corev1.PersistentVolumeClaim{}, err
	}
	return persistentVolumeClaim, nil
}

func (r *RookCephFSRefVolReconciler) getRefVolume(ctx context.Context, log logr.Logger, rookCephFSRefVolume *operatorv1.RookCephFSRefVol) (*corev1.PersistentVolume, error) {
	persistentVolume := &corev1.PersistentVolume{}

	// var PersistentVolume corev1.PersistentVolume

	if err := r.Get(ctx, client.ObjectKey{
		Name: rookCephFSRefVolume.Name + "-" + rookCephFSRefVolume.Spec.Namespace + "-" + rookCephFSRefVolume.Spec.PvcName,
	}, persistentVolume); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "unable to fetch source PersistentVolumeClaim")
			return nil, err
		}
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return &corev1.PersistentVolume{}, nil
	}

	return persistentVolume, nil
}

func (r *RookCephFSRefVolReconciler) shouldTriggerDelete(ctx context.Context, refVolume *corev1.PersistentVolume) bool {

	if refVolume.GetName() == "" {
		return false
	}
	parentPV := &corev1.PersistentVolume{}

	if err := r.Get(ctx, client.ObjectKey{
		Name: refVolume.GetAnnotations()[operatorv1.Parent],
	}, parentPV); err != nil {
		return false
	}
	return parentPV.DeletionTimestamp.IsZero()
}

func (r *RookCephFSRefVolReconciler) getSourcePersistentVolume(ctx context.Context, log logr.Logger, parentPvName string) (*corev1.PersistentVolume, error) {
	persistentVolume := &corev1.PersistentVolume{}

	if err := r.Get(ctx, client.ObjectKey{
		Name: parentPvName,
	}, persistentVolume); err != nil {
		return &corev1.PersistentVolume{}, err
	}

	return persistentVolume, nil
}

func (r *RookCephFSRefVolReconciler) deletePersistentVolume(ctx context.Context, log logr.Logger, inst *corev1.PersistentVolume) error {
	if err := r.Delete(ctx, inst); err != nil {
		log.Error(err, "While deleting ref volume")
		return err
	}
	return nil
}

// func (r *RookCephFSRefVolReconciler) isRootPv(ctx context.Context, log logr.Logger, rookcephfsRefVol *operatorv1.RookCephFSRefVol) error {
// 	state := rookcephfsRefVol.Status.State
// 	switch state {
// 	case operatorv1.Ok:
// 		// PV đã được tạo và bound với CR
// 		// Thực hiện xem có nên xoá PV k

// 	}
// 	return nil
// }

func (r *RookCephFSRefVolReconciler) updateStatus(ctx context.Context, log logr.Logger, rookcephfsRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolume) {

	// refVolumeParent := refVolume.Annotations[operatorv1.Parent]

	switch {
	case refVolume.Name == "":
		// PV chưa được tạo
		log.Info("PV associate with RookCephRefVol CRs: ", rookcephfsRefVol.Name, ", for Namespace: ", rookcephfsRefVol.Spec.Namespace, " is not created, start create one!!!")
		rookcephfsRefVol.Status.State = operatorv1.Missing
	// case rookcephfsRefVol.Name != createdBy:
	// 	log.Info("Conflicting When creating Pv, maybe the pv is already created by others", rookcephfsRefVol.Name)
	// 	rookcephfsRefVol.Status.State = operatorv1.Conflict
	default:
		if rookcephfsRefVol.Status.State != operatorv1.Ok {
			log.Info("Refvolume is already sucessfully created", "prev state", rookcephfsRefVol.Status.State)
		}
		rookcephfsRefVol.Status.State = operatorv1.Ok
	}
}

func (r *RookCephFSRefVolReconciler) writeInstance(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol) error {
	if rookCephFSRefVol.CreationTimestamp.IsZero() {
		if err := r.Create(ctx, rookCephFSRefVol); err != nil {
			log.Error(err, "while creating on apiserver")
			return err
		}
	} else {
		if err := r.Update(ctx, rookCephFSRefVol); err != nil {
			log.Error(err, "while updating on apiserver")
			return err
		}
	}
	return nil
}
