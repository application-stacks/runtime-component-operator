package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RuntimeOperationSpec defines the desired state of RuntimeOperation
// +k8s:openapi-gen=true
type RuntimeOperationSpec struct {
	// Name of the Pod to perform runtime operation on. Pod must be from the same namespace as the RuntimeOperation instance
	PodName string `json:"podName"`

	// Name of the container. Defaults to main container "app"
	ContainerName string `json:"containerName,omitempty"`

	// Command to execute. Not executed within a shell
	Command []string `json:"command"`
}

// RuntimeOperationStatus defines the observed state of RuntimeOperation
// +k8s:openapi-gen=true
type RuntimeOperationStatus struct {
	// +listType=atomic
	Conditions []OperationStatusCondition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RuntimeOperation is the Schema for the runtimeoperation API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=runtimeoperations,scope=Namespaced,shortName=runtimeop;runtimeops
// +kubebuilder:printcolumn:name="Started",type="string",JSONPath=".status.conditions[?(@.type=='Started')].status",priority=0,description="Indicates if runtime operation has started"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Started')].reason",priority=1,description="Reason for runtime operation failing to start"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Started')].message",priority=1,description="Message for runtime operation failing to start"
// +kubebuilder:printcolumn:name="Completed",type="string",JSONPath=".status.conditions[?(@.type=='Completed')].status",priority=0,description="Indicates if runtime operation has completed"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Completed')].reason",priority=1,description="Reason for runtime operation failing to complete"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Completed')].message",priority=1,description="Message for runtime operation failing to complete"
type RuntimeOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RuntimeOperationSpec   `json:"spec,omitempty"`
	Status RuntimeOperationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RuntimeOperationList contains a list of RuntimeOperation
type RuntimeOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RuntimeOperation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RuntimeOperation{}, &RuntimeOperationList{})
}
