package common

import (
	routev1 "github.com/openshift/api/route/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// StatusConditionType ...
type StatusConditionType string

// StatusEndpointScope ...
type StatusEndpointScope string

type StatusReferences map[string]string

const (
	StatusReferenceCertSecretName    = "svcCertSecretName"
	StatusReferencePullSecretName    = "saPullSecretName"
	StatusReferenceSAResourceVersion = "saResourceVersion"
	StatusReferenceRouteHost         = "routeHost"
)

// StatusCondition ...
type StatusCondition interface {
	GetLastTransitionTime() *metav1.Time
	SetLastTransitionTime(*metav1.Time)

	GetReason() string
	SetReason(string)

	GetMessage() string
	SetMessage(string)

	GetStatus() corev1.ConditionStatus
	SetStatus(corev1.ConditionStatus)

	GetType() StatusConditionType
	SetType(StatusConditionType)

	SetConditionFields(string, string, corev1.ConditionStatus) StatusCondition
}

// StatusEndpoint ...
type StatusEndpoint interface {
	GetEndpointName() string
	SetEndpointName(string)

	GetEndpointScope() StatusEndpointScope
	SetEndpointScope(StatusEndpointScope)

	GetEndpointType() string
	SetEndpointType(string)

	GetEndpointUri() string
	SetEndpointUri(string)

	SetStatusEndpointFields(StatusEndpointScope, string, string) StatusEndpoint
}

// BaseComponentStatus returns base appplication status
type BaseComponentStatus interface {
	GetConditions() []StatusCondition
	GetCondition(StatusConditionType) StatusCondition
	SetCondition(StatusCondition)
	NewCondition(StatusConditionType) StatusCondition

	GetStatusEndpoint(string) StatusEndpoint
	SetStatusEndpoint(StatusEndpoint)
	NewStatusEndpoint(string) StatusEndpoint
	RemoveStatusEndpoint(string)

	GetImageReference() string
	SetImageReference(string)

	GetBinding() *corev1.LocalObjectReference
	SetBinding(*corev1.LocalObjectReference)

	GetReferences() StatusReferences
	SetReferences(StatusReferences)
	SetReference(string, string)
}

const (
	// Status Condition Types
	StatusConditionTypeReconciled     StatusConditionType = "Reconciled"
	StatusConditionTypeResourcesReady StatusConditionType = "ResourcesReady"
	StatusConditionTypeReady          StatusConditionType = "Ready"

	// Status Condition Type Messages
	StatusConditionTypeReadyMessage string = "Application is reconciled and resources are ready."

	// Status Endpoint Scopes
	StatusEndpointScopeExternal StatusEndpointScope = "External"
	StatusEndpointScopeInternal StatusEndpointScope = "Internal"
)

// BaseComponentAutoscaling represents basic HPA configuration
type BaseComponentAutoscaling interface {
	GetMinReplicas() *int32
	GetMaxReplicas() int32
	GetTargetCPUUtilizationPercentage() *int32
}

// BaseComponentStorage represents basic PVC configuration
type BaseComponentStorage interface {
	GetSize() string
	GetClassName() string
	GetMountPath() string
	GetVolumeClaimTemplate() *corev1.PersistentVolumeClaim
}

// BaseComponentService represents basic service configuration
type BaseComponentService interface {
	GetPort() int32
	GetTargetPort() *int32
	GetPortName() string
	GetType() *corev1.ServiceType
	GetNodePort() *int32
	GetPorts() []corev1.ServicePort
	GetAnnotations() map[string]string
	GetCertificateSecretRef() *string
	GetCertificate() BaseComponentCertificate
	GetBindable() *bool
}
type BaseComponentCertificate interface {
	GetAnnotations() map[string]string
}

// BaseComponentNetworkPolicy represents a basic network policy configuration
type BaseComponentNetworkPolicy interface {
	GetNamespaceLabels() map[string]string
	GetFromLabels() map[string]string
}

// BaseComponentMonitoring represents basic service monitoring configuration
type BaseComponentMonitoring interface {
	GetLabels() map[string]string
	GetEndpoints() []prometheusv1.Endpoint
}

// BaseComponentRoute represents route configuration
type BaseComponentRoute interface {
	GetTermination() *routev1.TLSTerminationType
	GetInsecureEdgeTerminationPolicy() *routev1.InsecureEdgeTerminationPolicyType
	GetAnnotations() map[string]string
	GetHost() string
	GetPath() string
	GetPathType() networkingv1.PathType
	GetCertificateSecretRef() *string
}

// BaseComponentAffinity describes deployment and pod affinity
type BaseComponentAffinity interface {
	GetNodeAffinity() *corev1.NodeAffinity
	GetPodAffinity() *corev1.PodAffinity
	GetPodAntiAffinity() *corev1.PodAntiAffinity
	GetArchitecture() []string
	GetNodeAffinityLabels() map[string]string
}

// BaseComponentDeployment describes deployment
type BaseComponentDeployment interface {
	GetDeploymentUpdateStrategy() *appsv1.DeploymentStrategy
	GetAnnotations() map[string]string
}

// BaseComponentStatefulSet describes deployment
type BaseComponentStatefulSet interface {
	GetStatefulSetUpdateStrategy() *appsv1.StatefulSetUpdateStrategy
	GetStorage() BaseComponentStorage
	GetAnnotations() map[string]string
}

// +kubebuilder:object:generate=true
type BaseComponentProbe struct {
	// The action taken to determine the health of a container
	BaseComponentProbeHandler `json:",inline"`
	// Number of seconds after the container has started before liveness probes are initiated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
	// Optional duration in seconds the pod needs to terminate gracefully upon probe failure.
	// The grace period is the duration in seconds after the processes running in the pod are sent
	// a termination signal and the time when the processes are forcibly halted with a kill signal.
	// Set this value longer than the expected cleanup time for your process.
	// If this value is nil, the pod's terminationGracePeriodSeconds will be used. Otherwise, this
	// value overrides the value provided by the pod spec.
	// Value must be non-negative integer. The value zero indicates stop immediately via
	// the kill signal (no opportunity to shut down).
	// This is a beta field and requires enabling ProbeTerminationGracePeriod feature gate.
	// Minimum value is 1. spec.terminationGracePeriodSeconds is used if unset.
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

// +kubebuilder:object:generate=true
type BaseComponentProbeHandler struct {
	// Exec specifies the action to take.
	// +optional
	Exec *corev1.ExecAction `json:"exec,omitempty"`
	// HTTPGet specifies the http request to perform.
	// +optional
	HTTPGet *OptionalHTTPGetAction `json:"httpGet,omitempty"`
	// TCPSocket specifies an action involving a TCP port.
	// +optional
	TCPSocket *corev1.TCPSocketAction `json:"tcpSocket,omitempty"`

	// GRPC specifies an action involving a GRPC port.
	// This is a beta field and requires enabling GRPCContainerProbe feature gate.
	// +featureGate=GRPCContainerProbe
	// +optional
	GRPC *corev1.GRPCAction `json:"grpc,omitempty"`
}

// +kubebuilder:object:generate=true
type OptionalHTTPGetAction struct {
	// Path to access on the HTTP server.
	// +optional
	Path string `json:"path,omitempty" protobuf:"bytes,1,opt,name=path"`
	// Name or number of the port to access on the container.
	// Number must be in the range 1 to 65535.
	// Name must be an IANA_SVC_NAME.
	// +optional
	Port intstr.IntOrString `json:"port" protobuf:"bytes,2,opt,name=port"`
	// Host name to connect to, defaults to the pod IP. You probably want to set
	// "Host" in httpHeaders instead.
	// +optional
	Host string `json:"host,omitempty" protobuf:"bytes,3,opt,name=host"`
	// Scheme to use for connecting to the host.
	// Defaults to HTTP.
	// +optional
	Scheme corev1.URIScheme `json:"scheme,omitempty" protobuf:"bytes,4,opt,name=scheme,casttype=URIScheme"`
	// Custom headers to set in the request. HTTP allows repeated headers.
	// +optional
	HTTPHeaders []corev1.HTTPHeader `json:"httpHeaders,omitempty" protobuf:"bytes,5,rep,name=httpHeaders"`
}

// BaseComponentProbes describes the probes for application container
type BaseComponentProbes interface {
	GetLivenessProbe() *BaseComponentProbe
	GetReadinessProbe() *BaseComponentProbe
	GetStartupProbe() *BaseComponentProbe

	GetDefaultLivenessProbe(ba BaseComponent) *BaseComponentProbe
	GetDefaultReadinessProbe(ba BaseComponent) *BaseComponentProbe
	GetDefaultStartupProbe(ba BaseComponent) *BaseComponentProbe
}

type BaseComponentServiceAccount interface {
	GetMountToken() *bool
	GetName() *string
}

type BaseComponentTopologySpreadConstraints interface {
	GetConstraints() *[]corev1.TopologySpreadConstraint
	GetDisableOperatorDefaults() *bool
}

// BaseComponent represents basic kubernetes application
type BaseComponent interface {
	GetApplicationImage() string
	GetPullPolicy() *corev1.PullPolicy
	GetPullSecret() *string
	GetServiceAccountName() *string
	GetServiceAccount() BaseComponentServiceAccount
	GetReplicas() *int32
	GetProbes() BaseComponentProbes
	GetVolumes() []corev1.Volume
	GetVolumeMounts() []corev1.VolumeMount
	GetResourceConstraints() *corev1.ResourceRequirements
	GetExpose() *bool
	GetEnv() []corev1.EnvVar
	GetEnvFrom() []corev1.EnvFromSource
	GetCreateKnativeService() *bool
	GetAutoscaling() BaseComponentAutoscaling
	GetService() BaseComponentService
	GetNetworkPolicy() BaseComponentNetworkPolicy
	GetDeployment() BaseComponentDeployment
	GetStatefulSet() BaseComponentStatefulSet
	GetApplicationVersion() string
	GetApplicationName() string
	GetMonitoring() BaseComponentMonitoring
	GetLabels() map[string]string
	GetAnnotations() map[string]string
	GetStatus() BaseComponentStatus
	GetInitContainers() []corev1.Container
	GetSidecarContainers() []corev1.Container
	GetGroupName() string
	GetRoute() BaseComponentRoute
	GetAffinity() BaseComponentAffinity
	GetTopologySpreadConstraints() BaseComponentTopologySpreadConstraints
	GetSecurityContext() *corev1.SecurityContext
	GetManageTLS() *bool
}
