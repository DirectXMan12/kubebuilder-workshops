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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RedisSpec defines the desired state of Redis
type RedisSpec struct {
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0

	// The number of follower instances to run.
	FollowerReplicas *int32 `json:"followerReplicas,omitempty"`
}

// RedisStatus defines the observed state of Redis
type RedisStatus struct {
	// The name of the service created for the Redis leader.
	LeaderService string `json:"leaderService"`
	// The name of the service created for the Redis follower.
	FollowerService string `json:"followerService"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".status.leaderService",name="Leader",type="string"
// +kubebuilder:printcolumn:JSONPath=".status.followerService",name="Follower",type="string"
// +kubebuilder:printcolumn:JSONPath=".spec.followerReplicas",name="Desired",type="integer"

// Redis is the Schema for the redis API
type Redis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSpec   `json:"spec,omitempty"`
	Status RedisStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisList contains a list of Redis
type RedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Redis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Redis{}, &RedisList{})
}
