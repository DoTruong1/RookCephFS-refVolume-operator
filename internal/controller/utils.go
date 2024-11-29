// controllers/utils.go

package controller

import (
	"context"
	"fmt"
	"strings"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ownObject sets the owner reference of an object to the given CR
func (r *RookCephFSRefVolReconciler) ownObject(cr *operatorv1.RookCephFSRefVol, obj client.Object) error {
	return controllerutil.SetControllerReference(cr, obj, r.Scheme)
}

// writeInstance creates or updates an instance on the API server
func (r *RookCephFSRefVolReconciler) writeInstance(ctx context.Context, log logr.Logger, rookcephfsrefvol *operatorv1.RookCephFSRefVol) error {
	if rookcephfsrefvol.CreationTimestamp.IsZero() {
		if err := r.Create(ctx, rookcephfsrefvol); err != nil {
			log.Error(err, "while creating on apiserver")
			return err
		}
	} else {
		// if err := r.Get(ctx, client.ObjectKey{Name: rookcephfsrefvol.Name, Namespace: rookcephfsrefvol.Namespace}, rookcephfsrefvol); err != nil {
		// 	log.Error(err, "failed to fetch latest object before updating status")
		// 	return err
		// }
		// log.Info("Updating CRs status")
		// if err := r.Status().Update(ctx, rookcephfsrefvol); err != nil {
		// 	log.Error(err, "while updating status on apiserver")
		// 	return err
		// }
		// if err := r.Get(ctx, client.ObjectKey{Name: rookcephfsrefvol.Name, Namespace: rookcephfsrefvol.Namespace}, rookcephfsrefvol); err != nil {
		// 	log.Error(err, "failed to fetch latest object before updating status")
		// 	return err
		// }
		if err := r.Update(ctx, rookcephfsrefvol); err != nil {
			log.Error(err, "while updating on apiserver")
			return err
		}
		// if prevState.State != rookcephfsrefvol.Status.State {

		// }

	}
	return nil
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
func annotationMapping(obj client.Object, rookcephfsrefvol *operatorv1.RookCephFSRefVol, isBuildParentManifest bool) map[string]string {
	annotations := make(map[string]string)
	for k, v := range obj.GetAnnotations() {
		if !strings.HasPrefix(k, operatorv1.MetaGroup) {
			annotations[k] = v
		}
	}
	if !isBuildParentManifest {
		annotations[operatorv1.Parent] = obj.GetName()
	} else {
		annotations[operatorv1.Children] = rookcephfsrefvol.Name
		annotations[operatorv1.IsParent] = "true"
	}
	annotations[operatorv1.CreatedBy] = operatorv1.ControllerName
	return annotations
}

// deleteObject deletes a Kubernetes object
func (r *RookCephFSRefVolReconciler) deleteObject(ctx context.Context, obj client.Object) error {
	return r.Delete(ctx, obj)
}

func (r *RookCephFSRefVolReconciler) isNamespaceExists(ctx context.Context, namespace string) (bool, error) {
	logger := log.FromContext(ctx)

	// Tạo đối tượng Namespace để kiểm tra
	ns := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: namespace}, ns)
	if err != nil {
		if errors.IsNotFound(err) {
			// Namespace không tồn tại
			logger.Info(fmt.Sprintf("Namespace '%s' does not exist.", namespace))
			return false, nil
		}
		// Lỗi khác
		logger.Error(err, "Failed to check namespace existence")
		return false, err
	}

	// Namespace tồn tại
	logger.Info(fmt.Sprintf("Namespace '%s' exists.", namespace))
	return true, nil
}
