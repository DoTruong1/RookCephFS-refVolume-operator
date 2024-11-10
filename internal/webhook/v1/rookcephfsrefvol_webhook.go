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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorv1 "github.com/DoTruong1/RookCephFS-refVolume-operator.git/api/v1"
)

// nolint:unused
// log is for logging in this package.
var rookcephfsrefvollog = logf.Log.WithName("rookcephfsrefvol-resource")

// SetupRookCephFSRefVolWebhookWithManager registers the webhook for RookCephFSRefVol in the manager.
func SetupRookCephFSRefVolWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&operatorv1.RookCephFSRefVol{}).
		WithValidator(&RookCephFSRefVolCustomValidator{}).
		WithDefaulter(&RookCephFSRefVolCustomDefaulter{
			DefaultNameSpace: "default",
		}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-operator-dotv-home-arpa-v1-rookcephfsrefvol,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.dotv.home.arpa,resources=rookcephfsrefvols,verbs=create;update,versions=v1,name=mrookcephfsrefvol-v1.kb.io,admissionReviewVersions=v1

// RookCephFSRefVolCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind RookCephFSRefVol when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type RookCephFSRefVolCustomDefaulter struct {
	DefaultNameSpace string
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &RookCephFSRefVolCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind RookCephFSRefVol.
func (d *RookCephFSRefVolCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	rookcephfsrefvol, ok := obj.(*operatorv1.RookCephFSRefVol)

	if !ok {
		return fmt.Errorf("expected an RookCephFSRefVol object but got %T", obj)
	}
	rookcephfsrefvollog.Info("Defaulting for RookCephFSRefVol", "name", rookcephfsrefvol.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}

func (d *RookCephFSRefVolCustomDefaulter) applyDefaults(rookcephfsrefvol *operatorv1.RookCephFSRefVol) {
	if rookcephfsrefvol.Spec.Namespace == "" {
		rookcephfsrefvol.Spec.Namespace = d.DefaultNameSpace
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-operator-dotv-home-arpa-v1-rookcephfsrefvol,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.dotv.home.arpa,resources=rookcephfsrefvols,verbs=create;update,versions=v1,name=vrookcephfsrefvol-v1.kb.io,admissionReviewVersions=v1

// RookCephFSRefVolCustomValidator struct is responsible for validating the RookCephFSRefVol resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type RookCephFSRefVolCustomValidator struct {
	//TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &RookCephFSRefVolCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type RookCephFSRefVol.
func (v *RookCephFSRefVolCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rookcephfsrefvol, ok := obj.(*operatorv1.RookCephFSRefVol)
	if !ok {
		return nil, fmt.Errorf("expected a RookCephFSRefVol object but got %T", obj)
	}
	rookcephfsrefvollog.Info("Validation for RookCephFSRefVol upon creation", "name", rookcephfsrefvol.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type RookCephFSRefVol.
func (v *RookCephFSRefVolCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	rookcephfsrefvol, ok := newObj.(*operatorv1.RookCephFSRefVol)
	if !ok {
		return nil, fmt.Errorf("expected a RookCephFSRefVol object for the newObj but got %T", newObj)
	}
	rookcephfsrefvollog.Info("Validation for RookCephFSRefVol upon update", "name", rookcephfsrefvol.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type RookCephFSRefVol.
func (v *RookCephFSRefVolCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rookcephfsrefvol, ok := obj.(*operatorv1.RookCephFSRefVol)
	if !ok {
		return nil, fmt.Errorf("expected a RookCephFSRefVol object but got %T", obj)
	}
	rookcephfsrefvollog.Info("Validation for RookCephFSRefVol upon deletion", "name", rookcephfsrefvol.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
