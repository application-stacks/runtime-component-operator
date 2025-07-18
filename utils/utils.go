package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientcfg "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/application-stacks/runtime-component-operator/common"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const RCOOperandVersion = "1.4.4"

var APIVersionNotFoundError = errors.New("APIVersion is not available")

// CustomizeDeployment ...
func CustomizeDeployment(deploy *appsv1.Deployment, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	deploy.Labels = ba.GetLabels()
	deploy.Annotations = MergeMaps(deploy.Annotations, ba.GetAnnotations())

	if ba.GetAutoscaling() == nil {
		deploy.Spec.Replicas = ba.GetReplicas()
	}

	if deploy.Spec.Selector == nil {
		deploy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/instance": obj.GetName(),
			},
		}
	}

	dp := ba.GetDeployment()
	if dp != nil && dp.GetDeploymentUpdateStrategy() != nil {
		deploy.Spec.Strategy = *dp.GetDeploymentUpdateStrategy()
	} else {
		deploy.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}
	}
	if dp != nil && dp.GetAnnotations() != nil {
		deploy.Annotations = MergeMaps(deploy.Annotations, dp.GetAnnotations())
	}

}

// CustomizeStatefulSet ...
func CustomizeStatefulSet(statefulSet *appsv1.StatefulSet, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	statefulSet.Labels = ba.GetLabels()
	statefulSet.Annotations = MergeMaps(statefulSet.Annotations, ba.GetAnnotations())

	if ba.GetAutoscaling() == nil {
		statefulSet.Spec.Replicas = ba.GetReplicas()
	}
	statefulSet.Spec.ServiceName = obj.GetName() + "-headless"
	if statefulSet.Spec.Selector == nil {
		statefulSet.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/instance": obj.GetName(),
			},
		}
	}

	ss := ba.GetStatefulSet()
	if ss != nil {
		if ss.GetStatefulSetUpdateStrategy() != nil {
			statefulSet.Spec.UpdateStrategy = *ss.GetStatefulSetUpdateStrategy()
		} else {
			statefulSet.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType}
		}
	}
	if ss != nil && ss.GetAnnotations() != nil {
		statefulSet.Annotations = MergeMaps(statefulSet.Annotations, ss.GetAnnotations())
	}
}

// CustomizeRoute ...
func CustomizeRoute(route *routev1.Route, ba common.BaseComponent, key string, crt string, ca string, destCACert string) {
	obj := ba.(metav1.Object)
	route.Labels = ba.GetLabels()
	route.Annotations = MergeMaps(route.Annotations, ba.GetAnnotations())

	if ba.GetRoute() != nil {
		rt := ba.GetRoute()
		route.Annotations = MergeMaps(route.Annotations, rt.GetAnnotations())

		host := rt.GetHost()
		defaultHostName := common.LoadFromConfig(common.Config, common.OpConfigDefaultHostname)
		if host == "" && defaultHostName != "" {
			host = obj.GetName() + "-" + obj.GetNamespace() + "." + defaultHostName
		}

		ba.GetStatus().SetReference(common.StatusReferenceRouteHost, host)
		route.Spec.Host = host
		route.Spec.Path = rt.GetPath()
		if ba.GetRoute().GetTermination() != nil {
			if route.Spec.TLS == nil {
				route.Spec.TLS = &routev1.TLSConfig{}
			}
			route.Spec.TLS.Termination = *rt.GetTermination()
			if route.Spec.TLS.Termination == routev1.TLSTerminationReencrypt {
				route.Spec.TLS.Certificate = crt
				route.Spec.TLS.CACertificate = ca
				route.Spec.TLS.Key = key
				route.Spec.TLS.DestinationCACertificate = destCACert
			} else if route.Spec.TLS.Termination == routev1.TLSTerminationPassthrough {
				route.Spec.TLS.Certificate = ""
				route.Spec.TLS.CACertificate = ""
				route.Spec.TLS.Key = ""
				route.Spec.TLS.DestinationCACertificate = ""
			} else if route.Spec.TLS.Termination == routev1.TLSTerminationEdge {
				route.Spec.TLS.Certificate = crt
				route.Spec.TLS.CACertificate = ca
				route.Spec.TLS.Key = key
				route.Spec.TLS.DestinationCACertificate = ""
			}
		}
	}
	if ba.GetRoute() == nil || ba.GetRoute().GetTermination() == nil {
		route.Spec.TLS = nil
	}
	if ba.GetManageTLS() == nil || *ba.GetManageTLS() {
		if route.Spec.TLS == nil {
			route.Spec.TLS = &routev1.TLSConfig{}
			route.Spec.TLS.Termination = routev1.TLSTerminationReencrypt
			route.Spec.TLS.Certificate = crt
			route.Spec.TLS.CACertificate = ca
			route.Spec.TLS.DestinationCACertificate = destCACert
			route.Spec.TLS.Key = key
		}
	}

	// insecureEdgeTerminationPolicy can be set independently of the other TLS config
	// so route.Spec.TLS may be nil at this point.
	if rt := ba.GetRoute(); rt != nil && rt.GetInsecureEdgeTerminationPolicy() != nil {
		if route.Spec.TLS == nil {
			route.Spec.TLS = &routev1.TLSConfig{}
		}
		route.Spec.TLS.InsecureEdgeTerminationPolicy = *rt.GetInsecureEdgeTerminationPolicy()
	}

	if ba.GetRoute() == nil {
		route.Spec.Path = ""
	}
	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = obj.GetName()
	weight := int32(100)
	route.Spec.To.Weight = &weight
	if route.Spec.Port == nil {
		route.Spec.Port = &routev1.RoutePort{}
	}

	if ba.GetService().GetPortName() != "" {
		route.Spec.Port.TargetPort = intstr.FromString(ba.GetService().GetPortName())
	} else {
		route.Spec.Port.TargetPort = intstr.FromString(strconv.Itoa(int(ba.GetService().GetPort())) + "-tcp")
	}
}

// ErrorIsNoMatchesForKind ...
func ErrorIsNoMatchesForKind(err error, kind string, version string) bool {
	return strings.HasPrefix(err.Error(), fmt.Sprintf("no matches for kind \"%s\" in version \"%s\"", kind, version))
}

// CustomizeService ...
func CustomizeService(svc *corev1.Service, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	svc.Labels = ba.GetLabels()
	CustomizeServiceAnnotations(svc)
	svc.Annotations = MergeMaps(svc.Annotations, ba.GetAnnotations())

	if len(svc.Spec.Ports) == 0 {
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{})
	}

	svc.Spec.Ports[0].Port = ba.GetService().GetPort()
	svc.Spec.Ports[0].TargetPort = intstr.FromInt(int(ba.GetService().GetPort()))

	if ba.GetService().GetPortName() != "" {
		svc.Spec.Ports[0].Name = ba.GetService().GetPortName()
	} else {
		svc.Spec.Ports[0].Name = strconv.Itoa(int(ba.GetService().GetPort())) + "-tcp"
	}

	if *ba.GetService().GetType() == corev1.ServiceTypeNodePort && ba.GetService().GetNodePort() != nil {
		svc.Spec.Ports[0].NodePort = *ba.GetService().GetNodePort()
	}

	if *ba.GetService().GetType() == corev1.ServiceTypeClusterIP || strings.HasSuffix(svc.Name, "-headless") == true {
		svc.Spec.Ports[0].NodePort = 0
	}

	svc.Spec.Type = *ba.GetService().GetType()
	svc.Spec.Selector = map[string]string{
		"app.kubernetes.io/instance": obj.GetName(),
	}

	if ba.GetService().GetTargetPort() != nil {
		svc.Spec.Ports[0].TargetPort = intstr.FromInt(int(*ba.GetService().GetTargetPort()))
	}

	numOfAdditionalPorts := len(ba.GetService().GetPorts())
	numOfCurrentPorts := len(svc.Spec.Ports) - 1
	for i := 0; i < numOfAdditionalPorts; i++ {
		for numOfCurrentPorts < numOfAdditionalPorts {
			svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{})
			numOfCurrentPorts++
		}
		for numOfCurrentPorts > numOfAdditionalPorts && len(svc.Spec.Ports) != 0 {
			svc.Spec.Ports = svc.Spec.Ports[:len(svc.Spec.Ports)-1]
			numOfCurrentPorts--
		}
		svc.Spec.Ports[i+1].Port = ba.GetService().GetPorts()[i].Port
		svc.Spec.Ports[i+1].TargetPort = intstr.FromInt(int(ba.GetService().GetPorts()[i].Port))

		if ba.GetService().GetPorts()[i].Name != "" {
			svc.Spec.Ports[i+1].Name = ba.GetService().GetPorts()[i].Name
		} else {
			svc.Spec.Ports[i+1].Name = strconv.Itoa(int(ba.GetService().GetPorts()[i].Port)) + "-tcp"
		}

		if ba.GetService().GetPorts()[i].TargetPort.String() != "" {
			svc.Spec.Ports[i+1].TargetPort = intstr.FromInt(ba.GetService().GetPorts()[i].TargetPort.IntValue())
		}

		if *ba.GetService().GetType() == corev1.ServiceTypeNodePort && ba.GetService().GetPorts()[i].NodePort != 0 {
			svc.Spec.Ports[i+1].NodePort = ba.GetService().GetPorts()[i].NodePort
		}

		if *ba.GetService().GetType() == corev1.ServiceTypeClusterIP {
			svc.Spec.Ports[i+1].NodePort = 0
		}
	}
	if len(ba.GetService().GetPorts()) == 0 {
		for numOfCurrentPorts > 0 {
			svc.Spec.Ports = svc.Spec.Ports[:len(svc.Spec.Ports)-1]
			numOfCurrentPorts--
		}
	}

	// session affinity
	if sessionAffinity := ba.GetService().GetSessionAffinity(); sessionAffinity != nil {
		svc.Spec.SessionAffinity = sessionAffinity.GetType()
		svc.Spec.SessionAffinityConfig = sessionAffinity.GetConfig()
	} else {
		svc.Spec.SessionAffinity = v1.ServiceAffinityNone
		svc.Spec.SessionAffinityConfig = nil
	}
}

func CustomizeServiceAnnotations(svc *corev1.Service) {
	// Enable topology aware hints/routing
	serviceAnnotations := make(map[string]string)
	serviceAnnotations["service.kubernetes.io/topology-aware-hints"] = "Auto" // Topology Aware Hints (< k8s version 1.27)
	serviceAnnotations["service.kubernetes.io/topology-mode"] = "Auto"        // Topology Aware Routing (>= k8s version 1.27)
	svc.Annotations = MergeMaps(svc.Annotations, serviceAnnotations)
}

func CustomizeProbes(container *corev1.Container, ba common.BaseComponent) {
	probesConfig := ba.GetProbes()

	// Probes not defined -- reset all probesConfig to nil
	if probesConfig == nil {
		container.ReadinessProbe = nil
		container.LivenessProbe = nil
		container.StartupProbe = nil
		return
	}

	container.ReadinessProbe = customizeProbe(probesConfig.GetReadinessProbe(), probesConfig.GetDefaultReadinessProbe, ba)
	container.LivenessProbe = customizeProbe(probesConfig.GetLivenessProbe(), probesConfig.GetDefaultLivenessProbe, ba)
	container.StartupProbe = customizeProbe(probesConfig.GetStartupProbe(), probesConfig.GetDefaultStartupProbe, ba)
}

func customizeProbe(config *corev1.Probe, defaultProbeCallback func(ba common.BaseComponent) *corev1.Probe, ba common.BaseComponent) *corev1.Probe {
	// Probe not defined -- set probe to nil
	if config == nil {
		return nil
	}

	// Probe handler is defined in config so use probe as is
	if config.ProbeHandler != (corev1.ProbeHandler{}) {
		return config
	}

	// Probe handler is not defined so use default values for the probe if values not set in probe config
	return customizeProbeDefaults(config, defaultProbeCallback(ba))
}

func customizeProbeDefaults(config *corev1.Probe, defaultProbe *corev1.Probe) *corev1.Probe {
	probe := defaultProbe
	if config.InitialDelaySeconds != 0 {
		probe.InitialDelaySeconds = config.InitialDelaySeconds
	}
	if config.TimeoutSeconds != 0 {
		probe.TimeoutSeconds = config.TimeoutSeconds
	}
	if config.PeriodSeconds != 0 {
		probe.PeriodSeconds = config.PeriodSeconds
	}
	if config.SuccessThreshold != 0 {
		probe.SuccessThreshold = config.SuccessThreshold
	}
	if config.FailureThreshold != 0 {
		probe.FailureThreshold = config.FailureThreshold
	}

	return probe
}

// CustomizeNetworkPolicy configures the network policy.
func CustomizeNetworkPolicy(networkPolicy *networkingv1.NetworkPolicy, isOpenShift bool, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	networkPolicy.Labels = ba.GetLabels()
	networkPolicy.Annotations = MergeMaps(networkPolicy.Annotations, ba.GetAnnotations())

	networkPolicy.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}

	networkPolicy.Spec.PodSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.GetComponentNameLabel(ba): obj.GetName(),
		},
	}

	config := ba.GetNetworkPolicy()
	isExposed := ba.GetExpose() != nil && *ba.GetExpose()
	var rule networkingv1.NetworkPolicyIngressRule

	if config != nil && config.GetNamespaceLabels() != nil && len(config.GetNamespaceLabels()) == 0 &&
		config.GetFromLabels() != nil && len(config.GetFromLabels()) == 0 {
		rule = createAllowAllNetworkPolicyIngressRule()
	} else if isOpenShift {
		rule = createOpenShiftNetworkPolicyIngressRule(ba.GetApplicationName(), networkPolicy.Namespace, isExposed, config)
	} else {
		rule = createKubernetesNetworkPolicyIngressRule(ba.GetApplicationName(), networkPolicy.Namespace, isExposed, config)
	}

	customizeNetworkPolicyPorts(&rule, ba)
	networkPolicy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{rule}
}

func createOpenShiftNetworkPolicyIngressRule(appName string, namespace string, isExposed bool, config common.BaseComponentNetworkPolicy) networkingv1.NetworkPolicyIngressRule {
	rule := networkingv1.NetworkPolicyIngressRule{}

	// Add peer to allow traffic from the OpenShift router
	if isExposed {
		rule.From = append(rule.From,
			networkingv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"policy-group.network.openshift.io/ingress": "",
					},
				},
			},
			// Legacy label still required on OCP 4.6
			networkingv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"network.openshift.io/policy-group": "ingress",
					},
				},
			},
		)
	}

	rule.From = append(rule.From,
		// Add peer to allow traffic from other pods belonging to the app
		createNetworkPolicyPeer(appName, namespace, config),

		// Add peer to allow traffic from OpenShift monitoring
		networkingv1.NetworkPolicyPeer{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"network.openshift.io/policy-group": "monitoring",
				},
			},
		},
	)

	return rule
}

func createKubernetesNetworkPolicyIngressRule(appName string, namespace string, isExposed bool, config common.BaseComponentNetworkPolicy) networkingv1.NetworkPolicyIngressRule {
	if isExposed {
		return createAllowAllNetworkPolicyIngressRule()
	}

	rule := networkingv1.NetworkPolicyIngressRule{}
	rule.From = []networkingv1.NetworkPolicyPeer{
		createNetworkPolicyPeer(appName, namespace, config),
	}
	return rule
}

func createAllowAllNetworkPolicyIngressRule() networkingv1.NetworkPolicyIngressRule {
	return networkingv1.NetworkPolicyIngressRule{
		From: []networkingv1.NetworkPolicyPeer{{
			NamespaceSelector: &metav1.LabelSelector{},
		}},
	}
}

func createNetworkPolicyPeer(appName string, namespace string, networkPolicy common.BaseComponentNetworkPolicy) networkingv1.NetworkPolicyPeer {
	peer := networkingv1.NetworkPolicyPeer{
		NamespaceSelector: &metav1.LabelSelector{},
		PodSelector:       &metav1.LabelSelector{},
	}

	if networkPolicy == nil || networkPolicy.GetNamespaceLabels() == nil {
		peer.NamespaceSelector.MatchLabels = map[string]string{
			"kubernetes.io/metadata.name": namespace,
		}
	} else {
		peer.NamespaceSelector.MatchLabels = networkPolicy.GetNamespaceLabels()
	}

	if networkPolicy == nil || networkPolicy.GetFromLabels() == nil {
		peer.PodSelector.MatchLabels = map[string]string{
			"app.kubernetes.io/part-of": appName,
		}
	} else {
		peer.PodSelector.MatchLabels = networkPolicy.GetFromLabels()
	}

	return peer
}

func customizeNetworkPolicyPorts(ingress *networkingv1.NetworkPolicyIngressRule, ba common.BaseComponent) {
	// Resize the ingress ports
	currentLen := len(ingress.Ports)
	desiredLen := len(ba.GetService().GetPorts()) + 1 // Add one for normal port

	// Shrink if needed
	if currentLen > desiredLen {
		ingress.Ports = ingress.Ports[:desiredLen]
		currentLen = desiredLen
	}

	// Add additional ports if needed
	for currentLen < desiredLen {
		ingress.Ports = append(ingress.Ports, networkingv1.NetworkPolicyPort{})
		currentLen++
	}

	// Initialize the normal port
	primaryPortIntVal := ba.GetService().GetPort()
	if ba.GetService().GetTargetPort() != nil {
		primaryPortIntVal = *ba.GetService().GetTargetPort()
	}
	primaryPort := intstr.IntOrString{Type: intstr.Int, IntVal: primaryPortIntVal}
	ingress.Ports[0].Port = &primaryPort

	// Initialize additional ports
	for i := 0; i < len(ba.GetService().GetPorts()); i++ {
		portPtr := &ba.GetService().GetPorts()[i]
		if portPtr.TargetPort.String() != "" {
			ingress.Ports[i+1].Port = &portPtr.TargetPort
		} else {
			additionalPort := &intstr.IntOrString{Type: intstr.Int, IntVal: portPtr.Port}
			ingress.Ports[i+1].Port = additionalPort
		}
	}
}

// CustomizeAffinity ...
func CustomizeAffinity(affinity *corev1.Affinity, ba common.BaseComponent) {
	affinityConfig := ba.GetAffinity()
	if isCustomAffinityDefined(affinityConfig) {
		customizeAffinity(affinity, ba.GetAffinity())
	} else {
		obj := ba.(metav1.Object)
		customizeDefaultAffinity(affinity, obj.GetName())
	}
	customizeAffinityArchitectures(affinity, affinityConfig)
}

// isCustomAffinityDefined returns true if everything but .spec.affinity.architecture is not defined.
func isCustomAffinityDefined(affinityConfig common.BaseComponentAffinity) bool {
	return affinityConfig != nil &&
		(affinityConfig.GetNodeAffinity() != nil ||
			affinityConfig.GetPodAffinity() != nil ||
			affinityConfig.GetPodAntiAffinity() != nil ||
			affinityConfig.GetNodeAffinityLabels() != nil ||
			len(affinityConfig.GetNodeAffinityLabels()) > 0)
}

func customizeAffinity(affinity *corev1.Affinity, affinityConfig common.BaseComponentAffinity) {
	affinity.NodeAffinity = affinityConfig.GetNodeAffinity()
	affinity.PodAffinity = affinityConfig.GetPodAffinity()
	affinity.PodAntiAffinity = affinityConfig.GetPodAntiAffinity()

	if len(affinityConfig.GetNodeAffinityLabels()) == 0 {
		return
	}

	if affinity.NodeAffinity == nil {
		affinity.NodeAffinity = &corev1.NodeAffinity{}
	}
	if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
	}
	nodeSelector := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution

	if len(nodeSelector.NodeSelectorTerms) == 0 {
		nodeSelector.NodeSelectorTerms = append(nodeSelector.NodeSelectorTerms, corev1.NodeSelectorTerm{})
	}
	labels := affinityConfig.GetNodeAffinityLabels()

	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i := range nodeSelector.NodeSelectorTerms {
		for _, key := range keys {
			values := strings.Split(labels[key], ",")
			for i := range values {
				values[i] = strings.TrimSpace(values[i])
			}

			requirement := corev1.NodeSelectorRequirement{
				Key:      key,
				Operator: corev1.NodeSelectorOpIn,
				Values:   values,
			}

			nodeSelector.NodeSelectorTerms[i].MatchExpressions = append(nodeSelector.NodeSelectorTerms[i].MatchExpressions, requirement)
		}
	}
}

func customizeDefaultAffinity(affinity *corev1.Affinity, name string) {
	if affinity.PodAntiAffinity == nil {
		affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
	}

	term := []corev1.WeightedPodAffinityTerm{
		{
			Weight: 50,
			PodAffinityTerm: corev1.PodAffinityTerm{
				TopologyKey: "topology.kubernetes.io/zone",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/instance": name,
					},
				},
			},
		},
		{
			Weight: 50,
			PodAffinityTerm: corev1.PodAffinityTerm{
				TopologyKey: "kubernetes.io/hostname",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/instance": name,
					},
				},
			},
		},
	}
	affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = term
}

func customizeAffinityArchitectures(affinity *corev1.Affinity, affinityConfig common.BaseComponentAffinity) {
	if affinityConfig == nil {
		return
	}

	archs := affinityConfig.GetArchitecture()

	if len(archs) <= 0 {
		return
	}

	if affinity.NodeAffinity == nil {
		affinity.NodeAffinity = &corev1.NodeAffinity{}
	}
	if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
	}

	nodeSelector := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution

	if len(nodeSelector.NodeSelectorTerms) == 0 {
		nodeSelector.NodeSelectorTerms = append(nodeSelector.NodeSelectorTerms, corev1.NodeSelectorTerm{})
	}

	for i := range nodeSelector.NodeSelectorTerms {
		nodeSelector.NodeSelectorTerms[i].MatchExpressions = append(nodeSelector.NodeSelectorTerms[i].MatchExpressions,
			corev1.NodeSelectorRequirement{
				Key:      "kubernetes.io/arch",
				Operator: corev1.NodeSelectorOpIn,
				Values:   archs,
			},
		)
	}

	for i := range archs {
		term := corev1.PreferredSchedulingTerm{
			Weight: int32(len(archs)) - int32(i),
			Preference: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "kubernetes.io/arch",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{archs[i]},
					},
				},
			},
		}
		affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, term)
	}
}

// CustomizePodSpec ...
func CustomizePodSpec(pts *corev1.PodTemplateSpec, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	pts.Labels = ba.GetLabels()
	pts.Annotations = MergeMaps(pts.Annotations, ba.GetAnnotations())

	// If they exist, add annotations from the StatefulSet or Deployment to the pods
	// Both structs can exist, but if StatefulSet =! nil, then that is 'active' and the
	// deployment should be ignored
	dp := ba.GetDeployment()
	rcss := ba.GetStatefulSet()
	if rcss != nil {
		if rcss.GetAnnotations() != nil {
			pts.Annotations = MergeMaps(pts.Annotations, rcss.GetAnnotations())
		}
	} else {
		if dp != nil && dp.GetAnnotations() != nil {
			pts.Annotations = MergeMaps(pts.Annotations, dp.GetAnnotations())
		}
	}

	var appContainer corev1.Container
	if len(pts.Spec.Containers) == 0 {
		appContainer = corev1.Container{}
	} else {
		appContainer = *GetAppContainer(pts.Spec.Containers)
	}

	appContainer.Name = "app"
	if len(appContainer.Ports) == 0 {
		appContainer.Ports = append(appContainer.Ports, corev1.ContainerPort{})
	}

	if ba.GetService().GetTargetPort() != nil {
		appContainer.Ports[0].ContainerPort = *ba.GetService().GetTargetPort()
	} else {
		appContainer.Ports[0].ContainerPort = ba.GetService().GetPort()
	}

	appContainer.Image = ba.GetStatus().GetImageReference()
	if ba.GetService().GetPortName() != "" {
		appContainer.Ports[0].Name = ba.GetService().GetPortName()
	} else {
		appContainer.Ports[0].Name = strconv.Itoa(int(appContainer.Ports[0].ContainerPort)) + "-tcp"
	}
	if ba.GetResourceConstraints() != nil {
		appContainer.Resources = *ba.GetResourceConstraints()
	}

	CustomizeProbes(&appContainer, ba)

	if ba.GetPullPolicy() != nil {
		appContainer.ImagePullPolicy = *ba.GetPullPolicy()
	}
	appContainer.Env = ba.GetEnv()
	appContainer.EnvFrom = ba.GetEnvFrom()

	pts.Spec.InitContainers = ba.GetInitContainers()

	appContainer.VolumeMounts = ba.GetVolumeMounts()
	pts.Spec.Volumes = ba.GetVolumes()

	appContainer.SecurityContext = GetSecurityContext(ba)

	if ba.GetManageTLS() == nil || *ba.GetManageTLS() || ba.GetService().GetCertificateSecretRef() != nil {

		secretName := ba.GetStatus().GetReferences()[common.StatusReferenceCertSecretName]
		appContainer.Env = append(appContainer.Env, corev1.EnvVar{Name: "TLS_DIR", Value: "/etc/x509/certs"})
		pts.Spec.Volumes = append(pts.Spec.Volumes, corev1.Volume{
			Name: "svc-certificate",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		})
		appContainer.VolumeMounts = append(appContainer.VolumeMounts, corev1.VolumeMount{
			Name:      "svc-certificate",
			MountPath: "/etc/x509/certs",
			ReadOnly:  true,
		})
	}

	// This ensures that the pods are updated if the service account is updated
	saRV := ba.GetStatus().GetReferences()[common.StatusReferenceSAResourceVersion]
	if saRV != "" {
		appContainer.Env = append(appContainer.Env, corev1.EnvVar{Name: "SA_RESOURCE_VERSION", Value: saRV})
	}

	pts.Spec.Containers = append([]corev1.Container{appContainer}, ba.GetSidecarContainers()...)

	if name := GetServiceAccountName(ba); name != "" {
		pts.Spec.ServiceAccountName = name
	} else {
		pts.Spec.ServiceAccountName = obj.GetName()
	}
	pts.Spec.RestartPolicy = corev1.RestartPolicyAlways
	badns := ba.GetDNS()
	if badns != nil && badns.GetPolicy() != nil {
		pts.Spec.DNSPolicy = *badns.GetPolicy()
	} else {
		pts.Spec.DNSPolicy = corev1.DNSClusterFirst
	}
	if badns != nil && badns.GetConfig() != nil {
		pts.Spec.DNSConfig = badns.GetConfig()
	} else {
		pts.Spec.DNSConfig = nil
	}

	pts.Spec.TopologySpreadConstraints = make([]corev1.TopologySpreadConstraint, 0)
	topologySpreadConstraintsConfig := ba.GetTopologySpreadConstraints()
	if topologySpreadConstraintsConfig == nil || topologySpreadConstraintsConfig.GetDisableOperatorDefaults() == nil || !*topologySpreadConstraintsConfig.GetDisableOperatorDefaults() {
		CustomizeTopologySpreadConstraints(pts, map[string]string{"app.kubernetes.io/instance": obj.GetName()})
	}
	if topologySpreadConstraintsConfig != nil && topologySpreadConstraintsConfig.GetConstraints() != nil {
		pts.Spec.TopologySpreadConstraints = MergeTopologySpreadConstraints(pts.Spec.TopologySpreadConstraints, *topologySpreadConstraintsConfig.GetConstraints())
	}

	pts.Spec.Affinity = &corev1.Affinity{}
	CustomizeAffinity(pts.Spec.Affinity, ba)

	mount := true
	basa := ba.GetServiceAccount()
	if basa != nil {
		if basa.GetMountToken() != nil && !*basa.GetMountToken() {
			// not nil and set to false
			mount = false
		}
	}
	pts.Spec.AutomountServiceAccountToken = &mount

	if ba.GetDisableServiceLinks() != nil && *ba.GetDisableServiceLinks() == true {
		//pts.Spec.EnableServiceLinks = ba.GetEnableServiceLinks()
		fv := false
		pts.Spec.EnableServiceLinks = &fv
	} else {
		pts.Spec.EnableServiceLinks = nil
	}

	pts.Spec.Tolerations = ba.GetTolerations()
}

// Initialize an empty TopologySpreadConstraints list and optionally prefers scheduling across zones/hosts for pods with podMatchLabels
func CustomizeTopologySpreadConstraints(pts *corev1.PodTemplateSpec, podMatchLabels map[string]string) {
	if len(podMatchLabels) > 0 {
		pts.Spec.TopologySpreadConstraints = append(pts.Spec.TopologySpreadConstraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       "topology.kubernetes.io/zone",
			WhenUnsatisfiable: corev1.ScheduleAnyway,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: podMatchLabels,
			},
		})
		pts.Spec.TopologySpreadConstraints = append(pts.Spec.TopologySpreadConstraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       "kubernetes.io/hostname",
			WhenUnsatisfiable: corev1.ScheduleAnyway,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: podMatchLabels,
			},
		})
	}
}

// CustomizePersistence ...
func CustomizePersistence(statefulSet *appsv1.StatefulSet, ba common.BaseComponent) {
	obj, ss := ba.(metav1.Object), ba.GetStatefulSet()
	if ss.GetStorage() != nil {
		if len(statefulSet.Spec.VolumeClaimTemplates) == 0 {
			var pvc *corev1.PersistentVolumeClaim
			if ss.GetStorage().GetVolumeClaimTemplate() != nil {
				pvc = ss.GetStorage().GetVolumeClaimTemplate()
			} else {
				pvc = &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc",
						Namespace: obj.GetNamespace(),
						Labels:    ba.GetLabels(),
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(ss.GetStorage().GetSize()),
							},
						},
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
					},
				}
				pvc.Annotations = MergeMaps(pvc.Annotations, ba.GetAnnotations())
				if ss.GetStorage().GetClassName() != "" {
					storageClassName := ss.GetStorage().GetClassName()
					pvc.Spec.StorageClassName = &storageClassName
				}
			}
			statefulSet.Spec.VolumeClaimTemplates = append(statefulSet.Spec.VolumeClaimTemplates, *pvc)
		}

		appContainer := GetAppContainer(statefulSet.Spec.Template.Spec.Containers)

		if ss.GetStorage().GetMountPath() != "" {
			found := false
			for _, v := range appContainer.VolumeMounts {
				if v.Name == statefulSet.Spec.VolumeClaimTemplates[0].Name {
					found = true
				}
			}

			if !found {
				vm := corev1.VolumeMount{
					Name:      statefulSet.Spec.VolumeClaimTemplates[0].Name,
					MountPath: ss.GetStorage().GetMountPath(),
				}
				appContainer.VolumeMounts = append(appContainer.VolumeMounts, vm)
			}
		}
	}
}

// CustomizeServiceAccount ...
func CustomizeServiceAccount(sa *corev1.ServiceAccount, ba common.BaseComponent, client client.Client) error {
	sa.Labels = ba.GetLabels()
	sa.Annotations = MergeMaps(sa.Annotations, ba.GetAnnotations())

	psr := ba.GetStatus().GetReferences()[common.StatusReferencePullSecretName]
	if psr != "" && (ba.GetPullSecret() == nil || *ba.GetPullSecret() != psr) {
		// There is a reference to a pull secret but it doesn't match the one
		// from the CR (which is empty or different)
		// so delete the refered pull secret from the service account
		removePullSecret(sa, psr)
	}

	if ba.GetPullSecret() == nil {
		// There is no pull secret so delete the status reference
		// This has to happen after the reference has been used to remove the pull
		// secret from the service account
		delete(ba.GetStatus().GetReferences(), common.StatusReferencePullSecretName)
	} else {
		// Add the pull secret from the CR to the service account. First check that it is valid
		ps := *ba.GetPullSecret()
		err := client.Get(context.TODO(), types.NamespacedName{Name: ps, Namespace: ba.(metav1.Object).GetNamespace()}, &corev1.Secret{})
		if err != nil {
			return err
		}
		ba.GetStatus().SetReference(common.StatusReferencePullSecretName, ps)
		if len(sa.ImagePullSecrets) == 0 {
			sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{
				Name: ps,
			})
		} else {
			pullSecretName := ps
			found := false
			for _, obj := range sa.ImagePullSecrets {
				if obj.Name == pullSecretName {
					found = true
					break
				}
			}
			if !found {
				sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{
					Name: pullSecretName,
				})
			}
		}
	}

	return nil
}

func removePullSecret(sa *corev1.ServiceAccount, pullSecretName string) {
	index := -1
	for i, obj := range sa.ImagePullSecrets {
		if obj.Name == pullSecretName {
			index = i
			break
		}
	}
	if index != -1 {
		sa.ImagePullSecrets = append(sa.ImagePullSecrets[:index], sa.ImagePullSecrets[index+1:]...)
	}
}

// CustomizeKnativeService ...
func CustomizeKnativeService(ksvc *servingv1.Service, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	ksvc.Labels = ba.GetLabels()
	ksvc.Annotations = filterMerge("autoscaling.knative.dev/", ksvc.Annotations, ba.GetAnnotations())

	// If `expose` is not set to `true`, make Knative route a private route by adding `serving.knative.dev/visibility: cluster-local`
	// to the Knative service. If `serving.knative.dev/visibility: XYZ` is defined in cr.Labels, `expose` always wins.
	if ba.GetExpose() != nil && *ba.GetExpose() {
		delete(ksvc.Labels, "networking.knative.dev/visibility")
		delete(ksvc.Labels, "serving.knative.dev/visibility")
	} else {
		ksvc.Labels["networking.knative.dev/visibility"] = "cluster-local"
		ksvc.Labels["serving.knative.dev/visibility"] = "cluster-local"
	}

	if len(ksvc.Spec.Template.Spec.Containers) == 0 {
		ksvc.Spec.Template.Spec.Containers = append(ksvc.Spec.Template.Spec.Containers, corev1.Container{})
	}

	if len(ksvc.Spec.Template.Spec.Containers[0].Ports) == 0 {
		ksvc.Spec.Template.Spec.Containers[0].Ports = append(ksvc.Spec.Template.Spec.Containers[0].Ports, corev1.ContainerPort{})
	}
	ksvc.Spec.Template.ObjectMeta.Labels = ba.GetLabels()
	ksvc.Spec.Template.ObjectMeta.Annotations = MergeMaps(ksvc.Spec.Template.ObjectMeta.Annotations, ba.GetAnnotations())

	if ba.GetService().GetTargetPort() != nil {
		ksvc.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = *ba.GetService().GetTargetPort()
	} else {
		ksvc.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = ba.GetService().GetPort()
	}

	if ba.GetService().GetPortName() != "" {
		ksvc.Spec.Template.Spec.Containers[0].Ports[0].Name = ba.GetService().GetPortName()
	}

	ksvc.Spec.Template.Spec.Containers[0].Image = ba.GetStatus().GetImageReference()
	// Knative sets its own resource constraints
	// ksvc.Spec.Template.Spec.Containers[0].Resources = *cr.Spec.ResourceConstraints

	CustomizeProbes(&ksvc.Spec.Template.Spec.Containers[0], ba)

	ksvc.Spec.Template.Spec.Containers[0].ImagePullPolicy = *ba.GetPullPolicy()
	ksvc.Spec.Template.Spec.Containers[0].Env = ba.GetEnv()
	ksvc.Spec.Template.Spec.Containers[0].EnvFrom = ba.GetEnvFrom()

	ksvc.Spec.Template.Spec.Containers[0].SecurityContext = GetSecurityContext(ba)

	ksvc.Spec.Template.Spec.Containers[0].VolumeMounts = ba.GetVolumeMounts()
	ksvc.Spec.Template.Spec.Volumes = ba.GetVolumes()

	if name := GetServiceAccountName(ba); name != "" {
		ksvc.Spec.Template.Spec.ServiceAccountName = name
	} else {
		ksvc.Spec.Template.Spec.ServiceAccountName = obj.GetName()
	}

	if ksvc.Spec.Template.Spec.Containers[0].LivenessProbe != nil {
		if ksvc.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet != nil {
			ksvc.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Port = intstr.IntOrString{}
		}
		if ksvc.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket != nil {
			ksvc.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket.Port = intstr.IntOrString{}
		}
	}

	if ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe != nil {
		if ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet != nil {
			ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.IntOrString{}
		}
		if ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe.TCPSocket != nil {
			ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe.TCPSocket.Port = intstr.IntOrString{}
		}
	}

	if ksvc.Spec.Template.Spec.Containers[0].StartupProbe != nil {
		if ksvc.Spec.Template.Spec.Containers[0].StartupProbe.HTTPGet != nil {
			ksvc.Spec.Template.Spec.Containers[0].StartupProbe.HTTPGet.Port = intstr.IntOrString{}
		}
		if ksvc.Spec.Template.Spec.Containers[0].StartupProbe.TCPSocket != nil {
			ksvc.Spec.Template.Spec.Containers[0].StartupProbe.TCPSocket.Port = intstr.IntOrString{}
		}
	}

	basa := ba.GetServiceAccount()
	if basa != nil && basa.GetMountToken() != nil && !*basa.GetMountToken() {
		// not nil and set to false
		mount := false
		ksvc.Spec.Template.Spec.AutomountServiceAccountToken = &mount
	} else {
		ksvc.Spec.Template.Spec.AutomountServiceAccountToken = nil
	}
}

// CustomizeHPA ...
func CustomizeHPA(hpa *autoscalingv1.HorizontalPodAutoscaler, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	hpa.Labels = ba.GetLabels()
	hpa.Annotations = MergeMaps(hpa.Annotations, ba.GetAnnotations())

	hpa.Spec.MaxReplicas = ba.GetAutoscaling().GetMaxReplicas()
	hpa.Spec.MinReplicas = ba.GetAutoscaling().GetMinReplicas()
	hpa.Spec.TargetCPUUtilizationPercentage = ba.GetAutoscaling().GetTargetCPUUtilizationPercentage()

	hpa.Spec.ScaleTargetRef.Name = obj.GetName()
	hpa.Spec.ScaleTargetRef.APIVersion = "apps/v1"

	if ba.GetStatefulSet() != nil {
		hpa.Spec.ScaleTargetRef.Kind = "StatefulSet"
	} else {
		hpa.Spec.ScaleTargetRef.Kind = "Deployment"
	}
}

// Validate if the BaseComponent is valid
func Validate(ba common.BaseComponent) (bool, error) {
	// Storage validation
	ss := ba.GetStatefulSet()
	if ss != nil && ss.GetStorage() != nil {
		if ss.GetStorage().GetVolumeClaimTemplate() == nil {
			if ss.GetStorage().GetSize() == "" {
				return false, fmt.Errorf("validation failed: %s", requiredFieldMessage("spec.statefulSet.storage.size"))
			}
			if _, err := resource.ParseQuantity(ss.GetStorage().GetSize()); err != nil {
				return false, fmt.Errorf("validation failed: cannot parse '%v': %v", ss.GetStorage().GetSize(), err)
			}
		}
	}

	return true, nil
}

func Contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func appendNameIfUnique(names []string, name string) []string {
	if len(name) > 0 && !Contains(names, name) {
		return append(names, name)
	}
	return names
}

// Returns true if the ConfigMap is not specified as optional, otherwise return false.
func configMapShouldExist(configMapKeySelector corev1.ConfigMapKeySelector) bool {
	return configMapKeySelector.Optional == nil || !*configMapKeySelector.Optional
}

// Returns true if the Secret is not specified as optional, otherwise return false.
func secretShouldExist(secretKeySelector corev1.SecretKeySelector) bool {
	return secretKeySelector.Optional == nil || !*secretKeySelector.Optional
}

// Returns an error if any user specified non-optional Secret or ConfigMap for Prometheus monitoring does not exist, otherwise return nil.
func ValidatePrometheusMonitoringEndpoints(ba common.BaseComponent, client client.Client, namespace string) error {
	var basicAuth *prometheusv1.BasicAuth
	var oauth2 *prometheusv1.OAuth2
	var bearerTokenSecret corev1.SecretKeySelector
	var authorization *prometheusv1.SafeAuthorization
	monitoring := ba.GetMonitoring()
	if monitoring != nil {
		for _, endpoint := range monitoring.GetEndpoints() {
			var secretNames []string
			var configMapNames []string
			// BasicAuth
			basicAuth = endpoint.BasicAuth
			if basicAuth != nil {
				if secretShouldExist(basicAuth.Username) {
					secretNames = appendNameIfUnique(secretNames, basicAuth.Username.Name)
				}
				if secretShouldExist(basicAuth.Password) {
					secretNames = appendNameIfUnique(secretNames, basicAuth.Password.Name)
				}
			}
			// OAuth2
			oauth2 = endpoint.OAuth2
			if oauth2 != nil {
				if oauth2.ClientID.Secret != nil && secretShouldExist(*oauth2.ClientID.Secret) {
					secretNames = appendNameIfUnique(secretNames, oauth2.ClientID.Secret.Name)
				}
				if oauth2.ClientID.ConfigMap != nil && configMapShouldExist(*oauth2.ClientID.ConfigMap) {
					configMapNames = appendNameIfUnique(configMapNames, oauth2.ClientID.ConfigMap.Name)
				}
				if secretShouldExist(oauth2.ClientSecret) {
					secretNames = appendNameIfUnique(secretNames, oauth2.ClientSecret.Name)
				}
			}
			// BearerTokenSecret
			if endpoint.BearerTokenSecret != nil {
				bearerTokenSecret = *endpoint.BearerTokenSecret
			}
			if secretShouldExist(bearerTokenSecret) {
				secretNames = appendNameIfUnique(secretNames, bearerTokenSecret.Name)
			}
			// Authorization
			authorization = endpoint.Authorization
			if authorization != nil && authorization.Credentials != nil && secretShouldExist(*authorization.Credentials) {
				secretNames = appendNameIfUnique(secretNames, authorization.Credentials.Name)
			}
			// Error if any Secret is specified but does not exist
			for _, secretName := range secretNames {
				if err := client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: namespace}, &corev1.Secret{}); err != nil {
					errorMessage := fmt.Sprintf("Could not find Secret '%s' in this namespace.", secretName)
					return errors.New(errorMessage)
				}
			}
			// Error if any ConfigMap is specified but does not exist
			for _, configMapName := range configMapNames {
				if err := client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: namespace}, &corev1.ConfigMap{}); err != nil {
					errorMessage := fmt.Sprintf("Could not find ConfigMap '%s' in this namespace.", configMapName)
					return errors.New(errorMessage)
				}
			}
		}
	}
	return nil
}

func createValidationError(msg string) error {
	return fmt.Errorf("validation failed: %s", msg)
}

func requiredFieldMessage(fieldPaths ...string) string {
	return "must set the field(s): " + strings.Join(fieldPaths, ", ")
}

// CustomizeServiceMonitor ...
func CustomizeServiceMonitor(sm *prometheusv1.ServiceMonitor, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	sm.Labels = ba.GetLabels()
	sm.Annotations = MergeMaps(sm.Annotations, ba.GetAnnotations())

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/instance":                obj.GetName(),
			"monitor." + ba.GetGroupName() + "/enabled": "true",
		},
	}
	if len(sm.Spec.Endpoints) == 0 {
		sm.Spec.Endpoints = append(sm.Spec.Endpoints, prometheusv1.Endpoint{})
	}
	sm.Spec.Endpoints[0].Port = ""
	sm.Spec.Endpoints[0].TargetPort = nil
	sm.Spec.Endpoints[0].TLSConfig = nil
	sm.Spec.Endpoints[0].Scheme = ""

	if len(ba.GetMonitoring().GetEndpoints()) > 0 {
		port := ba.GetMonitoring().GetEndpoints()[0].Port
		targetPort := ba.GetMonitoring().GetEndpoints()[0].TargetPort
		if port != "" {
			sm.Spec.Endpoints[0].Port = port
		}
		if targetPort != nil {
			sm.Spec.Endpoints[0].TargetPort = targetPort
		}
		if port != "" && targetPort != nil {
			sm.Spec.Endpoints[0].TargetPort = nil
		}
	}
	if sm.Spec.Endpoints[0].Port == "" && sm.Spec.Endpoints[0].TargetPort == nil {
		if ba.GetService().GetPortName() != "" {
			sm.Spec.Endpoints[0].Port = ba.GetService().GetPortName()
		} else {
			sm.Spec.Endpoints[0].Port = strconv.Itoa(int(ba.GetService().GetPort())) + "-tcp"
		}
	}
	if len(ba.GetMonitoring().GetLabels()) > 0 {
		for k, v := range ba.GetMonitoring().GetLabels() {
			sm.Labels[k] = v
		}
	}

	if len(ba.GetMonitoring().GetEndpoints()) > 0 {
		endpoints := ba.GetMonitoring().GetEndpoints()
		if endpoints[0].Scheme != "" {
			sm.Spec.Endpoints[0].Scheme = endpoints[0].Scheme
		}
		if endpoints[0].Interval != "" {
			sm.Spec.Endpoints[0].Interval = endpoints[0].Interval
		}
		if endpoints[0].Path != "" {
			sm.Spec.Endpoints[0].Path = endpoints[0].Path
		}

		if endpoints[0].TLSConfig != nil {
			sm.Spec.Endpoints[0].TLSConfig = endpoints[0].TLSConfig
		}

		if endpoints[0].BasicAuth != nil {
			sm.Spec.Endpoints[0].BasicAuth = endpoints[0].BasicAuth
		}

		if endpoints[0].Params != nil {
			sm.Spec.Endpoints[0].Params = endpoints[0].Params
		}
		if endpoints[0].ScrapeTimeout != "" {
			sm.Spec.Endpoints[0].ScrapeTimeout = endpoints[0].ScrapeTimeout
		}
		if endpoints[0].BearerTokenFile != "" {
			sm.Spec.Endpoints[0].BearerTokenFile = endpoints[0].BearerTokenFile
		}
		sm.Spec.Endpoints[0].BearerTokenSecret = endpoints[0].BearerTokenSecret
		sm.Spec.Endpoints[0].ProxyURL = endpoints[0].ProxyURL
		sm.Spec.Endpoints[0].RelabelConfigs = endpoints[0].RelabelConfigs
		sm.Spec.Endpoints[0].MetricRelabelConfigs = endpoints[0].MetricRelabelConfigs
		sm.Spec.Endpoints[0].HonorTimestamps = endpoints[0].HonorTimestamps
		sm.Spec.Endpoints[0].HonorLabels = endpoints[0].HonorLabels
	}
	if ba.GetManageTLS() == nil || *ba.GetManageTLS() {
		if len(ba.GetMonitoring().GetEndpoints()) == 0 || ba.GetMonitoring().GetEndpoints()[0].TLSConfig == nil {
			sm.Spec.Endpoints[0].Scheme = "https"
			if sm.Spec.Endpoints[0].TLSConfig == nil {
				sm.Spec.Endpoints[0].TLSConfig = &prometheusv1.TLSConfig{}
			}
			sm.Spec.Endpoints[0].TLSConfig.CA = prometheusv1.SecretOrConfigMap{}
			sm.Spec.Endpoints[0].TLSConfig.CA.Secret = &corev1.SecretKeySelector{}
			sm.Spec.Endpoints[0].TLSConfig.CA.Secret.Name = ba.GetStatus().GetReferences()[common.StatusReferenceCertSecretName]
			sm.Spec.Endpoints[0].TLSConfig.CA.Secret.Key = "tls.crt"
			serverName := obj.GetName() + "." + obj.GetNamespace() + ".svc"
			sm.Spec.Endpoints[0].TLSConfig.ServerName = &serverName
		}

	}

}

// GetWatchNamespaces returns a slice of namespaces the operator should watch based on WATCH_NAMESPSCE value
// WATCH_NAMESPSCE value could be empty for watching the whole cluster or a comma-separated list of namespaces
func GetWatchNamespaces() ([]string, error) {
	watchNamespace, err := GetWatchNamespace()
	if err != nil {
		return nil, err
	}

	var watchNamespaces []string
	for _, ns := range strings.Split(watchNamespace, ",") {
		watchNamespaces = append(watchNamespaces, strings.TrimSpace(ns))
	}

	return watchNamespaces, nil
}

// MergeTopologySpreadConstraints returns the union of all TopologySpreadConstraint lists. The order of lists passed into the
// func, defines the importance.
func MergeTopologySpreadConstraints(constraints ...[]corev1.TopologySpreadConstraint) []corev1.TopologySpreadConstraint {
	dest := make([]corev1.TopologySpreadConstraint, 0)
	destTopologyKeyMemo := make(map[string]int)

	for i := range constraints {
		for j, constraint := range constraints[i] {
			if t, found := destTopologyKeyMemo[constraint.TopologyKey]; found {
				dest[t] = constraint
			} else {
				dest = append(dest, constraint)
				destTopologyKeyMemo[constraint.TopologyKey] = j
			}
		}
	}
	return dest
}

// MergeMaps returns a map containing the union of al the key-value pairs from the input maps. The order of the maps passed into the
// func, defines the importance. e.g. if (keyA, value1) is in map1, and (keyA, value2) is in map2, mergeMaps(map1, map2) would contain (keyA, value2).
// If the input map is nil, it is treated as empty map.
func MergeMaps(maps ...map[string]string) map[string]string {
	dest := make(map[string]string)

	for i := range maps {
		for key, value := range maps[i] {
			dest[key] = value
		}
	}

	return dest
}

// filterMerge merges maps as per MergeMaps, but will filter out any keys which
// match keyFilterPrefix
func filterMerge(keyFilterPrefix string, maps ...map[string]string) map[string]string {
	dest := make(map[string]string)

	for i := range maps {
		for key, value := range maps[i] {
			if strings.HasPrefix(key, keyFilterPrefix) {
				continue
			}
			dest[key] = value
		}
	}
	return dest
}

// BuildServiceBindingSecretName returns secret name of a consumable service
func BuildServiceBindingSecretName(name, namespace string) string {
	return fmt.Sprintf("%s-%s", namespace, name)
}

// ContainsString returns true if `s` is in the slice. Otherwise, returns false
func ContainsString(slice []string, s string) bool {
	for _, str := range slice {
		if str == s {
			return true
		}
	}
	return false
}

// AppendIfNotSubstring appends `a` to comma-separated list of strings in `s`
func AppendIfNotSubstring(a, s string) string {
	if s == "" {
		return a
	}
	subs := strings.Split(s, ",")
	if !ContainsString(subs, a) {
		subs = append(subs, a)
	}
	return strings.Join(subs, ",")
}

// EnsureOwnerRef adds the ownerref if needed. Removes ownerrefs with conflicting UIDs.
// Returns true if the input is mutated. Copied from "https://github.com/openshift/library-go/blob/release-4.5/pkg/controller/ownerref.go"
func EnsureOwnerRef(metadata metav1.Object, newOwnerRef metav1.OwnerReference) bool {
	foundButNotEqual := false
	for _, existingOwnerRef := range metadata.GetOwnerReferences() {
		if existingOwnerRef.APIVersion == newOwnerRef.APIVersion &&
			existingOwnerRef.Kind == newOwnerRef.Kind &&
			existingOwnerRef.Name == newOwnerRef.Name {

			// if we're completely the same, there's nothing to do
			if equality.Semantic.DeepEqual(existingOwnerRef, newOwnerRef) {
				return false
			}

			foundButNotEqual = true
			break
		}
	}

	// if we weren't found, then we just need to add ourselves
	if !foundButNotEqual {
		metadata.SetOwnerReferences(append(metadata.GetOwnerReferences(), newOwnerRef))
		return true
	}

	// if we need to remove an existing ownerRef, just do the easy thing and build it back from scratch
	newOwnerRefs := []metav1.OwnerReference{newOwnerRef}
	for i := range metadata.GetOwnerReferences() {
		existingOwnerRef := metadata.GetOwnerReferences()[i]
		if existingOwnerRef.APIVersion == newOwnerRef.APIVersion &&
			existingOwnerRef.Kind == newOwnerRef.Kind &&
			existingOwnerRef.Name == newOwnerRef.Name {
			continue
		}
		newOwnerRefs = append(newOwnerRefs, existingOwnerRef)
	}
	metadata.SetOwnerReferences(newOwnerRefs)
	return true
}

func normalizeEnvVariableName(name string) string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToUpper(name))
}

// GetOpenShiftAnnotations returns OpenShift specific annotations
func GetOpenShiftAnnotations(ba common.BaseComponent) map[string]string {
	// Conversion table between the pseudo Open Container Initiative <-> OpenShift annotations
	conversionMap := map[string]string{
		"image.opencontainers.org/source":   "app.openshift.io/vcs-uri",
		"image.opencontainers.org/revision": "app.openshift.io/vcs-ref",
	}

	annos := map[string]string{}
	for from, to := range conversionMap {
		if annoVal, ok := ba.GetAnnotations()[from]; ok {
			annos[to] = annoVal
		}
	}

	return annos
}

// IsClusterWide returns true if watchNamespaces is set to [""]
func IsClusterWide(watchNamespaces []string) bool {
	return len(watchNamespaces) == 1 && watchNamespaces[0] == ""
}

// GetAppContainer returns the container that is running the app
func GetAppContainer(containerList []corev1.Container) *corev1.Container {
	for i := 0; i < len(containerList); i++ {
		if containerList[i].Name == "app" {
			return &containerList[i]
		}
	}
	return &containerList[0]
}

// CustomizeIngress customizes ingress resource
func CustomizeIngress(ing *networkingv1.Ingress, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	ing.Labels = ba.GetLabels()
	servicePort := strconv.Itoa(int(ba.GetService().GetPort())) + "-tcp"
	host := ""
	path := ""
	pathType := networkingv1.PathType("")

	rt := ba.GetRoute()
	if rt != nil {
		host = rt.GetHost()
		path = rt.GetPath()
		pathType = rt.GetPathType()
		ing.Annotations = MergeMaps(ing.Annotations, ba.GetAnnotations(), rt.GetAnnotations())
	} else {
		ing.Annotations = MergeMaps(ing.Annotations, ba.GetAnnotations())
	}

	if ba.GetService().GetPortName() != "" {
		servicePort = ba.GetService().GetPortName()
	}
	defaultHostName := common.LoadFromConfig(common.Config, common.OpConfigDefaultHostname)
	if host == "" && defaultHostName != "" {
		host = obj.GetName() + "-" + obj.GetNamespace() + "." + defaultHostName
	}

	if host == "" {
		l := log.WithValues("Request.Namespace", obj.GetNamespace(), "Request.Name", obj.GetName())
		l.Info("No Ingress hostname is provided. Ingress might not function correctly without hostname. It is recommended to set Ingress host or to provide default value through operator's config map.")
	}

	if pathType == "" {
		pathType = networkingv1.PathTypeImplementationSpecific
	}

	ing.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     path,
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: obj.GetName(),
									Port: networkingv1.ServiceBackendPort{
										Name: servicePort,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	tlsSecretName := ""
	if rt != nil && rt.GetCertificateSecretRef() != nil && *rt.GetCertificateSecretRef() != "" {
		tlsSecretName = *rt.GetCertificateSecretRef()
	}
	if tlsSecretName != "" && host != "" {
		ing.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{host},
				SecretName: tlsSecretName,
			},
		}
	} else {
		ing.Spec.TLS = nil
	}
}

// ExecuteCommandInContainer Execute command inside a container in a pod through API
func ExecuteCommandInContainer(config *rest.Config, podName, podNamespace, containerName string, command []string) (string, error) {

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create Clientset")
		return "", fmt.Errorf("Failed to create Clientset: %v", err.Error())
	}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(podNamespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command:   command,
		Container: containerName,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("Encountered error while creating Executor: %v", err.Error())
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	if err != nil {
		return stderr.String(), fmt.Errorf("Encountered error while running command: %v ; Stderr: %v ; Error: %v", command, stderr.String(), err.Error())
	}

	return stderr.String(), nil
}

// GetWatchNamespace returns the Namespace the operator should be watching for changes
func GetWatchNamespace() (string, error) {
	// WatchNamespaceEnvVar is the constant for env variable WATCH_NAMESPACE
	// which specifies the Namespace to watch.
	// An empty value means the operator is running with cluster scope.
	var watchNamespaceEnvVar = "WATCH_NAMESPACE"

	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	return ns, nil
}

// GetOperatorNamespace returns the Namespace the operator installed in
func GetOperatorNamespace() (string, error) {
	var operatorNamespaceEnvVar = "OPERATOR_NAMESPACE"

	ns, found := os.LookupEnv(operatorNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", operatorNamespaceEnvVar)
	}
	return ns, nil
}

func equals(sl1, sl2 []string) bool {
	if len(sl1) != len(sl2) {
		return false
	}
	for i, v := range sl1 {
		if v != sl2[i] {
			return false
		}
	}
	return true
}

func (r *ReconcilerBase) toJSONFromRaw(content *runtime.RawExtension) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(content.Raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// Looks for a pull secret in the service account retrieved from the component
// Returns nil if there is at least one image pull secret, otherwise an error
func ServiceAccountPullSecretExists(ba common.BaseComponent, client client.Client) error {
	obj := ba.(metav1.Object)
	ns := obj.GetNamespace()
	saName := obj.GetName()
	if name := GetServiceAccountName(ba); name != "" {
		saName = name
	}

	sa := &corev1.ServiceAccount{}
	getErr := client.Get(context.TODO(), types.NamespacedName{Name: saName, Namespace: ns}, sa)
	if getErr != nil {
		return getErr
	}
	secrets := sa.ImagePullSecrets
	if len(secrets) > 0 {
		// if this is our service account there will be one image pull secret
		// For others there could be more. either way, just use the first?
		sName := secrets[0].Name
		err := client.Get(context.TODO(), types.NamespacedName{Name: sName, Namespace: ns}, &corev1.Secret{})
		if err != nil {
			saErr := errors.New("Service account " + saName + " isn't ready. Reason: " + err.Error())
			return saErr
		}
	}

	// Set a reference in the CR to the service account version. This is done here as
	// the service account has been retrieved whether it is ours or a user provided one
	ba.GetStatus().SetReference(common.StatusReferenceSAResourceVersion, sa.ResourceVersion)

	return nil
}

// Get security context from CR and apply customization to default settings
func GetSecurityContext(ba common.BaseComponent) *corev1.SecurityContext {
	baSecurityContext := ba.GetSecurityContext()

	valFalse := false
	valTrue := true

	cap := make([]corev1.Capability, 1)
	cap[0] = "ALL"

	// Set default security context
	secContext := &corev1.SecurityContext{
		AllowPrivilegeEscalation: &valFalse,
		Capabilities: &corev1.Capabilities{
			Drop: cap,
		},
		Privileged:             &valFalse,
		ReadOnlyRootFilesystem: &valFalse,
		RunAsNonRoot:           &valTrue,
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}

	// Customize security context
	if baSecurityContext != nil {
		if baSecurityContext.AllowPrivilegeEscalation == nil {
			baSecurityContext.AllowPrivilegeEscalation = secContext.AllowPrivilegeEscalation
		}
		if baSecurityContext.Capabilities == nil {
			baSecurityContext.Capabilities = secContext.Capabilities
		}
		if baSecurityContext.Privileged == nil {
			baSecurityContext.Privileged = secContext.Privileged
		}
		if baSecurityContext.ReadOnlyRootFilesystem == nil {
			baSecurityContext.ReadOnlyRootFilesystem = secContext.ReadOnlyRootFilesystem
		}
		if baSecurityContext.RunAsNonRoot == nil {
			baSecurityContext.RunAsNonRoot = secContext.RunAsNonRoot
		}
		if baSecurityContext.SeccompProfile == nil {
			baSecurityContext.SeccompProfile = secContext.SeccompProfile
		}
		return baSecurityContext
	}
	return secContext
}

func AddOCPCertAnnotation(ba common.BaseComponent, svc *corev1.Service) {
	bao := ba.(metav1.Object)

	if ba.GetCreateKnativeService() != nil && *ba.GetCreateKnativeService() {
		if val := svc.Annotations["service.beta.openshift.io/serving-cert-secret-name"]; val == bao.GetName()+"-svc-tls-ocp" {
			delete(svc.Annotations, "service.beta.openshift.io/serving-cert-secret-name")
			delete(svc.Annotations, "service.beta.openshift.io/serving-cert-signed-by")
			delete(svc.Annotations, "service.alpha.openshift.io/serving-cert-signed-by")

		}
		return
	}

	if ba.GetManageTLS() != nil && !*ba.GetManageTLS() || ba.GetService() != nil && ba.GetService().GetCertificateSecretRef() != nil {
		if val := svc.Annotations["service.beta.openshift.io/serving-cert-secret-name"]; val == bao.GetName()+"-svc-tls-ocp" {
			delete(svc.Annotations, "service.beta.openshift.io/serving-cert-secret-name")
			delete(svc.Annotations, "service.beta.openshift.io/serving-cert-signed-by")
			delete(svc.Annotations, "service.alpha.openshift.io/serving-cert-signed-by")
		}
		return
	}

	val, ok := svc.Annotations["service.beta.openshift.io/serving-cert-secret-name"]
	if !ok {
		val, ok = svc.Annotations["service.alpha.openshift.io/serving-cert-secret-name"]
		if ok {
			ba.GetStatus().SetReference(common.StatusReferenceCertSecretName, val)
			return
		}
	} else {
		ba.GetStatus().SetReference(common.StatusReferenceCertSecretName, val)
		return
	}

	svc.Annotations["service.beta.openshift.io/serving-cert-secret-name"] = bao.GetName() + "-svc-tls-ocp"
	ba.GetStatus().SetReference(common.StatusReferenceCertSecretName, bao.GetName()+"-svc-tls-ocp")

}

func CustomizePodWithSVCCertificate(pts *corev1.PodTemplateSpec, ba common.BaseComponent, client client.Client) error {

	if ba.GetManageTLS() == nil || *ba.GetManageTLS() || ba.GetService().GetCertificateSecretRef() != nil {
		obj := ba.(metav1.Object)
		secretName := ba.GetStatus().GetReferences()[common.StatusReferenceCertSecretName]
		if secretName != "" {
			return addSecretResourceVersionAsEnvVar(pts, obj, client, secretName, "SERVICE_CERT")
		} else {
			return errors.New("Service certifcate secret name must not be empty")
		}
	}
	return nil
}
func addSecretResourceVersionAsEnvVar(pts *corev1.PodTemplateSpec, object metav1.Object, client client.Client, secretName string, envNamePrefix string) error {
	secret := &corev1.Secret{}
	err := client.Get(context.Background(), types.NamespacedName{Name: secretName, Namespace: object.GetNamespace()}, secret)
	if err != nil {
		return fmt.Errorf("Secret %q was not found in namespace %q, %w", secretName, object.GetNamespace(), err)
	}
	pts.Spec.Containers[0].Env = append(pts.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  envNamePrefix + "_SECRET_RESOURCE_VERSION",
		Value: secret.ResourceVersion})
	return nil
}

// This should only be called once from main.go on operator start
// It checks for the presence of the operators config map and
// creates it if it doesn't exist
// As we load logging config from the config map, we can't use log messages as the logger won't be setup yet.
func CreateConfigMap(mapName string) {
	// The config map may not be in a watched namespace, in which case the normal client won't read it.
	client, clerr := client.New(clientcfg.GetConfigOrDie(), client.Options{})
	if clerr != nil {
		fmt.Fprintf(os.Stderr, "Couldn't create a client for config map creation: %s\n", clerr)
		return
	}

	operatorNs, _ := GetOperatorNamespace()
	if operatorNs == "" {
		// This could happen if the operator is running locally, i.e. outside the cluster
		watchNamespaces, err := GetWatchNamespaces()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting watch namespace: %s\n", err)
			return
		}
		// If the operator is running locally, use the first namespace in the `watchNamespaces`
		operatorNs = watchNamespaces[0]
	}
	configMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: mapName, Namespace: operatorNs}, configMap)
	if err != nil && apierrors.IsNotFound(err) {
		fmt.Fprintf(os.Stderr, "The operator config map was not found. Attempting to create it: %s\n", err)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't retrieve operator config map: %s\n", err)
		return
	} else {
		fmt.Fprintf(os.Stderr, "Existing operator config map was found\n")
		return
	}

	newConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mapName,
			Namespace: operatorNs,
			Labels:    map[string]string{"app.kubernetes.io/instance": mapName, "app.kubernetes.io/managed-by": "user", "app.kubernetes.io/name": mapName},
		}}
	// The config map doesn't exist, so need to initialize the default config data, and then
	// store it in a new map
	common.Config = common.DefaultOpConfig()
	_, cerr := controllerutil.CreateOrUpdate(context.TODO(), client, newConfigMap, func() error {
		newConfigMapData := make(map[string]string)
		common.Config.Range(func(key, value interface{}) bool {
			newConfigMapData[key.(string)] = value.(string)
			return true
		})
		newConfigMap.Data = newConfigMapData
		return nil
	})
	if cerr != nil {
		fmt.Fprintf(os.Stderr, "Couldn't create config map in namespace %s: %s\n", operatorNs, err)
	} else {
		fmt.Fprintf(os.Stderr, "Operator Config map created in namespace %s\n", operatorNs)
	}
}

// UpdateConfigMap ...
func UpdateConfigMap(configMap *corev1.ConfigMap, key string) {
	data := configMap.Data
	data[key] = common.LoadFromConfig(common.Config, key)
}

func GetIssuerResourceVersion(client client.Client, certificate *certmanagerv1.Certificate) (string, error) {
	issuer := &certmanagerv1.Issuer{}
	err := client.Get(context.Background(), types.NamespacedName{Name: certificate.Spec.IssuerRef.Name,
		Namespace: certificate.Namespace}, issuer)
	if err != nil {
		return "", err
	}
	if issuer.Spec.CA != nil {
		caSecret := &corev1.Secret{}
		err = client.Get(context.Background(), types.NamespacedName{Name: issuer.Spec.CA.SecretName,
			Namespace: certificate.Namespace}, caSecret)
		if err != nil {
			return "", err
		} else {
			return issuer.ResourceVersion + "," + caSecret.ResourceVersion, nil
		}
	} else {
		return issuer.ResourceVersion, nil
	}
}

func CheckForNameConflicts(operator string, name string, namespace string, client client.Client, request reconcile.Request, isKnativeSupported bool) error {
	defaultMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
	// Check to see if the knative service is owned by us
	if isKnativeSupported {
		ksvc := &servingv1.Service{ObjectMeta: defaultMeta}
		err := client.Get(context.TODO(), request.NamespacedName, ksvc)
		if err == nil {
			for _, element := range ksvc.OwnerReferences {
				if element.Kind != operator {
					message := "Existing Knative Service \"" + name + "\" is not managed by this operator. It is being managed by \"" + element.Kind + "\". Resolve the naming conflict."
					err = errors.New(message)
					return err
				}
			}
		}
	}
	// Check to see if the existing deployment or statefulsets are owned by us
	deploy := &appsv1.Deployment{ObjectMeta: defaultMeta}
	err := client.Get(context.TODO(), request.NamespacedName, deploy)
	if err == nil {
		for _, element := range deploy.OwnerReferences {
			if element.Kind != operator {
				message := "Existing Deployment \"" + name + "\" is not managed by this operator. It is being managed by \"" + element.Kind + "\". Resolve the naming conflict."
				err = errors.New(message)
				return err
			}
		}
	}
	// Check to see if the existing statefulset is owned by us
	statefulSet := &appsv1.StatefulSet{ObjectMeta: defaultMeta}
	err = client.Get(context.TODO(), request.NamespacedName, statefulSet)
	if err == nil {
		for _, element := range statefulSet.OwnerReferences {
			if element.Kind != operator {
				message := "Existing StatefulSet \"" + name + "\" is not managed by this operator. It is being managed by \"" + element.Kind + "\". Resolve the naming conflict."
				err = errors.New(message)
				return err
			}
		}
	}
	return nil
}

// Get the service account name to use or "" if not specified
// in the CR. .spec.ServiceAccountName is deprecated, so
// .spec.ServiceAccount.Name is prefered and should be used if it
// exists. Otherwise, check and fall back to .spec.ServiceAccountName
func GetServiceAccountName(ba common.BaseComponent) string {
	name := ""
	if basa := ba.GetServiceAccount(); basa != nil {
		// There is a .spec.ServiceAccount struct. Use the name from here if it exists
		// Even if the name doesn't exist, ignore the .spec.ServiceAccountName field?
		if baName := basa.GetName(); baName != nil && *baName != "" {
			name = *baName
		}
	} else if ba.GetServiceAccountName() != nil && *ba.GetServiceAccountName() != "" {
		name = *ba.GetServiceAccountName()
	}

	return name
}

// If a custome hostname was previously set, but is now not set, any previous
// route needs to be deleted, as the host in a route cannot be unset
// and the default generated hostname is difficult to manually recreate
func ShouldDeleteRoute(ba common.BaseComponent) bool {
	rh := ba.GetStatus().GetReferences()[common.StatusReferenceRouteHost]
	if rh != "" {
		// The host was previously set.
		// If the host is now empty, delete the old route
		rt := ba.GetRoute()
		defaultHostName := common.LoadFromConfig(common.Config, common.OpConfigDefaultHostname)
		if rt == nil || (rt.GetHost() == "" && defaultHostName == "") {
			return true
		}
	}
	return false
}

// Returns the configured operator max concurrent reconciles setting
func GetMaxConcurrentReconciles() int {
	envValue := os.Getenv("MAX_CONCURRENT_RECONCILES")
	if intVal := parseEnvAsPositiveInt(envValue); intVal != -1 {
		return intVal
	}
	return 1
}

// Parses env as positive int or returns -1 on failure
func parseEnvAsPositiveInt(envValue string) int {
	if envValue != "" {
		positiveIntVal, err := strconv.ParseInt(envValue, 10, 64)
		if err == nil && positiveIntVal >= 1 {
			return int(positiveIntVal)
		}
	}
	return -1
}

// Returns the env setting for Operator watching HorizontalPodAutoscaler (HPA)
func GetOperatorWatchHPA() bool {
	return parseEnvAsBool(os.Getenv("OPERATOR_WATCH_HPA"))
}

// Returns the env setting for disabling all watches
func GetOperatorDisableWatches() bool {
	return parseEnvAsBool(os.Getenv("OPERATOR_DISABLE_WATCHES"))
}

// Parses env as bool or returns false on failure
func parseEnvAsBool(envValue string) bool {
	if envValue != "" {
		boolVal, err := strconv.ParseBool(envValue)
		if err == nil {
			return boolVal
		}
	}
	return false
}
