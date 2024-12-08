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
	"strings"
	"time"

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
	"k8s.io/client-go/util/retry"
)

// const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RookCephFSRefVolReconciler reconciles a RookCephFSRefVol object
type RookCephFSRefVolReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var (
	finalizerName  = "operator.dotv.home.arpa/finalizer"
	controllerName = "RookCephFSController"
	pvIndexKey     = ".metadata.annotations." + operatorv1.Parent
	crIndexKey     = ".status.parentPersistentVolume"
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
			log.Info("[RookCephFSRefVol] has been deleted", "Name", req.Name)
			return ctrl.Result{}, nil
		}
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		log.Error(err, "[RookCephFSRefVol] failed to get RookCephFSRefVol")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if rookcephfsrefvol.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(rookcephfsrefvol, finalizerName) && rookcephfsrefvol.Status.State != operatorv1.ParentNotFound {
			controllerutil.AddFinalizer(rookcephfsrefvol, finalizerName)
			if err := r.Update(ctx, rookcephfsrefvol); err != nil {
				log.Error(err, "Gặp lỗi trong quá trình thêm finalizers của tài nguyên RookCephFSVol")
				return ctrl.Result{}, err
			}
		}
	}
	// Có thể cần có cơ chế hay hơn cho việc lấy theo status
	refVolume, err := r.getRefVolume(ctx, log, rookcephfsrefvol)
	if err != nil {
		return ctrl.Result{}, err
	}

	targetPVC := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, client.ObjectKey{
		Name:      rookcephfsrefvol.Spec.PvcName,
		Namespace: rookcephfsrefvol.Spec.Namespace,
	}, targetPVC); err != nil && !apierrors.IsNotFound(err) {

		return ctrl.Result{}, err
	}

	parentPV := &corev1.PersistentVolume{}

	if err := r.Get(ctx, client.ObjectKey{
		Name: targetPVC.Spec.VolumeName,
	}, parentPV); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	r.updateState(ctx, log, rookcephfsrefvol, refVolume, parentPV, targetPVC)

	// handle deletetion
	if (!rookcephfsrefvol.ObjectMeta.DeletionTimestamp.IsZero() && controllerutil.ContainsFinalizer(rookcephfsrefvol, finalizerName)) || rookcephfsrefvol.Status.State == operatorv1.IsDeleting {

		// if the object is being deleted
		log.Info("Có yêu cầu xoá! Proceeding to cleanup the finalizers...")
		if controllerutil.ContainsFinalizer(rookcephfsrefvol, finalizerName) {
			// Thực hiện xoá
			if err := r.onDelete(ctx, log, rookcephfsrefvol, refVolume); err != nil {
				log.Error(err, "Gặp lỗi trong quá trình xoá tài nguyên RookCephFSVol")
				return ctrl.Result{Requeue: true}, err
			}
		}

		// handle remove parent finalizer
		if rookcephfsrefvol.Status.State == operatorv1.IsDeleting || (controllerutil.ContainsFinalizer(parentPV, finalizerName) && controllerutil.ContainsFinalizer(targetPVC, finalizerName)) {
			log.Info("Check If we can remove finalizer on parent PV")
			// Update ParentPVC
			if err := r.Get(ctx, client.ObjectKey{
				Name:      rookcephfsrefvol.Spec.PvcName,
				Namespace: rookcephfsrefvol.Spec.Namespace,
			}, targetPVC); err != nil && !apierrors.IsNotFound(err) {

				return ctrl.Result{}, err
			}

			if err := r.Get(ctx, client.ObjectKey{
				Name: targetPVC.Spec.VolumeName,
			}, parentPV); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}

			var pvList corev1.PersistentVolumeList
			var rookCephFsRefVolList operatorv1.RookCephFSRefVolList
			if err := r.List(ctx, &pvList, client.MatchingFields{pvIndexKey: parentPV.Name}); err != nil {
				log.Error(err, "unable to list child Jobs")
				return ctrl.Result{}, err
			}
			if err := r.List(ctx, &rookCephFsRefVolList, client.MatchingFields{crIndexKey: parentPV.Name}); err != nil {
				log.Error(err, "unable to list child Jobs")
				return ctrl.Result{}, err
			}
			println("List size: ", len(pvList.Items))
			if len(pvList.Items) == 0 {
				log.Info("Parent dont have any child left safe to finalized")
				controllerutil.RemoveFinalizer(parentPV, finalizerName)
				retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					// Perform the patch or update operation
					return r.Update(ctx, parentPV)
				})
				// if err := r.Update(ctx, parentPV); err != nil {
				// 	log.Error(err, "Failed to update parent PV after finalizer removal")
				// 	return ctrl.Result{}, err
				// }
				controllerutil.RemoveFinalizer(targetPVC, finalizerName)
				retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					// Perform the patch or update operation
					return r.Update(ctx, targetPVC)
				})
			}
		}
	}

	//early exist trong trường hợp là parent not found:
	if rookcephfsrefvol.Status.State == operatorv1.ParentNotFound {
		log.Info("Can't found parent, removing finalizer")
		controllerutil.RemoveFinalizer(rookcephfsrefvol, finalizerName)
		if rookcephfsrefvol.Status.Children != "" {
			if err := r.deleteObject(ctx, log, rookcephfsrefvol); err != nil {
				return ctrl.Result{RequeueAfter: time.Second * 30}, r.writeInstance(ctx, log, rookcephfsrefvol)

			}
		}
		return ctrl.Result{}, r.writeInstance(ctx, log, rookcephfsrefvol)
	}

	// Tạo PV nếu chưa tạo
	if rookcephfsrefvol.Status.State == operatorv1.Missing {
		log.Info("Bắt đầu quá trình thực hiện tạo Pv cho RookCephFSRefVol")
		if err := r.createRefVolume(ctx, log, rookcephfsrefvol, parentPV, targetPVC); err != nil {
			if err := r.writeInstance(ctx, log, rookcephfsrefvol); err != nil {
				log.Error(err, "while setting rookcephfsrefvol state", "state", operatorv1.Missing, "reason", err)
			}
			return ctrl.Result{}, err
		}
	}

	// Reconcile PV

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RookCephFSRefVolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.PersistentVolume{}, pvIndexKey, func(rawObj client.Object) []string {
		pv := rawObj.(*corev1.PersistentVolume)
		parent, ok := pv.GetAnnotations()["operator.dotv.home.arpa/parent"]
		if !ok {
			return nil
		}
		return []string{parent}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &operatorv1.RookCephFSRefVol{}, crIndexKey, func(rawObj client.Object) []string {
		rookCephFSRefVol := rawObj.(*operatorv1.RookCephFSRefVol)

		if rookCephFSRefVol.Status.Parent != "" {
			return []string{rookCephFSRefVol.Status.Parent}
		}

		// If state is empty, return nil so it won't be indexed
		return nil
	}); err != nil {
		return err
	}
	updatePred := predicate.Funcs{
		// Only allow updates when the spec.size of the Busybox resource changes
		UpdateFunc: func(e event.UpdateEvent) bool {
			val, ok := e.ObjectNew.GetAnnotations()[operatorv1.IsParent]
			// Kiểm tra nếu là đối tượng PersistentVolumeClaim (PVC)
			if pvcNew, okPvc := e.ObjectNew.(*corev1.PersistentVolumeClaim); okPvc {
				pvcOld, okOldPvc := e.ObjectOld.(*corev1.PersistentVolumeClaim)

				// Kiểm tra điều kiện: volumeName thay đổi từ "" sang một giá trị mới
				if okOldPvc && pvcOld.Spec.VolumeName == "" && pvcNew.Spec.VolumeName != "" {
					return (ok && val == "true" && e.ObjectNew.GetDeletionTimestamp() != nil)
				}
			}
			return (ok && val == "true" && e.ObjectNew.GetDeletionTimestamp() != nil)
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
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(r.filterParent), builder.WithPredicates(updatePred)).
		// Watches(
		// 	&corev1.PersistentVolume{},
		// 	handler.EnqueueRequestsFromMapFunc(r.filterParent),
		// 	builder.WithPredicates(updatePred),
		// ).
		// Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

func (r *RookCephFSRefVolReconciler) filterParent(ctx context.Context, o client.Object) []ctrl.Request {
	var pvName string
	switch o := o.(type) {
	case *corev1.PersistentVolume:
		// If it's a PersistentVolume, use its name
		println("Parent PV is in terminating state, start delete RefVols", o.Name)
		pvName = o.GetName()
	case *corev1.PersistentVolumeClaim:
		println("Parent PVC is in terminating state, start delete RefVols", o.Name)
		// If it's a PersistentVolumeClaim, use its spec.VolumeName
		pvName = o.Spec.VolumeName
	default:
		// If it's neither, return an empty request list
		return []ctrl.Request{}
	}

	childrenList, err := r.fetchRefVolumeList(ctx, pvName)

	if err != nil && apierrors.IsNotFound(err) {
		return []ctrl.Request{}
	}

	reqs := make([]ctrl.Request, 0, len(childrenList))
	println("filterParent triggered - PV deleted:", pvName) // Debug log
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
		if cr.Status.Parent == parentPvName {
			matchingCRs = append(matchingCRs, cr)
		}
	}
	return matchingCRs, nil
}

func (r *RookCephFSRefVolReconciler) buildRefVolumeManifest(originalPv *corev1.PersistentVolume, rookCephFSRefVol *operatorv1.RookCephFSRefVol) *corev1.PersistentVolume {
	newPvPrefix := "-" + rookCephFSRefVol.ObjectMeta.Name + "-" + rookCephFSRefVol.ObjectMeta.Namespace
	newPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: rookCephFSRefVol.Name + "-" + rookCephFSRefVol.Spec.Namespace + "-" + rookCephFSRefVol.Spec.PvcName + "-",
			Annotations:  make(map[string]string),
			Labels:       make(map[string]string),
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

func (r *RookCephFSRefVolReconciler) ownObject(cr *operatorv1.RookCephFSRefVol, obj client.Object) error {

	err := ctrl.SetControllerReference(cr, obj, r.Scheme)
	if err != nil {
		return err
	}
	return nil
}

func (r *RookCephFSRefVolReconciler) createRefVolume(ctx context.Context, log logr.Logger, rookCephFsRefVol *operatorv1.RookCephFSRefVol, parentPV *corev1.PersistentVolume, parentPVC *corev1.PersistentVolumeClaim) error {

	if parentPV.DeletionTimestamp.IsZero() {
		desiredPv := r.buildRefVolumeManifest(parentPV, rookCephFsRefVol)
		err := r.ownObject(rookCephFsRefVol, desiredPv)

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
		rookCephFsRefVol.Status.State = operatorv1.Bounded
		rookCephFsRefVol.Status.Parent = parentPV.GetName()
		rookCephFsRefVol.Status.Children = desiredPv.Name

		annotations := parentPV.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		// Only add the annotation if it's not already set to "true"
		if currentValue, exists := annotations[operatorv1.IsParent]; !exists || currentValue != "true" {
			annotations[operatorv1.IsParent] = "true"
			parentPV.SetAnnotations(annotations)
		}

		pvcAnnotations := parentPVC.GetAnnotations()
		if pvcAnnotations == nil {
			pvcAnnotations = make(map[string]string)
		}

		// Only add the annotation if it's not already set to "true"
		if currentValue, exists := pvcAnnotations[operatorv1.IsParent]; !exists || currentValue != "true" {
			pvcAnnotations[operatorv1.IsParent] = "true"
			parentPVC.SetAnnotations(pvcAnnotations)
		}

		// Only add finalizers if not already added
		if !controllerutil.ContainsFinalizer(parentPV, finalizerName) {
			controllerutil.AddFinalizer(parentPV, finalizerName)
		}

		if !controllerutil.ContainsFinalizer(parentPVC, finalizerName) {
			controllerutil.AddFinalizer(parentPVC, finalizerName)
		}

		log.Info("Successfully created refVolume", "desiredPv", desiredPv.Name)

		if err := r.Update(ctx, parentPV); err != nil {
			log.Error(err, "Error while updating parent PV annotations")
			return err
		}

		if err := r.Update(ctx, parentPVC); err != nil {
			log.Error(err, "Error while updating parent PVC annotations")
			return err
		}

		r.Status().Update(ctx, rookCephFsRefVol)

		return nil
	}
	log.Info("Parent is being deleted, not creating ref volume")
	rookCephFsRefVol.Status.State = operatorv1.ParentDeleting
	rookCephFsRefVol.Status.Parent = parentPV.GetName()
	rookCephFsRefVol.Status.Children = ""
	return r.Status().Update(ctx, rookCephFsRefVol)
}

func annotationMapping(originalPv *corev1.PersistentVolume, sourceRookCephRefVolObj *operatorv1.RookCephFSRefVol) map[string]string {
	annotations := make(map[string]string)
	for k, v := range originalPv.GetAnnotations() {
		if !strings.HasPrefix(k, operatorv1.MetaGroup) {
			annotations[k] = v
		}

	}
	annotations[operatorv1.Owner] = sourceRookCephRefVolObj.Name
	annotations[operatorv1.Parent] = originalPv.Name
	annotations[operatorv1.CreatedBy] = operatorv1.ControllerName

	return annotations
}

func (r *RookCephFSRefVolReconciler) onDelete(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolume) error {

	// IsDeletingCRD, errr := crd.
	switch {
	case r.shouldDeleteRefVol(log, rookCephFSRefVol, refVolume):
		log.Info("Deleting refVolume due to CR being deleted")
		return r.deleteObject(ctx, log, refVolume)
	case r.shouldDeleteRookCephFsRefVolume(log, rookCephFSRefVol, refVolume):
		log.Info("Deleting rookCephFSRefVol due to CR being deleted")
		return r.deleteObject(ctx, log, rookCephFSRefVol)
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

func (r *RookCephFSRefVolReconciler) shouldDeleteRookCephFsRefVolume(log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolume) bool {

	if operatorv1.IsDeleting == rookCephFSRefVol.Status.State && refVolume.Name == "" && !rookCephFSRefVol.DeletionTimestamp.IsZero() {
		log.Info("RookcephFsRefVol state is being marked at deleting and no RefVol bound to it, should finailzed it!!")
		return false
	}
	println("true")
	return true
}

func (r *RookCephFSRefVolReconciler) shouldDeleteRefVol(log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolume) bool {
	// Kiểm tra nếu refVolume không tồn tại hoặc tên trống

	switch rookCephFSRefVol.Status.State {
	case operatorv1.IsDeleting:
		log.Info("Parent is deleting")
		if !controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
			log.Info("The CR is already finalized; no need to delete again", "name", rookCephFSRefVol.Name)
			return false
		}

		if refVolume.Name == "" {
			log.Info("No PV associate to this CRs, no need to start delete PV", "name", rookCephFSRefVol.Name)
			if rookCephFSRefVol.DeletionTimestamp.IsZero() {
				log.Info("RookCephFSRefvol state is being marked at deleting and no RefVol is bounding to it, Start deleting it", "name", rookCephFSRefVol.Name)
				return false

			}
			return false
		}
		return true

	case operatorv1.ParentNotFound:
		log.Info("Parent not found; no need to delete", "name", rookCephFSRefVol.Name)
		if rookCephFSRefVol.Status.Children != "" {
			return true
		}
		return false

	case operatorv1.Bounded:
		log.Info("Checking if deletion is needed")
		if !refVolume.DeletionTimestamp.IsZero() {
			log.Info("RefVolume is already being deleted; no need to delete again", "name", rookCephFSRefVol.Name)
			return false
		}
		if !controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
			log.Info("The CR is already finalized; no need to delete again", "name", rookCephFSRefVol.Name)
			return false
		}
		return true

	case operatorv1.Missing:
		log.Info("PV is missing; no need to delete again", "name", rookCephFSRefVol.Name)
		return false

	default:
		log.Info("Unhandled state; no action taken for deletion", "state", rookCephFSRefVol.Status.State)
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
		log.Info("ParentPv is deleting", rookCephFSRefVol.Status.Parent)
		return nil, false
	}

	return pv, true
}

func (r *RookCephFSRefVolReconciler) shouldRemoveFinalizer(log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolume) bool {
	// If the anchor is already finalized, there's no need to do it again.
	if !controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
		return false
	}
	// in anycases, if refVol still exist, we should not finalize the Crs
	if refVolume.Name != "" {
		return false
	}
	switch rookCephFSRefVol.Status.State {
	case operatorv1.Bounded:
		// RefVol và Crs được bound với nhau
		// Do đã gọi hàm shouldDeleteRefVol
		if refVolume.DeletionTimestamp.IsZero() {
			log.Info("refVolume will not be deleted, allow cr to be finalized")
			return true
		}
		log.Info("RefVolume is being deleted; cannot finalize cr yet")
		return false
	case operatorv1.IsDeleting:
		if refVolume.Name == "" {
			log.Info("The PV associate with RookCephFsRefVol is deleted allow cr to be finalized")
			return true
		}
		return false
	case operatorv1.ParentNotFound:
		log.Info("RookCephFSRefVol is in ParentNotFound state, safe to remove finalize")
		return true
	default:
		// Should never happen, so log an error and let it be deleted.
		// log.Error(errors.New("illegal state"), "Unknown state", "state", inst.Status.State)
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
		Name: rookCephFSRefVolume.Status.Children,
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

func (r *RookCephFSRefVolReconciler) deleteObject(ctx context.Context, log logr.Logger, obj client.Object) error {
	if err := r.Delete(ctx, obj); err != nil {
		log.Error(err, "While deleting", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
		return err
	}
	return nil
}

func (r *RookCephFSRefVolReconciler) RemoveFinalizer(ctx context.Context, log logr.Logger, rookCephFSRefVolume *operatorv1.RookCephFSRefVol) error {
	if err := r.Update(ctx, rookCephFSRefVolume); err != nil {
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

func (r *RookCephFSRefVolReconciler) updateState(ctx context.Context, log logr.Logger, rookcephfsRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolume, parentPV *corev1.PersistentVolume, parentPVC *corev1.PersistentVolumeClaim) {
	updatedState := rookcephfsRefVol.Status.State
	switch {
	case !rookcephfsRefVol.DeletionTimestamp.IsZero() || !parentPV.DeletionTimestamp.IsZero() || !parentPVC.DeletionTimestamp.IsZero():
		log.Info("RookCephFSRefVol is being deleted; marking state as IsDeleting")
		updatedState = operatorv1.IsDeleting
	case parentPV.Name == "" || parentPVC.Name == "":
		log.Info("Parent PV does not exist; marking RookCephFSRefVol as ParentNotFound")
		updatedState = operatorv1.ParentNotFound

		if rookcephfsRefVol.Status.Children != "" {
			// handle case bị bỏ lại orphan cuối cùng
			updatedState = operatorv1.IsDeleting
		}
	case refVolume.Name == "":
		log.Info("No PV is associated with RookCephFSRefVol; marking state as Missing",
			"RookCephFSRefVol", rookcephfsRefVol.Name, "Namespace", rookcephfsRefVol.Spec.Namespace)
		updatedState = operatorv1.Missing
	default:
		if rookcephfsRefVol.Status.State != operatorv1.Bounded {
			log.Info("RefVolume is successfully created; marking RookCephFSRefVol state as Bounded",
				"previous state", rookcephfsRefVol.Status.State)
		}
		updatedState = operatorv1.Bounded
	}

	// Cập nhật trạng thái nếu có sự thay đổi
	if rookcephfsRefVol.Status.State != updatedState {
		rookcephfsRefVol.Status.State = updatedState
		if err := r.Status().Update(ctx, rookcephfsRefVol); err != nil {
			log.Error(err, "Failed to update status of RookCephFSRefVol", "new state", updatedState)
		} else {
			log.Info("Status updated successfully", "new state", updatedState)
		}
	} else {
		log.Info("No change in state, skipping status update", "current state", updatedState)
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

// func (r *RookCephFSRefVolReconciler) updateStatus(ctx context.Context, log logr.Logger)
