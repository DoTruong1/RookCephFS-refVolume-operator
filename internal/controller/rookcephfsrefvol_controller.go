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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RookCephFSRefVolReconciler reconciles a RookCephFSRefVol object
type RookCephFSRefVolReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var (
	finalizerName  = "operator.dotv.home.arpa/finalizer"
	pvOwnerKey     = ".metadata.controller"
	controllerName = "RookCephFSController"
)

// +kubebuilder:rbac:groups=operator.dotv.home.arpa,resources=rookcephfsrefvols,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.dotv.home.arpa,resources=rookcephfsrefvols/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.dotv.home.arpa,resources=rookcephfsrefvols/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumes/status,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get

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

	log.Info("RookeCephFSRefVol controller triggerd")
	var rookcephfsrefvol operatorv1.RookCephFSRefVol

	// fetch toàn bộ các rookcephfsrefvol
	if err := r.Get(ctx, req.NamespacedName, &rookcephfsrefvol); err != nil {
		log.Error(err, "unable to fetch RookCephFSRefVol")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// sourcePvcName := &rookcephfsrefvol.Spec.PvcName

	// Handle delete ###
	// examine DeletionTimestamp to determine if object is under deletion
	if rookcephfsrefvol.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(&rookcephfsrefvol, finalizerName) {
			controllerutil.AddFinalizer(&rookcephfsrefvol, finalizerName)
			if err := r.Update(ctx, &rookcephfsrefvol); err != nil {
				log.Error(err, "Gặp lỗi trong quá trình thêm finalizers của tài nguyên RookCephFSVol")
				return ctrl.Result{}, err
			}
		}
	} else {
		// if the object is being deleted
		log.Info("Có yêu cầu xoá! Proceeding to cleanup the finalizers...")
		if controllerutil.ContainsFinalizer(&rookcephfsrefvol, finalizerName) {
			// Thực hiện xoá
			if err := r.onDelete(&rookcephfsrefvol); err != nil {
				log.Error(err, "Gặp lỗi trong quá trình  tài nguyên RookCephFSVol")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(&rookcephfsrefvol, finalizerName)

			if err := r.Update(ctx, &rookcephfsrefvol); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	log.Info("Bắt đầu quá trình thực hiện tạo Pv cho RookCephFSRefVol: ", rookcephfsrefvol.Name)

	originalPv, err := r.getSourcePersistentVolume(ctx, log, &rookcephfsrefvol)
	// check if desiredPv is alreay created
	if err != nil {
		log.Error(err, "Gặp lỗi trong quá trình lấy manifest từ pv gốc")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	foundedPv := &corev1.PersistentVolume{}
	if err := r.Get(ctx, client.ObjectKey{
		Name: rookcephfsrefvol.Name + "-" + rookcephfsrefvol.Namespace,
	}, foundedPv); err != nil && errors.IsNotFound(err) {
		if errors.IsNotFound(err) {
			// Tạo object nếu chưa có
			desiredPv := r.buildRefVolumeManifest(originalPv, &rookcephfsrefvol)
			log.Info("Chuẩn bị khởi tạo PersistenVolume", "PV.Namespace", desiredPv.Name)
			if err := r.Create(ctx, desiredPv); err != nil {
				log.Error(err, "Gặp lỗi khi tạo PersistenVolume", desiredPv.Name)
				return ctrl.Result{}, err
			}
			// Trigger lại reconcile để đảm bảo Pv được tạo
			return ctrl.Result{RequeueAfter: time.Minute}, r.Create(ctx, desiredPv)
		} else if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile PV

	// UPDATE CR Status

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RookCephFSRefVolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.RookCephFSRefVol{}). //specifies the type of resource to watch
		Owns(&corev1.PersistentVolume{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

// func generateRandomString() string {
// 	rand.Seed(uint64(time.Now().UnixNano()))
// 	b := make([]byte, 6)
// 	for i := range b {
// 		b[i] = charset[rand.Intn(len(charset))]
// 	}
// 	return string(b)
// }

func getSourcePvManifest(r *RookCephFSRefVolReconciler, ctx context.Context, originalPvName *string) (*corev1.PersistentVolume, error) {
	var PersistentVolume corev1.PersistentVolume
	if err := r.Get(ctx, client.ObjectKey{
		Name: *originalPvName,
	}, &PersistentVolume); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them

		return nil, client.IgnoreNotFound(err) // on deleted requests.

	}
	// TODO(user): your logic here
	return &PersistentVolume, nil
}

func (r *RookCephFSRefVolReconciler) buildRefVolumeManifest(originalPv *corev1.PersistentVolume, rookCephFSRefVol *operatorv1.RookCephFSRefVol) *corev1.PersistentVolume {
	newPvPrefix := "-" + rookCephFSRefVol.ObjectMeta.Name + "-" + rookCephFSRefVol.ObjectMeta.Namespace
	newPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: rookCephFSRefVol.Name + "-" + rookCephFSRefVol.Namespace,
			Labels: map[string]string{
				"original-data-source": originalPv.Name,
				"created-by":           controllerName,
			},
		},
		Spec: *originalPv.Spec.DeepCopy(),
	}
	if newPV.Spec.PersistentVolumeSource.CSI != nil {
		newPV.Spec.ClaimRef = nil
		newPV.Spec.PersistentVolumeSource.CSI.NodeStageSecretRef.Name = "rook-csi-cephfs-node-user"
		newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["staticVolume"] = "true"
		newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["rootPath"] = newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["subvolumePath"]
		newPV.Spec.PersistentVolumeSource.CSI.VolumeHandle = newPV.Spec.PersistentVolumeSource.CSI.VolumeHandle + newPvPrefix
		newPV.Spec.PersistentVolumeReclaimPolicy = "Retain"
	}
	// set parent child ownership
	controllerutil.SetControllerReference(rookCephFSRefVol, newPV, r.Scheme)
	return newPV
}

func (r *RookCephFSRefVolReconciler) onDelete(rookCephFSRefVol *operatorv1.RookCephFSRefVol) error {
	// We handle deletions differently depending on whether _one_ anchor is being deleted (i.e., the
	// user wants to delete the namespace) or whether the Anchor CRD is being deleted, which usually
	// means HNC is being uninstalled and we shouldn't delete _any_ namespaces.

	// TO DO:
	// get all pv associate with the root pv
	// check its status? delete able ()
	return nil
}

// func (r *RookCephFSRefVolReconciler) createRookCephFSPersistentVolume(ctx context.Context, log logr.Logger) error {

// 	log.Info("Thông tin PVC", "Volume name", persistentVolumeClaim.Spec.VolumeName)

// 	SourcePvName := &persistentVolumeClaim.Spec.VolumeName
// 	pv, err := getSourcePvManifest(r, ctx, SourcePvName)

// 	if err != nil {
// 		log.Error(err, "unable to fetch source PersistentVolume")

// 	}

// 	if pv != nil {
// 		log.Info("Thong tin PV", "PV info: ", pv.Name)

// 		refVolumeManifest := r.buildNewRefVolumeManifest(pv, &rookcephfsrefvol)

// 		err := r.Create(ctx, refVolumeManifest)

// 		if err != nil {
// 			log.Error(err, "Có lỗi xảy ra khi tạo ref volume")
// 			// we'll ignore not-found errors, since they can't be fixed by an immediate
// 			// requeue (we'll need to wait for a new notification), and we can get them
// 			// on deleted requests.
// 			return client.IgnoreNotFound(err)
// 		}
// 	}
// 	inst.ObjectMeta.Name = nm
// 	metadata.SetAnnotation(inst, api.SubnamespaceOf, pnm)

// 	// It's safe to use create here since if the namespace is created by someone
// 	// else while this reconciler is running, returning an error will trigger a
// 	// retry. The reconciler will set the 'Conflict' state instead of recreating
// 	// this namespace. All other transient problems should trigger a retry too.
// 	log.Info("Creating subnamespace")
// 	if err := r.Create(ctx, inst); err != nil {
// 		log.Error(err, "While creating subnamespace")
// 		return err
// 	}
// 	return nil
// }

func (r *RookCephFSRefVolReconciler) getPersistentVolumeClaim(ctx context.Context, log logr.Logger, pvcName string, namespace string) (*corev1.PersistentVolumeClaim, error) {
	persistentVolumeClaim := &corev1.PersistentVolumeClaim{}

	// var PersistentVolume corev1.PersistentVolume

	if err := r.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      pvcName,
	}, persistentVolumeClaim); err != nil {
		log.Error(err, "unable to fetch source PersistentVolumeClaim")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return &corev1.PersistentVolumeClaim{}, err
	}

	return persistentVolumeClaim, nil
}

func (r *RookCephFSRefVolReconciler) getSourcePersistentVolume(ctx context.Context, log logr.Logger, rookCephFSRefVol *operatorv1.RookCephFSRefVol) (*corev1.PersistentVolume, error) {
	persistentVolume := &corev1.PersistentVolume{}
	// var PersistentVolume corev1.PersistentVolume
	persistentVolumeClaim, err := r.getPersistentVolumeClaim(ctx, log, rookCephFSRefVol.Spec.PvcName, rookCephFSRefVol.Namespace)

	if err != nil {
		return &corev1.PersistentVolume{}, err
	}

	if err := r.Get(ctx, client.ObjectKey{
		Name: persistentVolumeClaim.Spec.VolumeName,
	}, persistentVolume); err != nil {
		log.Error(err, "unable to fetch source PersistentVolumeClaim")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return &corev1.PersistentVolume{}, err
	}

	return persistentVolume, nil
}

func (r *RookCephFSRefVolReconciler) deletePersistentVolume(ctx context.Context, log logr.Logger, inst *corev1.PersistentVolume) error {
	if err := r.Delete(ctx, inst); err != nil {
		log.Error(err, "While deleting ref volume")
		return err
	}
	return nil
}

func (r *RookCephFSRefVolReconciler) getRootPV(ctx context.Context, log logr.Logger, inst *corev1.PersistentVolume) error {
	return nil
}

// func (r *RookCephFSRefVolReconciler) handlePersistentVolumeEvent(persistentVolume client.Object) []reconcile.Request {
// 	labels := persistentVolume.GetLabels()
// 	if val, is_created_by_controller := labels["created-by"]; is_created_by_controller || val != controllerName {
// 		return []reconcile.Request{}
// 	}
// 	// doing sth with pv

// }
