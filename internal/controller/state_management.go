package controller

import (
	"context"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
)

func isOnDeletingState(rookcephfsrefvol *operatorv1.RookCephFSRefVol, parentPV *corev1.PersistentVolume, sourcePVC *corev1.PersistentVolumeClaim) bool {
	isDataSourcePvDeleting := !parentPV.DeletionTimestamp.IsZero()
	isDataSourcePvcDeleting := !sourcePVC.DeletionTimestamp.IsZero()
	isRookCephRefVolDeleting := !rookcephfsrefvol.DeletionTimestamp.IsZero()
	return (isRookCephRefVolDeleting || isDataSourcePvcDeleting || isDataSourcePvDeleting)
}

func (r *RookCephFSRefVolReconciler) updateState(ctx context.Context, log logr.Logger, rookcephfsrefvol *operatorv1.RookCephFSRefVol, parentPV *corev1.PersistentVolume, sourcePVC *corev1.PersistentVolumeClaim) {
	updatedState := rookcephfsrefvol.Status.State

	switch {
	case isOnDeletingState(rookcephfsrefvol, parentPV, sourcePVC):
		updatedState = operatorv1.IsDeleting
	case parentPV.Name == "" || sourcePVC.Name == "":
		log.Info("Parent PV or PVC not found; setting state to ParentNotFound")
		updatedState = operatorv1.ParentNotFound
	case rookcephfsrefvol.Status.Children == "":
		log.Info("No PV is associated with RookCephFSRefVol; marking state as Missing")
		updatedState = operatorv1.Missing
	default:
		updatedState = operatorv1.Bounded
	}

	if rookcephfsrefvol.Status.State != updatedState {
		rookcephfsrefvol.Status.State = updatedState
		if err := r.Status().Update(ctx, rookcephfsrefvol); err != nil {
			log.Error(err, "Failed to update status", "new state", updatedState)
		}
	}
}
