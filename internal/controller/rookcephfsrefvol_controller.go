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
)

// const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RookCephFSRefVolReconciler reconciles a RookCephFSRefVol object
type RookCephFSRefVolReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var (
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
	log := log.FromContext(ctx)
	rookcephfsrefvol := &operatorv1.RookCephFSRefVol{}

	if err := r.Get(ctx, req.NamespacedName, rookcephfsrefvol); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("RookCephFSRefVol resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get RookCephFSRefVol")
		return ctrl.Result{}, err
	}

	// Xử lý finalizers
	// Thêm nếu chưa có
	if err := r.handleFinalizers(ctx, rookcephfsrefvol); err != nil {
		return ctrl.Result{}, err
	}

	// Make sure source pvc is ready
	dataSourcePVC, err := r.ensureSourcePVC(ctx, log, rookcephfsrefvol)
	if err != nil {
		return ctrl.Result{}, err
	}

	// sourcePVC, err := r.getPersistentVolumeClaim(ctx, rookcephfsrefvol.Spec.DataSource.PvcInfo)
	// if err != nil {
	// 	log.Error(err, "Failed to get source PVC")
	// 	return ctrl.Result{}, err
	// }

	// Lấy tt PV nguồn
	dataSourcePv, err := r.getSourcePersistentVolume(ctx, dataSourcePVC.Spec.VolumeName)
	if err != nil {
		log.Error(err, "Failed to get parent PV")
		return ctrl.Result{}, err
	}

	r.updateState(ctx, log, rookcephfsrefvol, dataSourcePv, dataSourcePVC)

	if !rookcephfsrefvol.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := r.onDelete(ctx, log, rookcephfsrefvol, dataSourcePVC, dataSourcePv); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if rookcephfsrefvol.Status.State == operatorv1.Missing {
		if err := r.createRefVolume(ctx, log, rookcephfsrefvol, dataSourcePv, dataSourcePVC); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.createDestinationPVC(ctx, log, rookcephfsrefvol, dataSourcePVC); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: time.Minute * 1}, nil

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

func (r *RookCephFSRefVolReconciler) onDelete(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, dataSourcePvc *corev1.PersistentVolumeClaim, dataSourcePv *corev1.PersistentVolume) error {

	// IsDeletingCRD, errr := crd.
	switch {
	case r.shouldDeleteRefVol(log, rookCephFSRefVol, dataSourcePvc):
		log.Info("Deleting refVolume due to CR being deleted")
		// delete PVC
		err := r.deleteObject(ctx, dataSourcePvc)
		if err != nil {
			return err
		}

		// delete PV
		err2 := r.deleteObject(ctx, dataSourcePv)
		if err2 != nil {
			return err2
		}
		return nil
	case r.shouldDeleteRookCephFsRefVolume(log, rookCephFSRefVol, dataSourcePvc):
		log.Info("Deleting rookCephFSRefVol due to CR being deleted")
		return r.deleteObject(ctx, rookCephFSRefVol)
	case r.shouldRemoveFinalizer(log, rookCephFSRefVol, dataSourcePv, dataSourcePvc):
		log.Info("Remove finalizer from Crs")
		controllerutil.RemoveFinalizer(rookCephFSRefVol, finalizerName)
		return r.writeInstance(ctx, rookCephFSRefVol)
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

func (r *RookCephFSRefVolReconciler) shouldDeleteRookCephFsRefVolume(log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolumeClaim) bool {

	if operatorv1.IsDeleting == rookCephFSRefVol.Status.State && refVolume.Name == "" && !rookCephFSRefVol.DeletionTimestamp.IsZero() {
		log.Info("RookcephFsRefVol state is being marked at deleting and no RefVol bound to it, should finailzed it!!")
		return false
	}
	println("true")
	return true
}

func (r *RookCephFSRefVolReconciler) shouldDeleteRefVol(log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolumeClaim) bool {
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

// func (r *RookCephFSRefVolReconciler) shouldCreateRefVol(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol) (*corev1.PersistentVolume, bool) {
// 	persistentVolumeClaim, err := r.getPersistentVolumeClaim(ctx, log, rookCephFSRefVol)
// 	if err != nil {
// 		if !apierrors.IsNotFound(err) {
// 			log.Error(err, "Err when get PersistentClaim")
// 			return nil, false
// 		}
// 		log.Info("PeristenVolumeClaim", rookCephFSRefVol.Spec.PvcName, "Namespace", rookCephFSRefVol.Namespace, "is not existed")
// 		return nil, false
// 	}
// 	pv, err := r.getSourcePersistentVolume(ctx, log, persistentVolumeClaim.Spec.VolumeName)

// 	if err != nil {
// 		log.Info("Err when get PersistentVolume", rookCephFSRefVol.Spec.PvcName, "Namespace", rookCephFSRefVol.Namespace, "is not existed")
// 		return nil, false
// 	}

// 	if !pv.DeletionTimestamp.IsZero() {
// 		log.Info("ParentPv is deleting", rookCephFSRefVol.Status.Parent)
// 		return nil, false
// 	}

// 	return pv, true
// }

// func (r *RookCephFSRefVolReconciler) getPersistentVolumeClaim(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol) (*corev1.PersistentVolumeClaim, error) {
// 	persistentVolumeClaim := &corev1.PersistentVolumeClaim{}

// 	// var PersistentVolume corev1.PersistentVolume

// 	if err := r.Get(ctx, client.ObjectKey{
// 		Namespace: rookCephFSRefVol.Spec.Namespace,
// 		Name:      rookCephFSRefVol.Spec.PvcName,
// 	}, persistentVolumeClaim); err != nil {
// 		// we'll ignore not-found errors, since they can't be fixed by an immediate
// 		// requeue (we'll need to wait for a new notification), and we can get them
// 		// on deleted requests.
// 		return &corev1.PersistentVolumeClaim{}, err
// 	}
// 	return persistentVolumeClaim, nil
// }

// func (r *RookCephFSRefVolReconciler) getRefVolume(ctx context.Context, log logr.Logger, rookCephFSRefVolume *operatorv1.RookCephFSRefVol) (*corev1.PersistentVolume, error) {
// 	persistentVolume := &corev1.PersistentVolume{}

// 	// var PersistentVolume corev1.PersistentVolume
// 	if err := r.Get(ctx, client.ObjectKey{
// 		Name: rookCephFSRefVolume.Status.Children,
// 	}, persistentVolume); err != nil {
// 		if !apierrors.IsNotFound(err) {
// 			log.Error(err, "unable to fetch source PersistentVolumeClaim")
// 			return nil, err
// 		}
// 		// we'll ignore not-found errors, since they can't be fixed by an immediate
// 		// requeue (we'll need to wait for a new notification), and we can get them
// 		// on deleted requests.
// 		return &corev1.PersistentVolume{}, nil
// 	}

// 	return persistentVolume, nil
// }

// func (r *RookCephFSRefVolReconciler) shouldTriggerDelete(ctx context.Context, refVolume *corev1.PersistentVolume) bool {

// 	if refVolume.GetName() == "" {
// 		return false
// 	}
// 	parentPV := &corev1.PersistentVolume{}

// 	if err := r.Get(ctx, client.ObjectKey{
// 		Name: refVolume.GetAnnotations()[operatorv1.Parent],
// 	}, parentPV); err != nil {
// 		return false
// 	}
// 	return parentPV.DeletionTimestamp.IsZero()
// }

// func (r *RookCephFSRefVolReconciler) getSourcePersistentVolume(ctx context.Context, log logr.Logger, parentPvName string) (*corev1.PersistentVolume, error) {
// 	persistentVolume := &corev1.PersistentVolume{}

// 	if err := r.Get(ctx, client.ObjectKey{
// 		Name: parentPvName,
// 	}, persistentVolume); err != nil {
// 		return &corev1.PersistentVolume{}, err
// 	}

// 	return persistentVolume, nil
// }

// func (r *RookCephFSRefVolReconciler) deleteObject(ctx context.Context, log logr.Logger, obj client.Object) error {
// 	if err := r.Delete(ctx, obj); err != nil {
// 		log.Error(err, "While deleting", "Kind", obj.GetObjectKind().GroupVersionKind().Kind, "Name", obj.GetName())
// 		return err
// 	}
// 	return nil
// }

// func (r *RookCephFSRefVolReconciler) RemoveFinalizer(ctx context.Context, log logr.Logger, rookCephFSRefVolume *operatorv1.RookCephFSRefVol) error {
// 	if err := r.Update(ctx, rookCephFSRefVolume); err != nil {
// 		log.Error(err, "While deleting ref volume")
// 		return err
// 	}
// 	return nil
// }

// func (r *RookCephFSRefVolReconciler) isRootPv(ctx context.Context, log logr.Logger, rookcephfsRefVol *operatorv1.RookCephFSRefVol) error {
// 	state := rookcephfsRefVol.Status.State
// 	switch state {
// 	case operatorv1.Ok:
// 		// PV đã được tạo và bound với CR
// 		// Thực hiện xem có nên xoá PV k

// 	}
// 	return nil
// }

// func (r *RookCephFSRefVolReconciler) updateState(ctx context.Context, log logr.Logger, rookcephfsRefVol *operatorv1.RookCephFSRefVol, refVolume *corev1.PersistentVolume, parentPV *corev1.PersistentVolume, parentPVC *corev1.PersistentVolumeClaim) {
// 	updatedState := rookcephfsRefVol.Status.State
// 	switch {
// 	case !rookcephfsRefVol.DeletionTimestamp.IsZero() || !parentPV.DeletionTimestamp.IsZero() || !parentPVC.DeletionTimestamp.IsZero():
// 		log.Info("RookCephFSRefVol is being deleted; marking state as IsDeleting")
// 		updatedState = operatorv1.IsDeleting
// 	case parentPV.Name == "" || parentPVC.Name == "":
// 		log.Info("Parent PV does not exist; marking RookCephFSRefVol as ParentNotFound")
// 		updatedState = operatorv1.ParentNotFound

// 		if rookcephfsRefVol.Status.Children != "" {
// 			// handle case bị bỏ lại orphan cuối cùng
// 			updatedState = operatorv1.IsDeleting
// 		}
// 	case refVolume.Name == "":
// 		log.Info("No PV is associated with RookCephFSRefVol; marking state as Missing",
// 			"RookCephFSRefVol", rookcephfsRefVol.Name, "Namespace", rookcephfsRefVol.Spec.Namespace)
// 		updatedState = operatorv1.Missing
// 	default:
// 		if rookcephfsRefVol.Status.State != operatorv1.Bounded {
// 			log.Info("RefVolume is successfully created; marking RookCephFSRefVol state as Bounded",
// 				"previous state", rookcephfsRefVol.Status.State)
// 		}
// 		updatedState = operatorv1.Bounded
// 	}

// 	// Cập nhật trạng thái nếu có sự thay đổi
// 	if rookcephfsRefVol.Status.State != updatedState {
// 		rookcephfsRefVol.Status.State = updatedState
// 		if err := r.Status().Update(ctx, rookcephfsRefVol); err != nil {
// 			log.Error(err, "Failed to update status of RookCephFSRefVol", "new state", updatedState)
// 		} else {
// 			log.Info("Status updated successfully", "new state", updatedState)
// 		}
// 	} else {
// 		log.Info("No change in state, skipping status update", "current state", updatedState)
// 	}
// }

// func (r *RookCephFSRefVolReconciler) writeInstance(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol) error {
// 	if rookCephFSRefVol.CreationTimestamp.IsZero() {
// 		if err := r.Create(ctx, rookCephFSRefVol); err != nil {
// 			log.Error(err, "while creating on apiserver")
// 			return err
// 		}
// 	} else {
// 		if err := r.Update(ctx, rookCephFSRefVol); err != nil {
// 			log.Error(err, "while updating on apiserver")
// 			return err
// 		}
// 	}
// 	return nil
// }

// func (r *RookCephFSRefVolReconciler) updateStatus(ctx context.Context, log logr.Logger)
