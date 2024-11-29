package controller

import (
	"context"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *RookCephFSRefVolReconciler) createRefVolume(ctx context.Context, log logr.Logger, rookcephfsrefvol *operatorv1.RookCephFSRefVol, parentPV *corev1.PersistentVolume, sourcePVC *corev1.PersistentVolumeClaim) error {
	if parentPV.DeletionTimestamp.IsZero() {
		desiredPv := r.buildRefVolumeManifest(parentPV, rookcephfsrefvol)
		controllerutil.SetControllerReference(rookcephfsrefvol, desiredPv, r.Scheme)
		if err := r.Create(ctx, desiredPv); err != nil {
			log.Error(err, "Error creating RefVolume")
			rookcephfsrefvol.Status.State = operatorv1.Conflict
			return err
		}

		if err := r.createDestinationPVC(ctx, log, rookcephfsrefvol, sourcePVC, desiredPv.Name); err != nil {
			log.Error(err, "Error creating Destination PV")
			rookcephfsrefvol.Status.State = operatorv1.Conflict
			return err
		}
		rookcephfsrefvol.Status.State = operatorv1.Bounded
		rookcephfsrefvol.Status.Parent = parentPV.GetName()
		rookcephfsrefvol.Status.Children = desiredPv.Name

		log.Info("Successfully created refVolume", "desiredPv", desiredPv.Name)

		return nil
	}
	log.Info("Parent is being deleted, not creating ref volume")
	rookcephfsrefvol.Status.State = operatorv1.IsDeleting
	rookcephfsrefvol.Status.Parent = parentPV.GetName()
	rookcephfsrefvol.Status.Children = ""
	return nil
}

func (r *RookCephFSRefVolReconciler) buildRefVolumeManifest(originalPv *corev1.PersistentVolume, rookcephfsrefvol *operatorv1.RookCephFSRefVol) *corev1.PersistentVolume {
	newPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: rookcephfsrefvol.Name + "-pv-",
			Annotations:  make(map[string]string),
			Labels:       make(map[string]string),
		},
		Spec: *originalPv.Spec.DeepCopy(),
	}

	if newPV.Spec.PersistentVolumeSource.CSI != nil {
		newPV.Spec.ClaimRef = nil
		newPV.Spec.PersistentVolumeSource.CSI.NodeStageSecretRef.Name = rookcephfsrefvol.Spec.CephFsUserSecretName
		newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["staticVolume"] = "true"
		newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["rootPath"] = newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["subvolumePath"]
		newPV.Spec.PersistentVolumeSource.CSI.VolumeHandle = newPV.Spec.PersistentVolumeSource.CSI.VolumeHandle + "-" + rookcephfsrefvol.Name
		newPV.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
	}

	newPV.ObjectMeta.Annotations = annotationMapping(originalPv, rookcephfsrefvol, false)
	for k, v := range originalPv.GetLabels() {
		newPV.ObjectMeta.Labels[k] = v
	}

	// Thiết lập owner reference
	controllerutil.SetControllerReference(rookcephfsrefvol, newPV, r.Scheme)
	return newPV
}
