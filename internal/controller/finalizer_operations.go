package controller

import (
	"context"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const finalizerName = "operator.dotv.home.arpa/finalizer"

// func (r *RookCephFSRefVolReconciler) handleFinalizers(ctx context.Context, obj client.Object) error {
// 	if obj.GetDeletionTimestamp().IsZero() {
// 		if !controllerutil.ContainsFinalizer(obj, finalizerName) {
// 			controllerutil.AddFinalizer(obj, finalizerName)
// 			return r.Update(ctx, obj)
// 		}
// 	} else {
// 		if controllerutil.ContainsFinalizer(obj, finalizerName) {
// 			controllerutil.RemoveFinalizer(obj, finalizerName)
// 			return r.Update(ctx, obj)
// 		}
// 	}
// 	return nil
// }

func (r *RookCephFSRefVolReconciler) shouldRemoveFinalizer(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol) bool {
	childrenPvc, _ := r.getPersistentVolumeClaim(ctx, rookCephFSRefVol.Spec.Destination.PvcInfo)
	childrenPv, _ := r.getSourcePersistentVolume(ctx, rookCephFSRefVol.Status.Children)

	println("Enter remove finalize")
	// K có thì k cần phải xóa finalizer
	if !controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
		return false
	}

	switch rookCephFSRefVol.Status.State {
	case operatorv1.Bounded:
		// Đã gọi hàm xóa CR ==> các child cũng phải đang bị xóa theo
		// nhưng mà nếu vì lí do gì đó mà k xóa thì tốt hơn hết cứ gỡ finalizer
		if childrenPv.DeletionTimestamp.IsZero() || childrenPvc.DeletionTimestamp.IsZero() {
			log.Info("refVolume will not be deleted, allow cr to be finalized")
			return true
		}
		log.Info("RefVolume is being deleted; cannot finalize cr yet")
		return false
	case operatorv1.Missing:
		log.Info("The CRs is not bounded to any PV ==> safe to remove finalize")
		return true
	case operatorv1.IsDeleting:
		println("Enter remove operatorv1.IsDeleting finalize")

		// có thể pv và pvc đích bị kẹt ở trạng thái terminating do đang được sử dụng  ==> 0 gỡ finalizer
		if childrenPvc.DeletionTimestamp != nil || childrenPv.DeletionTimestamp != nil {
			println("Enter remove operatorv1.IsDeleting finalize")
			return false
		}
		// các case còn lại có thể gỡ
		return true

	case operatorv1.Conflict:
		log.Info("The Crs is in Conflict state, safe to remove finalize")
		return false
	default:
		// Should never happen, so log an error and let it be deleted.
		// log.Error(errors.New("illegal state"), "Unknown state", "state", inst.Status.State)
		return true
	}
}
