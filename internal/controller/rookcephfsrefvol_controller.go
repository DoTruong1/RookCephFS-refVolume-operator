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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RookCephFSRefVolReconciler reconciles a RookCephFSRefVol object
type RookCephFSRefVolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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

	var rookcephfsrefvols operatorv1.RookCephFSRefVol

	// fetch toàn bộ các rookcephfsrefvols
	if err := r.Get(ctx, req.NamespacedName, &rookcephfsrefvols); err != nil {
		log.Error(err, "unable to fetch CronJob")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	sourcePvcName := &rookcephfsrefvols.Spec.PvcName

	var persistentVolumeClaim corev1.PersistentVolumeClaim
	// var PersistentVolume corev1.PersistentVolume

	if err := r.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      *sourcePvcName,
	}, &persistentVolumeClaim); err != nil {
		log.Error(err, "unable to fetch source PersistentVolumeClaim")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)

	}

	log.Info("Thông tin PVC", "Volume name", persistentVolumeClaim.Spec.VolumeName)

	SourcePvName := &persistentVolumeClaim.Spec.VolumeName
	pv, err := getSourcePvManifest(r, ctx, SourcePvName)

	if err != nil {
		log.Error(err, "unable to fetch source PersistentVolume")

	}

	if pv != nil {
		log.Info("Thong tin PV", "PV info: ", pv.Name)

		refVolumeManifest := buildNewRefVolumeManifest(pv)

		err := r.Create(ctx, refVolumeManifest)

		if err != nil {
			log.Error(err, "Có lỗi xảy ra khi tạo ref volume")
			// we'll ignore not-found errors, since they can't be fixed by an immediate
			// requeue (we'll need to wait for a new notification), and we can get them
			// on deleted requests.
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RookCephFSRefVolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.RookCephFSRefVol{}). //specifies the type of resource to watch
		Complete(r)
}

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

func buildNewRefVolumeManifest(originalPv *corev1.PersistentVolume) *corev1.PersistentVolume {
	newPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-new-pv",
			Labels: map[string]string{
				"original-data-source": originalPv.Name,
			},
		},
		Spec: *originalPv.Spec.DeepCopy(),
	}
	if newPV.Spec.PersistentVolumeSource.CSI != nil {
		newPV.Spec.ClaimRef = nil
		newPV.Spec.PersistentVolumeSource.CSI.NodeStageSecretRef.Name = "rook-csi-cephfs-node-user"
		newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["staticVolume"] = "true"
		newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["rootPath"] = newPV.Spec.PersistentVolumeSource.CSI.VolumeAttributes["subvolumePath"]
		newPV.Spec.PersistentVolumeSource.CSI.VolumeHandle = newPV.Spec.PersistentVolumeSource.CSI.VolumeHandle + "-" + newPV.Name
		newPV.Spec.PersistentVolumeReclaimPolicy = "Retain"
	}
	return newPV
}
