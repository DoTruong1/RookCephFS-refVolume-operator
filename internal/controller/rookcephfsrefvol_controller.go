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
	"fmt"

	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

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
	//debug
	println("Start reconcile")
	if err := r.Get(ctx, req.NamespacedName, rookcephfsrefvol); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("RookCephFSRefVol resource not found. Ignoring since object must be deleted Start checking on parent PVC.")
			// Get latest manifesrt

			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get RookCephFSRefVol")
		return ctrl.Result{}, err
	}

	// Make sure source pvc is ready
	// Nếu chưa có thì tạo
	dataSourcePVC, err := r.ensureSourcePVC(ctx, log, rookcephfsrefvol)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// if rookcephfsrefvol.Spec.DataSource.PvcSpec.CreateIfNotExists {
			// 	log.Info("Not found datasource PVC:", rookcephfsrefvol.Spec.DataSource.PvcInfo.PvcName,
			// 		"Namespace:", rookcephfsrefvol.Spec.DataSource.PvcInfo.Namespace,
			// 		"maybe the datasource PVC is not created yet, reconciling")
			// 	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			log.Info("Not found datasource PvC")
			dataSourcePVC = &corev1.PersistentVolumeClaim{}

		} else {
			return ctrl.Result{}, err
		}
	}

	// Lấy tt PV nguồn
	var dataSourcePv *corev1.PersistentVolume
	if dataSourcePVC.Spec.VolumeName != "" {
		dataSourcePv, err = r.getSourcePersistentVolume(ctx, dataSourcePVC.Spec.VolumeName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Not found parent, maybe the datasource Pv is not created yet, retrying")
				// Set lại datasource là nil để đưa trạng thái về conflict
				dataSourcePv = &corev1.PersistentVolume{}
			} else {
				return ctrl.Result{}, err
			}
		}
	} else {
		dataSourcePv = nil
	}
	r.updateState(ctx, log, rookcephfsrefvol, dataSourcePv, dataSourcePVC)

	// handle cr deleting
	if rookcephfsrefvol.Status.State == operatorv1.IsDeleting {
		// check parent trước khi xóa

		return ctrl.Result{}, r.onDelete(ctx, log, rookcephfsrefvol)
	}

	// Nếu đến bước này thì có nghĩa là k có sự kiện xóa
	// có thể CR vừa được tạo hoặc update ==> có thể chưa có finalizer

	// State missing là chưa gắn với pv nào
	if rookcephfsrefvol.Status.State == operatorv1.Missing {
		if !controllerutil.ContainsFinalizer(rookcephfsrefvol, finalizerName) {
			println("Set finalizer")

			controllerutil.AddFinalizer(rookcephfsrefvol, finalizerName)
			if err := r.Update(ctx, rookcephfsrefvol); err != nil {
				return ctrl.Result{}, err
			}
		}
		log.Info("Start creating RefVol")
		if err := r.createRefVolume(ctx, log, rookcephfsrefvol, dataSourcePv, dataSourcePVC); err != nil {
			return ctrl.Result{}, r.writeInstance(ctx, log, rookcephfsrefvol)
		}

	}

	// Update lại datasourPVC và PV
	// r.Update(ctx, dataSourcePVC)
	return ctrl.Result{}, r.writeInstance(ctx, log, rookcephfsrefvol)

}

// SetupWithManager sets up the controller with the Manager.
func (r *RookCephFSRefVolReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&operatorv1.RookCephFSRefVol{},
		"spec.datasource.pvcInfo.pvcName",
		func(obj client.Object) []string {
			// Trả về giá trị name của tài nguyên
			return []string{obj.GetName()}
		},
	); err != nil {
		return fmt.Errorf("failed to index metadata.namespace: %w", err)
	}

	// Đăng ký index dựa trên "metadata.namespace"
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&operatorv1.RookCephFSRefVol{},
		"spec.datasource.pvcInfo.namespace",
		func(obj client.Object) []string {
			// Trả về giá trị namespace của tài nguyên
			return []string{obj.GetNamespace()}
		},
	); err != nil {
		return fmt.Errorf("failed to index metadata.namespace: %w", err)
	}
	updatePred := predicate.Funcs{
		// Only allow updates when the spec.size of the Busybox resource changes
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPVc := e.ObjectOld.(*corev1.PersistentVolumeClaim)
			// if _, ok := e.ObjectNew.(*operatorv1.RookCephFSRefVol); ok {
			// 	return true // Always trigger reconcile for updates to RookCephFSRefVol
			// }
			// Kiểm tra nếu là đối tượng PersistentVolumeClaim (PVC)
			if pvcNew, okPvc := e.ObjectNew.(*corev1.PersistentVolumeClaim); okPvc {
				pvcOld, okOldPvc := e.ObjectOld.(*corev1.PersistentVolumeClaim)

				// Kiểm tra PVC đã đc bound vô PV
				if okOldPvc && pvcOld.Spec.VolumeName == "" && pvcNew.Spec.VolumeName != "" {
					// Nếu PVC có annotation là "isParent: true", trigger lại reconcile
					if val, ok := e.ObjectNew.GetAnnotations()[operatorv1.IsParent]; ok && val == "true" {
						return true
					}
				}

				// Bắt sự kiện xóa
				// Should only trigger once
				if pvcNew.DeletionTimestamp != nil && oldPVc.DeletionTimestamp.IsZero() {
					// Nếu PVC có annotation là "isParent: true", trigger lại reconcile
					if val, ok := e.ObjectNew.GetAnnotations()[operatorv1.IsParent]; ok && val == "true" {
						return true
					}
				}
			}
			return false
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
		// Owns(&corev1.PersistentVolume{}).
		// Owns(&corev1.PersistentVolumeClaim{}).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(r.filterParent), builder.WithPredicates(updatePred)).
		Complete(r)
}

func (r *RookCephFSRefVolReconciler) filterParent(ctx context.Context, o client.Object) []ctrl.Request {
	// var pvName string
	switch o := o.(type) {
	// case *corev1.PersistentVolume:
	// 	// println("Parent PV is in terminating state, start delete RefVols", o.Name)
	// 	pvName = o.GetName()
	// 	println("filterParent triggered - PV deleted:", pvName) // Debug log

	case *corev1.PersistentVolumeClaim:
		// println("Parent PVC is in terminating state, start delete RefVols", o.Name)
		// pvName = o.Spec.VolumeName
		println("PVC trigger:", o.Name) // Debug log

	default:
		return []ctrl.Request{}
	}

	if o.GetAnnotations()[operatorv1.IsParent] == "true" {
		childrenList, err := r.fetchRefVolumeList(ctx, o)
		if err != nil && apierrors.IsNotFound(err) {
			return []ctrl.Request{}
		}

		reqs := make([]ctrl.Request, 0, len(childrenList))
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
	return []ctrl.Request{}
}

func (r *RookCephFSRefVolReconciler) fetchRefVolumeList(ctx context.Context, parent client.Object) ([]operatorv1.RookCephFSRefVol, error) {
	var matchingCRs []operatorv1.RookCephFSRefVol

	var rookCephFSRefVolList operatorv1.RookCephFSRefVolList
	if err := r.List(ctx, &rookCephFSRefVolList); err != nil {
		return nil, err
	}

	for _, cr := range rookCephFSRefVolList.Items {
		if cr.Spec.DataSource.PvcInfo.PvcName == parent.GetName() && cr.Spec.DataSource.PvcInfo.Namespace == parent.GetNamespace() {
			matchingCRs = append(matchingCRs, cr)
		}
	}
	return matchingCRs, nil
}

func (r *RookCephFSRefVolReconciler) onDelete(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol) error {
	// IsDeletingCRD, errr := crd.

	switch {

	case r.shouldDeleteCr(ctx, log, rookCephFSRefVol):
		deletePolicy := metav1.DeletePropagationForeground

		log.Info("Deleting CR")

		return r.Delete(ctx, rookCephFSRefVol, &client.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		})

	case r.shouldRemoveFinalizer(ctx, log, rookCephFSRefVol):
		log.Info("Remove finalizer from Crs")
		controllerutil.RemoveFinalizer(rookCephFSRefVol, finalizerName)

		return r.writeInstance(ctx, log, rookCephFSRefVol)

	default:
		if controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
			log.Info("Waiting for refVolume to be fully purged before letting the CR be deleted")
		} else {
			log.Info("Waiting for K8s to delete this CR (all finalizers are removed)")
			dataSourcePVC, err := r.getPersistentVolumeClaim(ctx, rookCephFSRefVol.Spec.DataSource.PvcInfo)
			if apierrors.IsNotFound(err) {
				return nil
			}
			var childResources operatorv1.RookCephFSRefVolList

			// Truy vấn trực tiếp từ 	API Server
			if err := r.Client.List(ctx, &childResources, client.MatchingFields{
				"spec.datasource.pvcInfo.pvcName":   dataSourcePVC.Name,
				"spec.datasource.pvcInfo.namespace": dataSourcePVC.Namespace,
			}); err != nil {
				log.Error(err, "Failed to fetch child resources from API Server")
				return err
			}
			var activeChildResources []operatorv1.RookCephFSRefVol

			// Lấy danh sách chưa bị xóa
			for _, child := range childResources.Items {
				if child.DeletionTimestamp.IsZero() {
					activeChildResources = append(activeChildResources, child)
				}
			}
			println("Số phần tử con còn lại: %s", len(activeChildResources))

			if len(childResources.Items) == 0 {
				if controllerutil.ContainsFinalizer(dataSourcePVC, finalizerName) {
					println("Số phần tử con còn lại: %s, gỡ finalizer cho parent", len(activeChildResources))
					controllerutil.RemoveFinalizer(dataSourcePVC, finalizerName)
					if err := r.Update(ctx, dataSourcePVC); err != nil {
						log.Error(err, "Gặp lỗi")
						return err
					}
				}
			}
		}
		return nil
		// return r.writeInstance(ctx, log, rookCephFSRefVol, rookCephFSRefVol.Status)
		// case operatorv1.Conflict:
		// case operatorv1.Missing:
	}

}

func (r *RookCephFSRefVolReconciler) shouldDeleteCr(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol) bool {
	// Kiểm tra nếu refVolume không tồn tại hoặc tên trống
	sourcePv, _ := r.getSourcePersistentVolume(ctx, rookCephFSRefVol.Status.Parent)
	sourcePVC, _ := r.getPersistentVolumeClaim(ctx, rookCephFSRefVol.Spec.DataSource.PvcInfo)
	switch rookCephFSRefVol.Status.State {
	case operatorv1.IsDeleting:
		// thêm !rookCephFSRefVol.DeletionTimestamp.IsZero() trong trường hợp CR được tạo với datasource đang ở trạng thái deleting
		if !controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) && !rookCephFSRefVol.DeletionTimestamp.IsZero() {
			log.Info("The CR is already finalized; no need to delete again", "name", rookCephFSRefVol.Name)
			return false
		}

		// xử lý trường hợp xóa cascading, người dùng xóa pvc gốc chứ k phải xóa CR
		// ==> trigger xóa lần đầu
		if rookCephFSRefVol.DeletionTimestamp.IsZero() && (!sourcePv.DeletionTimestamp.IsZero() || !sourcePVC.DeletionTimestamp.IsZero()) {
			log.Info("DataSource is being deleted, start delte CR", "name", rookCephFSRefVol.Name)
			return true
		}

		// xử lý trường hợp người dùng xóa CR ==> Set deletion timestamp đã thành côgn --> cần phải gỡ finalizer
		// ==> trigger xóa lần đầu
		if !rookCephFSRefVol.DeletionTimestamp.IsZero() {
			log.Info("CR is being deleted, start remove finalizer from CR", "name", rookCephFSRefVol.Name)
			return false
		}
		return true

	case operatorv1.Conflict:
		log.Info("CR is in Conflict State, Safe to delelete", "name", rookCephFSRefVol.Name)
		return true

	case operatorv1.Bounded:
		if !controllerutil.ContainsFinalizer(rookCephFSRefVol, finalizerName) {
			log.Info("The CR is already finalized; no need to delete again", "name", rookCephFSRefVol.Name)
			return false
		}
		// Bị xoá trong trạng thái bounded
		return true

	case operatorv1.Missing:
		log.Info("PV is missing; no need to delete again", "name", rookCephFSRefVol.Name)
		return true

	default:
		log.Info("Unhandled state; no action taken for deletion", "state", rookCephFSRefVol.Status.State)
		return false
	}
}
