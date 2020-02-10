package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpenLibertyDumpSpec defines the desired state of OpenLibertyDump
// +k8s:openapi-gen=true
type OpenLibertyDumpSpec struct {
	PodName string `json:"podName"`
	// +listType=set
	Include []OpenLibertyDumpInclude `json:"include,omitempty"`
}

// OpenLibertyDumpInclude defines the possible values for dump types
// +kubebuilder:validation:Enum=thread;heap;system
type OpenLibertyDumpInclude string

const (
	//OpenLibertyDumpIncludeHeap heap dump
	OpenLibertyDumpIncludeHeap OpenLibertyDumpInclude = "heap"
	//OpenLibertyDumpIncludeThread thread dump
	OpenLibertyDumpIncludeThread OpenLibertyDumpInclude = "thread"
	//OpenLibertyDumpIncludeSystem system (core) dump
	OpenLibertyDumpIncludeSystem OpenLibertyDumpInclude = "system"
)

// OpenLibertyDumpStatus defines the observed state of OpenLibertyDump
// +k8s:openapi-gen=true
type OpenLibertyDumpStatus struct {
	// +listType=atomic
	Conditions []OperationStatusCondition `json:"conditions,omitempty"`
	DumpFile   string                     `json:"dumpFile,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenLibertyDump is the Schema for the openlibertydumps API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=openlibertydumps,scope=Namespaced,shortName=oldump;oldumps
// +kubebuilder:printcolumn:name="Started",type="string",JSONPath=".status.conditions[?(@.type=='Started')].status",priority=0,description="Indicates if dump operation has started"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Started')].reason",priority=1,description="Reason for dump operation failing to start"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Started')].message",priority=1,description="Message for dump operation failing to start"
// +kubebuilder:printcolumn:name="Completed",type="string",JSONPath=".status.conditions[?(@.type=='Completed')].status",priority=0,description="Indicates if dump operation has completed"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Completed')].reason",priority=1,description="Reason for dump operation failing to complete"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Completed')].message",priority=1,description="Message for dump operation failing to complete"
// +kubebuilder:printcolumn:name="Dump file",type="string",JSONPath=".status.dumpFile",priority=0,description="Indicates filename of the server dump"
type OpenLibertyDump struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenLibertyDumpSpec   `json:"spec,omitempty"`
	Status OpenLibertyDumpStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenLibertyDumpList contains a list of OpenLibertyDump
type OpenLibertyDumpList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenLibertyDump `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenLibertyDump{}, &OpenLibertyDumpList{})
}
