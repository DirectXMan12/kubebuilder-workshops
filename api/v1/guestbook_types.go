/*
Copyright 2019 Google LLC

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

// GuestBookSpec defines the desired state of GuestBook
type GuestBookSpec struct {
	Frontend  FrontendSpec `json:"frontend"`
	RedisName string       `json:"redisName,omitempty"`
}

type FrontendSpec struct {
	// +optional
	Resources corev1.ResourceRequirements `json:"resources"`

	// +optional
	// +kubebuilder:default=8080
	// +kubebuilder:validation:Minimum=0
	ServingPort int32 `json:"servingPort"`

	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`
}

// GuestBookStatus defines the observed state of GuestBook
type GuestBookStatus struct {
	URL string `json:"url"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.url",name="URL",type="string"
// +kubebuilder:printcolumn:JSONPath=".spec.frontend.replicas",name="Desired",type="integer"

// GuestBook is the Schema for the guestbooks API
type GuestBook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GuestBookSpec   `json:"spec,omitempty"`
	Status GuestBookStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GuestBookList contains a list of GuestBook
type GuestBookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GuestBook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GuestBook{}, &GuestBookList{})
}
