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

package v1

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"
)

// log is for logging in this package.
var rookcephfsrefvollog = logf.Log.WithName("rookcephfsrefvol-resource")

// SetupRookCephFSRefVolWebhookWithManager registers the webhook for RookCephFSRefVol in the manager.
func SetupRookCephFSRefVolWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&operatorv1.RookCephFSRefVol{}).
		WithValidator(&RookCephFSRefVolCustomValidator{}).
		WithDefaulter(&RookCephFSRefVolCustomDefaulter{
			DefaultNameSpace: "default",
		}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-operator-dotv-home-arpa-v1-rookcephfsrefvol,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.dotv.home.arpa,resources=rookcephfsrefvols,verbs=create;update,versions=v1,name=mrookcephfsrefvol-v1.kb.io,admissionReviewVersions=v1

type RookCephFSRefVolCustomDefaulter struct {
	DefaultNameSpace string
}

var _ webhook.CustomDefaulter = &RookCephFSRefVolCustomDefaulter{}

// Default sets default values for the RookCephFSRefVol resource.
func (d *RookCephFSRefVolCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	rookcephfsrefvol, ok := obj.(*operatorv1.RookCephFSRefVol)
	if !ok {
		return fmt.Errorf("expected a RookCephFSRefVol object but got %T", obj)
	}

	rookcephfsrefvollog.Info("Defaulting for RookCephFSRefVol", "name", rookcephfsrefvol.GetName())
	// Set default namespace if not provided
	if rookcephfsrefvol.Spec.DataSource.PvcInfo.Namespace == "" {
		rookcephfsrefvol.Spec.DataSource.PvcInfo.Namespace = d.DefaultNameSpace
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-operator-dotv-home-arpa-v1-rookcephfsrefvol,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.dotv.home.arpa,resources=rookcephfsrefvols,verbs=create;update;delete,versions=v1,name=vrookcephfsrefvol-v1.kb.io,admissionReviewVersions=v1

type RookCephFSRefVolCustomValidator struct{}

var _ webhook.CustomValidator = &RookCephFSRefVolCustomValidator{}

// ValidateCreate validates the RookCephFSRefVol resource upon creation.
func (v *RookCephFSRefVolCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rookcephfsrefvol, ok := obj.(*operatorv1.RookCephFSRefVol)
	if !ok {
		return nil, fmt.Errorf("expected a RookCephFSRefVol object but got %T", obj)
	}

	rookcephfsrefvollog.Info("Validation for RookCephFSRefVol upon creation", "name", rookcephfsrefvol.GetName())

	// Example: Ensure CephFsUserSecretName is not empty
	if rookcephfsrefvol.Spec.CephFsUserSecretName == "" {
		return nil, fmt.Errorf("CephFsUserSecretName cannot be empty")
	}

	return nil, nil
}

// ValidateUpdate blocks updates to the Spec of the RookCephFSRefVol resource but allows updates to metadata and status.
// ValidateUpdate allows updates to metadata, finalizers, and status, but blocks updates to the Spec of the RookCephFSRefVol resource.
func (v *RookCephFSRefVolCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	rookcephfsrefvolNew, ok := newObj.(*operatorv1.RookCephFSRefVol)
	if !ok {
		return nil, fmt.Errorf("expected a RookCephFSRefVol object for the newObj but got %T", newObj)
	}

	rookcephfsrefvolOld, ok := oldObj.(*operatorv1.RookCephFSRefVol)
	if !ok {
		return nil, fmt.Errorf("expected a RookCephFSRefVol object for the oldObj but got %T", oldObj)
	}

	rookcephfsrefvollog.Info("Validation for RookCephFSRefVol upon update", "name", rookcephfsrefvolNew.GetName())

	// Check if Spec has changed, if so reject the update
	if !reflect.DeepEqual(rookcephfsrefvolOld.Spec, rookcephfsrefvolNew.Spec) {
		return nil, fmt.Errorf("updates to Spec are not allowed for RookCephFSRefVol resources")
	}
	rookcephfsrefvollog.Info("Allow update status", "name", rookcephfsrefvolNew.GetName())

	return nil, nil
}

// ValidateDelete validates the RookCephFSRefVol resource upon deletion.
func (v *RookCephFSRefVolCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rookcephfsrefvol, ok := obj.(*operatorv1.RookCephFSRefVol)
	if !ok {
		return nil, fmt.Errorf("expected a RookCephFSRefVol object but got %T", obj)
	}

	rookcephfsrefvollog.Info("Validation for RookCephFSRefVol upon deletion", "name", rookcephfsrefvol.GetName())

	return nil, nil
}
