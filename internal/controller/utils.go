// controllers/utils.go

package controller

import (
	"context"
	"strings"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ownObject sets the owner reference of an object to the given CR
func (r *RookCephFSRefVolReconciler) ownObject(cr *operatorv1.RookCephFSRefVol, obj client.Object) error {
	return controllerutil.SetControllerReference(cr, obj, r.Scheme)
}

// writeInstance creates or updates an instance on the API server
func (r *RookCephFSRefVolReconciler) writeInstance(ctx context.Context, rookcephfsrefvol *operatorv1.RookCephFSRefVol) error {
	if rookcephfsrefvol.CreationTimestamp.IsZero() {
		return r.Create(ctx, rookcephfsrefvol)
	}
	return r.Update(ctx, rookcephfsrefvol)
}

// getPersistentVolumeClaim retrieves a PVC from the cluster
func (r *RookCephFSRefVolReconciler) getPersistentVolumeClaim(ctx context.Context, pvcInfo operatorv1.PvcInfo) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: pvcInfo.Namespace,
		Name:      pvcInfo.PvcName,
	}, pvc)
	return pvc, err
}

// getSourcePersistentVolume retrieves a PV from the cluster
func (r *RookCephFSRefVolReconciler) getSourcePersistentVolume(ctx context.Context, volumeName string) (*corev1.PersistentVolume, error) {
	pv := &corev1.PersistentVolume{}
	err := r.Get(ctx, client.ObjectKey{
		Name: volumeName,
	}, pv)
	return pv, err
}

// annotationMapping copies annotations from the original PV and adds necessary ones
func annotationMapping(originalPv *corev1.PersistentVolume, rookcephfsrefvol *operatorv1.RookCephFSRefVol) map[string]string {
	annotations := make(map[string]string)
	for k, v := range originalPv.GetAnnotations() {
		if !strings.HasPrefix(k, operatorv1.MetaGroup) {
			annotations[k] = v
		}
	}
	annotations[operatorv1.Owner] = rookcephfsrefvol.Name
	annotations[operatorv1.Parent] = originalPv.Name
	annotations[operatorv1.CreatedBy] = operatorv1.ControllerName
	return annotations
}

// deleteObject deletes a Kubernetes object
func (r *RookCephFSRefVolReconciler) deleteObject(ctx context.Context, obj client.Object) error {
	return r.Delete(ctx, obj)
}
