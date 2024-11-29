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

// func isConfict(ookcephfsrefvol *operatorv1.RookCephFSRefVol, parentPV *corev1.PersistentVolume, sourcePVC *corev1.PersistentVolumeClaim) bool {

// }

func (r *RookCephFSRefVolReconciler) updateState(ctx context.Context, log logr.Logger, rookcephfsrefvol *operatorv1.RookCephFSRefVol, datasourcetPv *corev1.PersistentVolume, datasourcePvc *corev1.PersistentVolumeClaim) {

	switch {
	case datasourcetPv == nil || datasourcePvc == nil:
		log.Info("Parent PV or PVC not found; setting state to Conflict")
		rookcephfsrefvol.Status.State = operatorv1.Conflict
	case isOnDeletingState(rookcephfsrefvol, datasourcetPv, datasourcePvc):
		log.Info("Deleting event; marking state as Deleting")
		rookcephfsrefvol.Status.State = operatorv1.IsDeleting

	case rookcephfsrefvol.Status.Children == "":
		log.Info("No PV is associated with RookCephFSRefVol; marking state as Missing")
		rookcephfsrefvol.Status.State = operatorv1.Missing
	default:
		log.Info("PV is associated with RookCephFSRefVol; marking state as Bounded")
		rookcephfsrefvol.Status.State = operatorv1.Bounded
	}
}
