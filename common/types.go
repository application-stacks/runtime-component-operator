package common

import (
	routev1 "github.com/openshift/api/route/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
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
	UnsetCondition(StatusCondition)
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

	GetReconcileInterval() *int32
	SetReconcileInterval(*int32)
	UnsetReconcileInterval()

	GetLatestTransitionTime() *metav1.Time
}

const (
	// Status Condition Types
	StatusConditionTypeReconciled     StatusConditionType = "Reconciled"
	StatusConditionTypeResourcesReady StatusConditionType = "ResourcesReady"
	StatusConditionTypeReady          StatusConditionType = "Ready"
	StatusConditionTypeWarning        StatusConditionType = "Warning"

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

// BaseComponentKnativeService represents basic Knative service configuration
type BaseComponentKnativeService interface {
	GetTrafficTarget() []servingv1.TrafficTarget
	GetAutoscalingAnnotations() map[string]string
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

// BaseComponentProbes describes the probes for application container
type BaseComponentProbes interface {
	GetLivenessProbe() *corev1.Probe
	GetReadinessProbe() *corev1.Probe
	GetStartupProbe() *corev1.Probe

	GetDefaultLivenessProbe(ba BaseComponent) *corev1.Probe
	GetDefaultReadinessProbe(ba BaseComponent) *corev1.Probe
	GetDefaultStartupProbe(ba BaseComponent) *corev1.Probe
}

type BaseComponentServiceAccount interface {
	GetMountToken() *bool
	GetName() *string
}

type BaseComponentTopologySpreadConstraints interface {
	GetConstraints() *[]corev1.TopologySpreadConstraint
	GetDisableOperatorDefaults() *bool
}

type BaseComponentDNS interface {
	GetPolicy() *corev1.DNSPolicy
	GetConfig() *corev1.PodDNSConfig
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
	GetKnativeService() BaseComponentKnativeService
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
	GetDisableServiceLinks() *bool
	GetTolerations() []corev1.Toleration
	GetDNS() BaseComponentDNS
}
