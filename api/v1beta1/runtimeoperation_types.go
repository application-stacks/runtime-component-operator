/*


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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Defines the desired state of RuntimeOperation
type RuntimeOperationSpec struct {
	// Name of the Pod to perform runtime operation on. Pod must be from the same namespace as the RuntimeOperation instance.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Pod Name",xDescriptors="urn:alm:descriptor:com.tectonic.ui:text"
	PodName string `json:"podName"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container Name",xDescriptors="urn:alm:descriptor:com.tectonic.ui:text"
	ContainerName string `json:"containerName,omitempty"`

	// Command to execute. Not executed within a shell.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Command",xDescriptors="urn:alm:descriptor:com.tectonic.ui:text"
	Command []string `json:"command"`
}

// Defines the observed state of RuntimeOperation.
type RuntimeOperationStatus struct {
	// +listType=atomic
	Conditions []OperationStatusCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RuntimeOperation is the Schema for the runtimeoperations API.
type RuntimeOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RuntimeOperationSpec   `json:"spec,omitempty"`
	Status RuntimeOperationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RuntimeOperationList contains a list of RuntimeOperation.
type RuntimeOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RuntimeOperation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RuntimeOperation{}, &RuntimeOperationList{})
}
