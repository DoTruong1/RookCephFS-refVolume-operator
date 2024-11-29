package controller

import (
	"context"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *RookCephFSRefVolReconciler) ensureSourcePVC(
	ctx context.Context,
	log logr.Logger,
	rookcephfsrefvol *operatorv1.RookCephFSRefVol,
) (*corev1.PersistentVolumeClaim, error) {
	pvcInfo := rookcephfsrefvol.Spec.DataSource.PvcInfo
	createIfNotExists := rookcephfsrefvol.Spec.DataSource.PvcSpec.CreateIfNotExists

	sourcePVC := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: pvcInfo.Namespace,
		Name:      pvcInfo.PvcName,
	}, sourcePVC)

	if apierrors.IsNotFound(err) {
		if createIfNotExists && rookcephfsrefvol.DeletionTimestamp.IsZero() {
			log.Info("Source PVC not found; creating new PVC", "PVC Name", pvcInfo.PvcName)

			sourcePVC = r.buildPVCManifest(pvcInfo, &rookcephfsrefvol.Spec.DataSource.PvcSpec, rookcephfsrefvol)
			controllerutil.AddFinalizer(sourcePVC, finalizerName)
			err := r.Create(ctx, sourcePVC)

			if err != nil {
				if apierrors.IsAlreadyExists(err) {
					log.Info("Source PVC already created by another reconcile loop", "PVC Name", pvcInfo.PvcName)
					err = r.Get(ctx, client.ObjectKey{
						Namespace: pvcInfo.Namespace,
						Name:      pvcInfo.PvcName,
					}, sourcePVC)
					if err != nil {
						log.Error(err, "Failed to fetch source PVC after concurrent creation")
						return nil, err
					}
					return sourcePVC, nil
				}
				log.Error(err, "Failed to create source PVC")
				return nil, err
			}
			// Cập nhật lại State nếu tạo thành công
			r.Update(ctx, sourcePVC)
			log.Info("Source PVC created successfully", "PVC Name", pvcInfo.PvcName)
			return sourcePVC, err
		} else {
			log.Info("Source PVC not found and createIfNotExists is false", "PVC Name", pvcInfo.PvcName)
			// fmt.Errorf("source PVC not found: %s/%s", pvcInfo.Namespace, pvcInfo.PvcName)
			return nil, err
		}
	} else if err != nil {
		// Lỗi khác khi gọi API server
		log.Error(err, "Error getting source PVC")
		return nil, err
	}

	sourcePVC.Annotations = annotationMapping(sourcePVC, rookcephfsrefvol, true)
	// childs, _ := r.fetchRefVolumeList(ctx, sourcePVC)
	if sourcePVC.DeletionTimestamp.IsZero() {
		controllerutil.AddFinalizer(sourcePVC, finalizerName)
	}
	// PVC đã tồn tại, trả về
	// log.Info("Source PVC exists", "PVC Name", pvcInfo.PvcName)
	return sourcePVC, nil
}

func (r *RookCephFSRefVolReconciler) createDestinationPVC(ctx context.Context, log logr.Logger, rookcephfsrefvol *operatorv1.RookCephFSRefVol, dataSourcePvc *corev1.PersistentVolumeClaim, pvName string) error {
	destinationPvcInfo := rookcephfsrefvol.Spec.Destination.PvcInfo
	destPVC := &corev1.PersistentVolumeClaim{}
	isExist, namespaceErr := r.isNamespaceExists(ctx, destinationPvcInfo.Namespace)
	if !isExist {
		if namespaceErr != nil {
			log.Error(namespaceErr, "Error when checking if namespace is existed")
			return namespaceErr
		}
		log.Info("Namespace: ", destinationPvcInfo.Namespace, "is not exist, not creating destination PVC")
		return nil
	}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: destinationPvcInfo.Namespace,
		Name:      destinationPvcInfo.PvcName,
	}, destPVC)

	if err != nil && apierrors.IsNotFound(err) {
		log.Info("Destination PVC not found; creating new PVC", "PVC Name", destinationPvcInfo.PvcName)
		destPVC = r.buildDestinationPVCManifest(rookcephfsrefvol, dataSourcePvc)
		destPVC.Spec.VolumeName = pvName
		controllerutil.SetControllerReference(rookcephfsrefvol, destPVC, r.Scheme)
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

// func (r *RookCephFSRefVolReconciler) deletePvc(ctx context.Context, rookcephfsrefvol *operatorv1.RookCephFSRefVol) error {
// 	pvcInfo := rookcephfsrefvol.Spec.Destination.PvcInfo
// 	destPVC := &corev1.PersistentVolumeClaim{}
// 	err := r.Get(ctx, client.ObjectKey{
// 		Namespace: pvcInfo.Namespace,
// 		Name:      pvcInfo.PvcName,
// 	}, destPVC)
// 	if err != nil && !apierrors.IsNotFound(err) {
// 		return err
// 	} else if err == nil {
// 		if err := r.deleteObject(ctx, destPVC); err != nil && !apierrors.IsNotFound(err) {
// 			return err
// 		}
// 	}
// 	return nil
// }

func (r *RookCephFSRefVolReconciler) buildPVCManifest(pvcInfo operatorv1.PvcInfo, pvcSpec *operatorv1.PvcSpec, rookcephfsrefvol *operatorv1.RookCephFSRefVol) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvcInfo.PvcName,
			Namespace:   pvcInfo.Namespace,
			Annotations: map[string]string{},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      pvcSpec.AccessModes,
			Resources:        pvcSpec.Resources,
			StorageClassName: pvcSpec.StorageClassName,
		},
	}

	pvc.ObjectMeta.Annotations = annotationMapping(pvc, rookcephfsrefvol, true)
	return pvc
}

func (r *RookCephFSRefVolReconciler) buildDestinationPVCManifest(rookcephfsrefvol *operatorv1.RookCephFSRefVol, pvcInfo *corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaim {
	pvcMetadata := rookcephfsrefvol.Spec.Destination.PvcInfo
	pvcSpec := pvcInfo.DeepCopy().Spec
	storageClassName := ""
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvcMetadata.PvcName,
			Namespace:   pvcMetadata.Namespace,
			Annotations: annotationMapping(pvcInfo, rookcephfsrefvol, false),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      pvcSpec.AccessModes,
			Resources:        pvcSpec.Resources,
			StorageClassName: &storageClassName,
			VolumeName:       rookcephfsrefvol.Status.Children,
		},
	}
	controllerutil.SetControllerReference(rookcephfsrefvol, pvc, r.Scheme)
	return pvc
}
