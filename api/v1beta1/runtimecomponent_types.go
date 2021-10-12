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
	"time"

	"github.com/application-stacks/runtime-component-operator/common"
	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Defines the desired state of RuntimeComponent
type RuntimeComponentSpec struct {
	Version          string `json:"version,omitempty"`
	ApplicationImage string `json:"applicationImage"`
	Replicas         *int32 `json:"replicas,omitempty"`

	Autoscaling *RuntimeComponentAutoScaling `json:"autoscaling,omitempty"`

	// Policy for pulling container images. Defaults to IfNotPresent. Parameters autoscaling.maxReplicas and resourceConstraints.requests.cpu must be specified.
	PullPolicy *corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// Name of the Secret to use to pull images from the specified repository. It is not required if the cluster is configured with a global image pull secret.
	PullSecret *string `json:"pullSecret,omitempty"`

	// Represents a pod volume with data that is accessible to the containers.
	// +listType=map
	// +listMapKey=name
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Represents where to mount the volumes into containers.
	// +listType=atomic
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Limits the amount of required resources.
	ResourceConstraints *corev1.ResourceRequirements `json:"resourceConstraints,omitempty"`

	// Detects if the services are ready to serve.
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`

	// Detects if the services needs to be restarted.
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// Protects slow starting containers from livenessProbe.
	StartupProbe *corev1.Probe `json:"startupProbe,omitempty"`

	Service *RuntimeComponentService `json:"service,omitempty"`

	// A boolean that toggles the external exposure of this deployment via a Route or a Knative Route resource.
	Expose *bool `json:"expose,omitempty"`

	Deployment  *RuntimeComponentDeployment  `json:"deployment,omitempty"`
	StatefulSet *RuntimeComponentStatefulSet `json:"statefulSet,omitempty"`

	// An array of references to ConfigMap or Secret resources containing environment variables.
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// An array of environment variables following the format of {name, value}, where value is a simple string.
	// +listType=map
	// +listMapKey=name
	Env []corev1.EnvVar `json:"env,omitempty"`

	// The name of the OpenShift service account to be used during deployment.
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`

	// An array of architectures to be considered for deployment. Their position in the array indicates preference.
	// +listType=set
	Architecture []string `json:"architecture,omitempty"`

	Storage *RuntimeComponentStorage `json:"storage,omitempty"`

	// A boolean to toggle the creation of Knative resources and usage of Knative serving.
	CreateKnativeService *bool `json:"createKnativeService,omitempty"`

	Monitoring *RuntimeComponentMonitoring `json:"monitoring,omitempty"`

	// The name of the application this resource is part of. If not specified, it defaults to the name of the CR.
	ApplicationName string `json:"applicationName,omitempty"`

	// List of containers that run before other containers in a pod.
	// +listType=map
	// +listMapKey=name
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// The list of sidecar containers. These are additional containers to be added to the pods.
	// +listType=map
	// +listMapKey=name
	SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`

	Route    *RuntimeComponentRoute    `json:"route,omitempty"`
	Bindings *RuntimeComponentBindings `json:"bindings,omitempty"`
	Affinity *RuntimeComponentAffinity `json:"affinity,omitempty"`
}

// Configures a Pod to run on particular Nodes
type RuntimeComponentAffinity struct {
	// Controls which nodes the pod are scheduled to run on, based on labels on the node.
	NodeAffinity *corev1.NodeAffinity `json:"nodeAffinity,omitempty"`

	// Controls the nodes the pod are scheduled to run on, based on labels on the pods that are already running on the node.
	PodAffinity *corev1.PodAffinity `json:"podAffinity,omitempty"`

	// Enables the ability to prevent running a pod on the same node as another pod.
	PodAntiAffinity *corev1.PodAntiAffinity `json:"podAntiAffinity,omitempty"`

	// An array of architectures to be considered for deployment. Their position in the array indicates preference.
	// +listType=set
	Architecture []string `json:"architecture,omitempty"`

	// A YAML object that contains set of required labels and their values.
	NodeAffinityLabels map[string]string `json:"nodeAffinityLabels,omitempty"`
}

// Configures the desired resource consumption of pods
type RuntimeComponentAutoScaling struct {
	// Target average CPU utilization (represented as a percentage of requested CPU) over all the pods.
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// Lower limit for the number of pods that can be set by the autoscaler.
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// Required field for autoscaling. Upper limit for the number of pods that can be set by the autoscaler.
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas,omitempty"`
}

// Configures parameters for the network service of pods
type RuntimeComponentService struct {
	Type *corev1.ServiceType `json:"type,omitempty"`

	// The port exposed by the container.
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Minimum=1
	Port int32 `json:"port,omitempty"`

	// The port that the operator assigns to containers inside pods. Defaults to the value of service.port.
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Minimum=1
	TargetPort *int32 `json:"targetPort,omitempty"`

	// Node proxies this port into your service.
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Minimum=0
	NodePort *int32 `json:"nodePort,omitempty"`

	// The name for the port exposed by the container.
	PortName string `json:"portName,omitempty"`

	// An array consisting of service ports.
	Ports []corev1.ServicePort `json:"ports,omitempty"`

	// Annotations to be added to the service.
	Annotations map[string]string `json:"annotations,omitempty"`

	// +listType=atomic
	Consumes []ServiceBindingConsumes `json:"consumes,omitempty"`
	Provides *ServiceBindingProvides  `json:"provides,omitempty"`

	// 	A name of a secret that already contains TLS key, certificate and CA to be mounted in the pod.
	// +k8s:openapi-gen=true
	CertificateSecretRef *string `json:"certificateSecretRef,omitempty"`
}

// Defines the desired state and cycle of applications
type RuntimeComponentDeployment struct {
	// Specifies the strategy to replace old deployment pods with new pods
	UpdateStrategy *appsv1.DeploymentStrategy `json:"updateStrategy,omitempty"`
}

// Defines the desired state and cycle of stateful applications
type RuntimeComponentStatefulSet struct {
	// Specifies the strategy to replace old statefulSet pods with new pods
	UpdateStrategy *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
}

// Configures the OpenAPI information to expose
type ServiceBindingProvides struct {
	// Service binding type to be provided by this CR. At this time, the only allowed value is openapi.
	Category common.ServiceBindingCategory `json:"category"`

	// Specifies context root of the service.
	Context string `json:"context,omitempty"`

	// Protocol of the provided service. Defauts to http.
	Protocol string `json:"protocol,omitempty"`

	Auth *ServiceBindingAuth `json:"auth,omitempty"`
}

// Represents a service to be consumed
type ServiceBindingConsumes struct {
	// The name of the service to be consumed. If binding to a RuntimeComponent, then this would be the provider’s CR name.
	Name string `json:"name"`

	// The namespace of the service to be consumed. If binding to a RuntimeComponent, then this would be the provider’s CR namespace.
	Namespace string `json:"namespace,omitempty"`

	// The type of service binding to be consumed. At this time, the only allowed value is openapi.
	Category common.ServiceBindingCategory `json:"category"`

	// Optional field to specify which location in the pod, service binding secret should be mounted.
	MountPath string `json:"mountPath,omitempty"`
}

// Defines settings of persisted storage for StatefulSets
type RuntimeComponentStorage struct {
	// A convenient field to set the size of the persisted storage.
	// +kubebuilder:validation:Pattern=^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$
	Size string `json:"size,omitempty"`

	// The directory inside the container where this persisted storage will be bound to.
	MountPath string `json:"mountPath,omitempty"`

	// A YAML object that represents a volumeClaimTemplate component of a StatefulSet.
	VolumeClaimTemplate *corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
}

// Specifies parameters for Service Monitor
type RuntimeComponentMonitoring struct {
	// Labels to set on ServiceMonitor.
	Labels map[string]string `json:"labels,omitempty"`

	// A YAML snippet representing an array of Endpoint component from ServiceMonitor.
	Endpoints []prometheusv1.Endpoint `json:"endpoints,omitempty"`
}

// Configures the ingress resource
// +k8s:openapi-gen=true
type RuntimeComponentRoute struct {
	// Annotations to be added to the Route.
	Annotations map[string]string `json:"annotations,omitempty"`

	// TLS termination policy. Can be one of edge, reencrypt and passthrough.
	Termination *routev1.TLSTerminationType `json:"termination,omitempty"`

	// HTTP traffic policy with TLS enabled. Can be one of Allow, Redirect and None.
	InsecureEdgeTerminationPolicy *routev1.InsecureEdgeTerminationPolicyType `json:"insecureEdgeTerminationPolicy,omitempty"`

	// A name of a secret that already contains TLS key, certificate and CA to be used in the route. Also can contain destination CA certificate.
	CertificateSecretRef *string `json:"certificateSecretRef,omitempty"`

	// Hostname to be used for the Route.
	Host string `json:"host,omitempty"`

	// Path to be used for Route.
	Path string `json:"path,omitempty"`
}

// Allows a service to provide authentication information
type ServiceBindingAuth struct {
	// The secret that contains the username for authenticating
	Username corev1.SecretKeySelector `json:"username,omitempty"`
	// The secret that contains the password for authenticating
	Password corev1.SecretKeySelector `json:"password,omitempty"`
}

// Represents service binding related parameters
type RuntimeComponentBindings struct {
	// A boolean to toggle whether the operator should automatically detect and use a ServiceBindingRequest resource with <CR_NAME>-binding naming format.
	AutoDetect *bool `json:"autoDetect,omitempty"`

	// The name of a ServiceBindingRequest custom resource created manually in the same namespace as the application.
	ResourceRef string `json:"resourceRef,omitempty"`

	// A YAML object that represents a ServiceBindingRequest custom resource.
	Embedded *runtime.RawExtension `json:"embedded,omitempty"`

	Expose *RuntimeComponentBindingExpose `json:"expose,omitempty"`
}

// Encapsulates information exposed by the application
type RuntimeComponentBindingExpose struct {
	// A boolean to toggle whether the operator expose the application as a bindable service. The default value for this parameter is false.
	Enabled *bool `json:"enabled,omitempty"`
}

// Defines the observed state of RuntimeComponent
type RuntimeComponentStatus struct {
	// +listType=atomic
	Conditions       []StatusCondition       `json:"conditions,omitempty"`
	ConsumedServices common.ConsumedServices `json:"consumedServices,omitempty"`
	// +listType=set
	ResolvedBindings []string                     `json:"resolvedBindings,omitempty"`
	ImageReference   string                       `json:"imageReference,omitempty"`
	Binding          *corev1.LocalObjectReference `json:"binding,omitempty"`
}

// Defines possible status conditions
type StatusCondition struct {
	LastTransitionTime *metav1.Time           `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     metav1.Time            `json:"lastUpdateTime,omitempty"`
	Reason             string                 `json:"reason,omitempty"`
	Message            string                 `json:"message,omitempty"`
	Status             corev1.ConditionStatus `json:"status,omitempty"`
	Type               StatusConditionType    `json:"type,omitempty"`
}

// Defines the type of status condition
type StatusConditionType string

const (
	// StatusConditionTypeReconciled ...
	StatusConditionTypeReconciled StatusConditionType = "Reconciled"

	// StatusConditionTypeDependenciesSatisfied ...
	StatusConditionTypeDependenciesSatisfied StatusConditionType = "DependenciesSatisfied"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// The Schema for the runtimecomponents API
type RuntimeComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RuntimeComponentSpec   `json:"spec,omitempty"`
	Status RuntimeComponentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// Contains a list of RuntimeComponent
type RuntimeComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RuntimeComponent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RuntimeComponent{}, &RuntimeComponentList{})
}

// GetApplicationImage returns application image
func (cr *RuntimeComponent) GetApplicationImage() string {
	return cr.Spec.ApplicationImage
}

// GetPullPolicy returns image pull policy
func (cr *RuntimeComponent) GetPullPolicy() *corev1.PullPolicy {
	return cr.Spec.PullPolicy
}

// GetPullSecret returns secret name for docker registry credentials
func (cr *RuntimeComponent) GetPullSecret() *string {
	return cr.Spec.PullSecret
}

// GetServiceAccountName returns service account name
func (cr *RuntimeComponent) GetServiceAccountName() *string {
	return cr.Spec.ServiceAccountName
}

// GetReplicas returns number of replicas
func (cr *RuntimeComponent) GetReplicas() *int32 {
	return cr.Spec.Replicas
}

// GetLivenessProbe returns liveness probe
func (cr *RuntimeComponent) GetLivenessProbe() *corev1.Probe {
	return cr.Spec.LivenessProbe
}

// GetReadinessProbe returns readiness probe
func (cr *RuntimeComponent) GetReadinessProbe() *corev1.Probe {
	return cr.Spec.ReadinessProbe
}

// GetStartupProbe returns startup probe
func (cr *RuntimeComponent) GetStartupProbe() *corev1.Probe {
	return cr.Spec.StartupProbe
}

// GetVolumes returns volumes slice
func (cr *RuntimeComponent) GetVolumes() []corev1.Volume {
	return cr.Spec.Volumes
}

// GetVolumeMounts returns volume mounts slice
func (cr *RuntimeComponent) GetVolumeMounts() []corev1.VolumeMount {
	return cr.Spec.VolumeMounts
}

// GetResourceConstraints returns resource constraints
func (cr *RuntimeComponent) GetResourceConstraints() *corev1.ResourceRequirements {
	return cr.Spec.ResourceConstraints
}

// GetExpose returns expose flag
func (cr *RuntimeComponent) GetExpose() *bool {
	return cr.Spec.Expose
}

// GetEnv returns slice of environment variables
func (cr *RuntimeComponent) GetEnv() []corev1.EnvVar {
	return cr.Spec.Env
}

// GetEnvFrom returns slice of environment variables from source
func (cr *RuntimeComponent) GetEnvFrom() []corev1.EnvFromSource {
	return cr.Spec.EnvFrom
}

// GetCreateKnativeService returns flag that toggles Knative service
func (cr *RuntimeComponent) GetCreateKnativeService() *bool {
	return cr.Spec.CreateKnativeService
}

// GetArchitecture returns slice of architectures
func (cr *RuntimeComponent) GetArchitecture() []string {
	return cr.Spec.Architecture
}

// GetAutoscaling returns autoscaling settings
func (cr *RuntimeComponent) GetAutoscaling() common.BaseComponentAutoscaling {
	if cr.Spec.Autoscaling == nil {
		return nil
	}
	return cr.Spec.Autoscaling
}

// GetStorage returns storage settings
func (cr *RuntimeComponent) GetStorage() common.BaseComponentStorage {
	if cr.Spec.Storage == nil {
		return nil
	}
	return cr.Spec.Storage
}

// GetService returns service settings
func (cr *RuntimeComponent) GetService() common.BaseComponentService {
	if cr.Spec.Service == nil {
		return nil
	}
	return cr.Spec.Service
}

// GetVersion returns application version
func (cr *RuntimeComponent) GetVersion() string {
	return cr.Spec.Version
}

// GetApplicationName returns Application name to be used for integration with kAppNav
func (cr *RuntimeComponent) GetApplicationName() string {
	return cr.Spec.ApplicationName
}

// GetMonitoring returns monitoring settings
func (cr *RuntimeComponent) GetMonitoring() common.BaseComponentMonitoring {
	if cr.Spec.Monitoring == nil {
		return nil
	}
	return cr.Spec.Monitoring
}

// GetStatus returns RuntimeComponent status
func (cr *RuntimeComponent) GetStatus() common.BaseComponentStatus {
	return &cr.Status
}

// GetInitContainers returns list of init containers
func (cr *RuntimeComponent) GetInitContainers() []corev1.Container {
	return cr.Spec.InitContainers
}

// GetSidecarContainers returns list of user specified containers
func (cr *RuntimeComponent) GetSidecarContainers() []corev1.Container {
	return cr.Spec.SidecarContainers
}

// GetGroupName returns group name to be used in labels and annotation
func (cr *RuntimeComponent) GetGroupName() string {
	return "app.stacks"
}

// GetRoute returns route configuration for RuntimeComponent
func (cr *RuntimeComponent) GetRoute() common.BaseComponentRoute {
	if cr.Spec.Route == nil {
		return nil
	}
	return cr.Spec.Route
}

// GetBindings returns binding configuration for RuntimeComponent
func (cr *RuntimeComponent) GetBindings() common.BaseComponentBindings {
	if cr.Spec.Bindings == nil {
		return nil
	}
	return cr.Spec.Bindings
}

// GetAffinity returns deployment's node and pod affinity settings
func (cr *RuntimeComponent) GetAffinity() common.BaseComponentAffinity {
	if cr.Spec.Affinity == nil {
		return nil
	}
	return cr.Spec.Affinity
}

// GetDeployment returns deployment settings
func (cr *RuntimeComponent) GetDeployment() common.BaseComponentDeployment {
	if cr.Spec.Deployment == nil {
		return nil
	}
	return cr.Spec.Deployment
}

// GetDeploymentStrategy returns deployment strategy struct
func (cr *RuntimeComponentDeployment) GetDeploymentUpdateStrategy() *appsv1.DeploymentStrategy {
	return cr.UpdateStrategy
}

// GetStatefulSet returns statefulSet settings
func (cr *RuntimeComponent) GetStatefulSet() common.BaseComponentStatefulSet {
	if cr.Spec.StatefulSet == nil {
		return nil
	}
	return cr.Spec.StatefulSet
}

// GetStatefulSetUpdateStrategy returns statefulSet strategy struct
func (cr *RuntimeComponentStatefulSet) GetStatefulSetUpdateStrategy() *appsv1.StatefulSetUpdateStrategy {
	return cr.UpdateStrategy
}

// GetResolvedBindings returns a map of all the service names to be consumed by the application
func (s *RuntimeComponentStatus) GetResolvedBindings() []string {
	return s.ResolvedBindings
}

// SetResolvedBindings sets ConsumedServices
func (s *RuntimeComponentStatus) SetResolvedBindings(rb []string) {
	s.ResolvedBindings = rb
}

// GetConsumedServices returns a map of all the service names to be consumed by the application
func (s *RuntimeComponentStatus) GetConsumedServices() common.ConsumedServices {
	if s.ConsumedServices == nil {
		return nil
	}
	return s.ConsumedServices
}

// SetConsumedServices sets ConsumedServices
func (s *RuntimeComponentStatus) SetConsumedServices(c common.ConsumedServices) {
	s.ConsumedServices = c
}

// GetImageReference returns Docker image reference to be deployed by the CR
func (s *RuntimeComponentStatus) GetImageReference() string {
	return s.ImageReference
}

// SetImageReference sets Docker image reference on the status portion of the CR
func (s *RuntimeComponentStatus) SetImageReference(imageReference string) {
	s.ImageReference = imageReference
}

// GetBinding returns BindingStatus representing binding status
func (s *RuntimeComponentStatus) GetBinding() *corev1.LocalObjectReference {
	return s.Binding
}

// SetBinding sets BindingStatus representing binding status
func (s *RuntimeComponentStatus) SetBinding(r *corev1.LocalObjectReference) {
	s.Binding = r
}

// GetMinReplicas returns minimum replicas
func (a *RuntimeComponentAutoScaling) GetMinReplicas() *int32 {
	return a.MinReplicas
}

// GetMaxReplicas returns maximum replicas
func (a *RuntimeComponentAutoScaling) GetMaxReplicas() int32 {
	return a.MaxReplicas
}

// GetTargetCPUUtilizationPercentage returns target cpu usage
func (a *RuntimeComponentAutoScaling) GetTargetCPUUtilizationPercentage() *int32 {
	return a.TargetCPUUtilizationPercentage
}

// GetSize returns persistent volume size
func (s *RuntimeComponentStorage) GetSize() string {
	return s.Size
}

// GetMountPath returns mount path for persistent volume
func (s *RuntimeComponentStorage) GetMountPath() string {
	return s.MountPath
}

// GetVolumeClaimTemplate returns a template representing requested persistent volume
func (s *RuntimeComponentStorage) GetVolumeClaimTemplate() *corev1.PersistentVolumeClaim {
	return s.VolumeClaimTemplate
}

// GetAnnotations returns a set of annotations to be added to the service
func (s *RuntimeComponentService) GetAnnotations() map[string]string {
	return s.Annotations
}

// GetPort returns service port
func (s *RuntimeComponentService) GetPort() int32 {
	return s.Port
}

// GetNodePort returns service nodePort
func (s *RuntimeComponentService) GetNodePort() *int32 {
	if s.NodePort == nil {
		return nil
	}
	return s.NodePort
}

// GetTargetPort returns the internal target port for containers
func (s *RuntimeComponentService) GetTargetPort() *int32 {
	if s.TargetPort == nil {
		return nil
	}

	return s.TargetPort
}

// GetPortName returns name of service port
func (s *RuntimeComponentService) GetPortName() string {
	return s.PortName
}

// GetType returns service type
func (s *RuntimeComponentService) GetType() *corev1.ServiceType {
	return s.Type
}

// GetPorts returns a list of service ports
func (s *RuntimeComponentService) GetPorts() []corev1.ServicePort {
	return s.Ports
}

// GetProvides returns service provider configuration
func (s *RuntimeComponentService) GetProvides() common.ServiceBindingProvides {
	if s.Provides == nil {
		return nil
	}
	return s.Provides
}

// GetCertificateSecretRef returns a secret reference with a certificate
func (s *RuntimeComponentService) GetCertificateSecretRef() *string {
	return s.CertificateSecretRef
}

// GetCategory returns category of a service provider configuration
func (p *ServiceBindingProvides) GetCategory() common.ServiceBindingCategory {
	return p.Category
}

// GetContext returns context of a service provider configuration
func (p *ServiceBindingProvides) GetContext() string {
	return p.Context
}

// GetAuth returns secret of a service provider configuration
func (p *ServiceBindingProvides) GetAuth() common.ServiceBindingAuth {
	if p.Auth == nil {
		return nil
	}
	return p.Auth
}

// GetProtocol returns protocol of a service provider configuration
func (p *ServiceBindingProvides) GetProtocol() string {
	return p.Protocol
}

// GetConsumes returns a list of service consumers' configuration
func (s *RuntimeComponentService) GetConsumes() []common.ServiceBindingConsumes {
	consumes := make([]common.ServiceBindingConsumes, len(s.Consumes))
	for i := range s.Consumes {
		consumes[i] = &s.Consumes[i]
	}
	return consumes
}

// GetName returns service name of a service consumer configuration
func (c *ServiceBindingConsumes) GetName() string {
	return c.Name
}

// GetNamespace returns namespace of a service consumer configuration
func (c *ServiceBindingConsumes) GetNamespace() string {
	return c.Namespace
}

// GetCategory returns category of a service consumer configuration
func (c *ServiceBindingConsumes) GetCategory() common.ServiceBindingCategory {
	return common.ServiceBindingCategoryOpenAPI
}

// GetMountPath returns mount path of a service consumer configuration
func (c *ServiceBindingConsumes) GetMountPath() string {
	return c.MountPath
}

// GetUsername returns username of a service binding auth object
func (a *ServiceBindingAuth) GetUsername() corev1.SecretKeySelector {
	return a.Username
}

// GetPassword returns password of a service binding auth object
func (a *ServiceBindingAuth) GetPassword() corev1.SecretKeySelector {
	return a.Password
}

// GetLabels returns labels to be added on ServiceMonitor
func (m *RuntimeComponentMonitoring) GetLabels() map[string]string {
	return m.Labels
}

// GetEndpoints returns endpoints to be added to ServiceMonitor
func (m *RuntimeComponentMonitoring) GetEndpoints() []prometheusv1.Endpoint {
	return m.Endpoints
}

// GetAnnotations returns route annotations
func (r *RuntimeComponentRoute) GetAnnotations() map[string]string {
	return r.Annotations
}

// GetCertificateSecretRef returns a secret reference with a certificate
func (r *RuntimeComponentRoute) GetCertificateSecretRef() *string {
	return r.CertificateSecretRef
}

// GetTermination returns terminatation of the route's TLS
func (r *RuntimeComponentRoute) GetTermination() *routev1.TLSTerminationType {
	return r.Termination
}

// GetInsecureEdgeTerminationPolicy returns terminatation of the route's TLS
func (r *RuntimeComponentRoute) GetInsecureEdgeTerminationPolicy() *routev1.InsecureEdgeTerminationPolicyType {
	return r.InsecureEdgeTerminationPolicy
}

// GetHost returns hostname to be used by the route
func (r *RuntimeComponentRoute) GetHost() string {
	return r.Host
}

// GetPath returns path to use for the route
func (r *RuntimeComponentRoute) GetPath() string {
	return r.Path
}

// GetAutoDetect returns a boolean to specify if the operator should auto-detect ServiceBinding CRs with the same name as the RuntimeComponent CR
func (r *RuntimeComponentBindings) GetAutoDetect() *bool {
	return r.AutoDetect
}

// GetResourceRef returns name of ServiceBinding CRs created manually in the same namespace as the RuntimeComponent CR
func (r *RuntimeComponentBindings) GetResourceRef() string {
	return r.ResourceRef
}

// GetEmbedded returns the embedded underlying Service Binding resource
func (r *RuntimeComponentBindings) GetEmbedded() *runtime.RawExtension {
	return r.Embedded
}

// GetExpose returns the map used making this application a bindable service
func (r *RuntimeComponentBindings) GetExpose() common.BaseComponentExpose {
	if r.Expose == nil {
		return nil
	}
	return r.Expose
}

// GetEnabled returns whether the application should be exposable as a service
func (e *RuntimeComponentBindingExpose) GetEnabled() *bool {
	return e.Enabled
}

// GetNodeAffinity returns node affinity
func (a *RuntimeComponentAffinity) GetNodeAffinity() *corev1.NodeAffinity {
	return a.NodeAffinity
}

// GetPodAffinity returns pod affinity
func (a *RuntimeComponentAffinity) GetPodAffinity() *corev1.PodAffinity {
	return a.PodAffinity
}

// GetPodAntiAffinity returns pod anti-affinity
func (a *RuntimeComponentAffinity) GetPodAntiAffinity() *corev1.PodAntiAffinity {
	return a.PodAntiAffinity
}

// GetArchitecture returns list of architecture names
func (a *RuntimeComponentAffinity) GetArchitecture() []string {
	return a.Architecture
}

// GetNodeAffinityLabels returns list of architecture names
func (a *RuntimeComponentAffinity) GetNodeAffinityLabels() map[string]string {
	return a.NodeAffinityLabels
}

// Initialize the RuntimeComponent instance
func (cr *RuntimeComponent) Initialize() {
	if cr.Spec.PullPolicy == nil {
		pp := corev1.PullIfNotPresent
		cr.Spec.PullPolicy = &pp
	}

	if cr.Spec.ResourceConstraints == nil {
		cr.Spec.ResourceConstraints = &corev1.ResourceRequirements{}
	}

	// Default applicationName to cr.Name, if a user sets createAppDefinition to true but doesn't set applicationName
	if cr.Spec.ApplicationName == "" {
		if cr.Labels != nil && cr.Labels["app.kubernetes.io/part-of"] != "" {
			cr.Spec.ApplicationName = cr.Labels["app.kubernetes.io/part-of"]
		} else {
			cr.Spec.ApplicationName = cr.Name
		}
	}

	if cr.Labels != nil {
		cr.Labels["app.kubernetes.io/part-of"] = cr.Spec.ApplicationName
	}

	// This is to handle when there is no service in the CR
	if cr.Spec.Service == nil {
		cr.Spec.Service = &RuntimeComponentService{}
	}

	if cr.Spec.Service.Type == nil {
		st := corev1.ServiceTypeClusterIP
		cr.Spec.Service.Type = &st
	}

	if cr.Spec.Service.Port == 0 {
		cr.Spec.Service.Port = 8080
	}

	if cr.Spec.Service.Provides != nil && cr.Spec.Service.Provides.Protocol == "" {
		cr.Spec.Service.Provides.Protocol = "http"
	}

}

// GetLabels returns set of labels to be added to all resources
func (cr *RuntimeComponent) GetLabels() map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/instance":   cr.Name,
		"app.kubernetes.io/name":       cr.Name,
		"app.kubernetes.io/managed-by": "runtime-component-operator",
		"app.kubernetes.io/component":  "backend",
		"app.kubernetes.io/part-of":    cr.Spec.ApplicationName,
	}

	if cr.Spec.Version != "" {
		labels["app.kubernetes.io/version"] = cr.Spec.Version
	}

	for key, value := range cr.Labels {
		if key != "app.kubernetes.io/instance" {
			labels[key] = value
		}
	}

	if cr.Spec.Service != nil && cr.Spec.Service.Provides != nil {
		labels["service.app.stacks/bindable"] = "true"
	}

	return labels
}

// GetAnnotations returns set of annotations to be added to all resources
func (cr *RuntimeComponent) GetAnnotations() map[string]string {
	annotations := map[string]string{}
	for k, v := range cr.Annotations {
		annotations[k] = v
	}
	delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
	return annotations
}

// GetType returns status condition type
func (c *StatusCondition) GetType() common.StatusConditionType {
	return convertToCommonStatusConditionType(c.Type)
}

// SetType returns status condition type
func (c *StatusCondition) SetType(ct common.StatusConditionType) {
	c.Type = convertFromCommonStatusConditionType(ct)
}

// GetLastTransitionTime return time of last status change
func (c *StatusCondition) GetLastTransitionTime() *metav1.Time {
	return c.LastTransitionTime
}

// SetLastTransitionTime sets time of last status change
func (c *StatusCondition) SetLastTransitionTime(t *metav1.Time) {
	c.LastTransitionTime = t
}

// GetLastUpdateTime return time of last status update
func (c *StatusCondition) GetLastUpdateTime() metav1.Time {
	return c.LastUpdateTime
}

// SetLastUpdateTime sets time of last status update
func (c *StatusCondition) SetLastUpdateTime(t metav1.Time) {
	c.LastUpdateTime = t
}

// GetMessage return condition's message
func (c *StatusCondition) GetMessage() string {
	return c.Message
}

// SetMessage sets condition's message
func (c *StatusCondition) SetMessage(m string) {
	c.Message = m
}

// GetReason return condition's message
func (c *StatusCondition) GetReason() string {
	return c.Reason
}

// SetReason sets condition's reason
func (c *StatusCondition) SetReason(r string) {
	c.Reason = r
}

// GetStatus return condition's status
func (c *StatusCondition) GetStatus() corev1.ConditionStatus {
	return c.Status
}

// SetStatus sets condition's status
func (c *StatusCondition) SetStatus(s corev1.ConditionStatus) {
	c.Status = s
}

// NewCondition returns new condition
func (s *RuntimeComponentStatus) NewCondition() common.StatusCondition {
	return &StatusCondition{}
}

// GetConditions returns slice of conditions
func (s *RuntimeComponentStatus) GetConditions() []common.StatusCondition {
	var conditions = make([]common.StatusCondition, len(s.Conditions))
	for i := range s.Conditions {
		conditions[i] = &s.Conditions[i]
	}
	return conditions
}

// GetCondition ...
func (s *RuntimeComponentStatus) GetCondition(t common.StatusConditionType) common.StatusCondition {
	for i := range s.Conditions {
		if s.Conditions[i].GetType() == t {
			return &s.Conditions[i]
		}
	}
	return nil
}

// SetCondition ...
func (s *RuntimeComponentStatus) SetCondition(c common.StatusCondition) {
	condition := &StatusCondition{}
	found := false
	for i := range s.Conditions {
		if s.Conditions[i].GetType() == c.GetType() {
			condition = &s.Conditions[i]
			found = true
		}
	}

	if condition.GetStatus() != c.GetStatus() {
		condition.SetLastTransitionTime(&metav1.Time{Time: time.Now()})
	}

	condition.SetLastUpdateTime(metav1.Time{Time: time.Now()})
	condition.SetReason(c.GetReason())
	condition.SetMessage(c.GetMessage())
	condition.SetStatus(c.GetStatus())
	condition.SetType(c.GetType())
	if !found {
		s.Conditions = append(s.Conditions, *condition)
	}
}

func convertToCommonStatusConditionType(c StatusConditionType) common.StatusConditionType {
	switch c {
	case StatusConditionTypeReconciled:
		return common.StatusConditionTypeReconciled
	case StatusConditionTypeDependenciesSatisfied:
		return common.StatusConditionTypeDependenciesSatisfied
	default:
		panic(c)
	}
}

func convertFromCommonStatusConditionType(c common.StatusConditionType) StatusConditionType {
	switch c {
	case common.StatusConditionTypeReconciled:
		return StatusConditionTypeReconciled
	case common.StatusConditionTypeDependenciesSatisfied:
		return StatusConditionTypeDependenciesSatisfied
	default:
		panic(c)
	}
}
