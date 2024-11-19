package controller

import (
	"context"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const finalizerName = "operator.dotv.home.arpa/finalizer"

func (r *RookCephFSRefVolReconciler) handleFinalizers(ctx context.Context, rookCephFSRefVol *operatorv1.RookCephFSRefVol) error {
	if rookCephFSRefVol.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
			controllerutil.AddFinalizer(rookCephFSRefVol, finalizerName)
			return r.Update(ctx, rookCephFSRefVol)
		}
	}
	return nil
}

func (r *RookCephFSRefVolReconciler) shouldRemoveFinalizer(log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol, dataSourcePv *corev1.PersistentVolume, dataSourcePvc *corev1.PersistentVolumeClaim) bool {
	// If the anchor is already finalized, there's no need to do it again.
	if !controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
		return false
	}
	// in anycases, if refVol still exist, we should not finalize the Crs
	if dataSourcePv.Name != "" {
		return false
	}
	switch rookCephFSRefVol.Status.State {
	case operatorv1.Bounded:
		// RefVol và Crs được bound với nhau
		// Do đã gọi hàm shouldDeleteRefVol
		if dataSourcePv.DeletionTimestamp.IsZero() {
			log.Info("refVolume will not be deleted, allow cr to be finalized")
			return true
		}
		log.Info("RefVolume is being deleted; cannot finalize cr yet")
		return false
	case operatorv1.IsDeleting:
		if dataSourcePv.Name == "" {
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
