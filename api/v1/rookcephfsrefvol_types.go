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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type RookCephFSRefVolState string

const (
	MetaGroup = "operator.dotv.home.arpa"
	// PV đã được tạo Ok
	Ok RookCephFSRefVolState = "Ok"
	// PV chưa được tạo
	Missing RookCephFSRefVolState = "Missing"
	// Conflict trong trường hợp trùng tên PV
	Conflict RookCephFSRefVolState = "Conflict"
	// ? Có trường hợp nào là Parent not found không?
	ParentNotFound RookCephFSRefVolState = "ParentNotFound"

	ParentDeleting RookCephFSRefVolState = "ParentDeleting"

	CreatedBy = MetaGroup + "/created-by"
	Parent    = MetaGroup + "/parent"
	IsParent  = MetaGroup + "/is-parent"
)

// RookCephFSRefVolSpec defines the desired state of RookCephFSRefVol
type RookCephFSRefVolSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of RookCephFSRefVol. Edit rookcephfsrefvol_types.go to remove/update
	// Tên của PVC muốn tham chiếu
	PvcName              string `json:"pvcName"`
	Namespace            string `json:"namespace"`
	CephFsUserSecretName string `json:"cephFsUserSecretName"`
	// userSecretName string `json:"userSecretName,omitempty"`
	// +optional
	// VolumeTemplates PersistentVolume `json:"volumeTemplates"`
}

// RookCephFSRefVolStatus defines the observed state of RookCephFSRefVol
type RookCephFSRefVolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	State    RookCephFSRefVolState `json:"state,omitempty"`
	Parent   string                `json:"parentPersistentVolume"`
	Children string                `json:"refVolumeName"`
	// RootPVName

}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Parent",type=string,JSONPath=`.status.parentPersistentVolume`
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
