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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type RookCephFSRefVolState string

const (
	MetaGroup      = "operator.dotv.home.arpa"
	ControllerName = "rookcephfsRefVolOperator"
	// PV đã được tạo Ok
	Bounded RookCephFSRefVolState = "Bounded"
	// PV chưa được tạo
	Missing RookCephFSRefVolState = "Missing"
	//
	Conflict RookCephFSRefVolState = "Conflict"
	// ? Có trường hợp nào là Parent not found không?
	ParentNotFound RookCephFSRefVolState = "ParentNotFound"
	//
	ParentDeleting RookCephFSRefVolState = "ParentDeleting"

	IsDeleting RookCephFSRefVolState = "Deleting"
	CreatedBy                        = MetaGroup + "/createdBy"
	Children                         = MetaGroup + "/children"
	Parent                           = MetaGroup + "/parent"
	IsParent                         = MetaGroup + "/is-parent"
)

type PvcInfo struct {
	// +required
	PvcName string `json:"pvcName"`
	// +required
	Namespace string `json:"namespace"`
}
type PvcSpec struct {
	CreateIfNotExists bool                                `json:"createIfNotExists,omitempty"`
	StorageClassName  *string                             `json:"storageClassName,omitempty"`
	AccessModes       []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	Resources         corev1.VolumeResourceRequirements   `json:"resources,omitempty"`
}
type DataSource struct {
	// +required
	// +kubebuilder:validation:Required
	PvcInfo PvcInfo `json:"pvcInfo,omitempty"`
	// +kubebuilder:validation:Required
	// +required
	PvcSpec PvcSpec `json:"pvcSpec"`
}
type Destination struct {
	// +required
	// +kubebuilder:validation:Required
	PvcInfo PvcInfo `json:"pvcInfo,omitempty"`
}

type RookCephFSRefVolSpec struct {
	// +required
	// +kubebuilder:validation:Required
	DataSource  DataSource  `json:"datasource"`
	Destination Destination `json:"destination"`
	// +kubebuilder:validation:Required
	CephFsUserSecretName string `json:"cephFsUserSecretName"`
}

type RookCephFSRefVolStatus struct {
	State    RookCephFSRefVolState `json:"state,omitempty"`
	Parent   string                `json:"parentPersistentVolume"`
	Children string                `json:"refVolumeName"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Parent",type=string,JSONPath=`.status.parentPersistentVolume`
// +kubebuilder:printcolumn:name="Children",type=string,JSONPath=`.status.refVolumeName`
// RookCephFSRefVol is the Schema for the rookcephfsrefvols API
type RookCephFSRefVol struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RookCephFSRefVolSpec   `json:"spec,omitempty"`
	Status RookCephFSRefVolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RookCephFSRefVolList contains a list of RookCephFSRefVol
type RookCephFSRefVolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RookCephFSRefVol `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RookCephFSRefVol{}, &RookCephFSRefVolList{})
}
