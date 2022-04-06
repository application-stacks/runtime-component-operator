package common

import (
	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatusConditionType ...
type StatusConditionType string

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
}

// BaseComponentStatus returns base appplication status
type BaseComponentStatus interface {
	GetConditions() []StatusCondition
	GetCondition(StatusConditionType) StatusCondition
	SetCondition(StatusCondition)
	NewCondition() StatusCondition
	GetImageReference() string
	SetImageReference(string)
	GetBinding() *corev1.LocalObjectReference
	SetBinding(*corev1.LocalObjectReference)
}

const (
	// StatusConditionTypeReconciled ...
	StatusConditionTypeReconciled StatusConditionType = "Reconciled"
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
	GetBindable() *bool
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

// BaseComponentProbes describes the probes for application container
type BaseComponentProbes interface {
	GetLivenessProbe() *corev1.Probe
	GetReadinessProbe() *corev1.Probe
	GetStartupProbe() *corev1.Probe

	GetDefaultLivenessProbe(ba BaseComponent) *corev1.Probe
	GetDefaultReadinessProbe(ba BaseComponent) *corev1.Probe
	GetDefaultStartupProbe(ba BaseComponent) *corev1.Probe
}

// BaseComponent represents basic kubernetes application
type BaseComponent interface {
	GetApplicationImage() string
	GetPullPolicy() *corev1.PullPolicy
	GetPullSecret() *string
	GetServiceAccountName() *string
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
	GetSecurityContext() *corev1.SecurityContext
}
