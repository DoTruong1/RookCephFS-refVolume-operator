package controller

import (
	"context"
	"fmt"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *RookCephFSRefVolReconciler) ensureSourcePVC(ctx context.Context, log logr.Logger, rookcephfsrefvol *operatorv1.RookCephFSRefVol) (*corev1.PersistentVolumeClaim, error) {
	pvcInfo := rookcephfsrefvol.Spec.DataSource.PvcInfo
	createIfNotExists := rookcephfsrefvol.Spec.DataSource.PvcSpec.CreateIfNotExists

	sourcePVC := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: pvcInfo.Namespace,
		Name:      pvcInfo.PvcName,
	}, sourcePVC)

	if err != nil && apierrors.IsNotFound(err) {
		if createIfNotExists {
			log.Info("Source PVC not found; creating new PVC", "PVC Name", pvcInfo.PvcName)
			sourcePVC = r.buildPVCManifest(pvcInfo, &rookcephfsrefvol.Spec.DataSource.PvcSpec)
			if err := r.Create(ctx, sourcePVC); err != nil {
				log.Error(err, "Failed to create source PVC")
				return nil, err
			}
			log.Info("Source PVC created successfully")
		} else {
			log.Info("Source PVC not found and createIfNotExists is false")
			return nil, fmt.Errorf("source PVC not found")
		}
	} else if err != nil {
		log.Error(err, "Error getting source PVC")
		return nil, err
	}
	return sourcePVC, nil
}

func (r *RookCephFSRefVolReconciler) createDestinationPVC(ctx context.Context, log logr.Logger, rookcephfsrefvol *operatorv1.RookCephFSRefVol, dataSourcePvc *corev1.PersistentVolumeClaim) error {
	destinationPvcInfo := rookcephfsrefvol.Spec.Destination.PvcInfo
	destPVC := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: destinationPvcInfo.Namespace,
		Name:      destinationPvcInfo.PvcName,
	}, destPVC)

	if err != nil && apierrors.IsNotFound(err) {
		log.Info("Destination PVC not found; creating new PVC", "PVC Name", destinationPvcInfo.PvcName)
		destPVC = r.buildDestinationPVCManifest(rookcephfsrefvol, dataSourcePvc)
		if err := controllerutil.SetControllerReference(rookcephfsrefvol, destPVC, r.Scheme); err != nil {
			log.Error(err, "Failed to set owner reference for destination PVC")
			return err
		}
		if err := r.Create(ctx, destPVC); err != nil {
			log.Error(err, "Failed to create destination PVC")
			return err
		}
		log.Info("Destination PVC created successfully")
	} else if err != nil {
		log.Error(err, "Error getting destination PVC")
		return err
	}
	return nil
}

func (r *RookCephFSRefVolReconciler) buildPVCManifest(pvcInfo operatorv1.PvcInfo, pvcSpec *operatorv1.PvcSpec) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcInfo.PvcName,
			Namespace: pvcInfo.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      pvcSpec.AccessModes,
			Resources:        pvcSpec.Resources,
			StorageClassName: pvcSpec.StorageClassName,
		},
	}
}

func (r *RookCephFSRefVolReconciler) buildDestinationPVCManifest(rookcephfsrefvol *operatorv1.RookCephFSRefVol, pvcInfo *corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaim {
	pvcMetadata := rookcephfsrefvol.Spec.Destination.PvcInfo
	pvcSpec := pvcInfo.DeepCopy().Spec
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcMetadata.PvcName,
			Namespace: pvcMetadata.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      pvcSpec.AccessModes,
			Resources:        pvcSpec.Resources,
			StorageClassName: pvcSpec.StorageClassName,
			VolumeName:       rookcephfsrefvol.Status.Children,
		},
	}
}
