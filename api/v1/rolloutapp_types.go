/*
Copyright 2025.

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

// RolloutAppSpec defines the desired state of RolloutApp.
type RolloutAppSpec struct {
	// StatefulSetName is the name of the StatefulSet to be rolled out
	StatefulSetName string `json:"statefulSetName"`
	// Namespace is the namespace where the StatefulSet is located
	Namespace string `json:"namespace"`
	//RaftUrl is the URL for the Raft cluster
	RaftUrl string `json:"rafturl"`
	// +kubebuilder:default=3
	// InitPartition is the initial partition size for the rollout
	InitPartition int32 `json:"initPartition"`

	// +kubebuilder:default=false
	// Suspend prevents the operator from continuing rollout
	Suspend bool `json:"suspend,omitempty"`
}

// RolloutAppStatus defines the observed state of RolloutApp.
type RolloutAppStatus struct {
	Phase            string `json:"phase"`
	CurrentPartition int32  `json:"currentPartition"`
	LastUpdated      string `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RolloutApp is the Schema for the rolloutapps API.
type RolloutApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RolloutAppSpec   `json:"spec,omitempty"`
	Status RolloutAppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RolloutAppList contains a list of RolloutApp.
type RolloutAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RolloutApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RolloutApp{}, &RolloutAppList{})
}
