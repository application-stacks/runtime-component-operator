package utils

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"

	appstacksv1 "github.com/application-stacks/runtime-component-operator/api/v1"
	"github.com/application-stacks/runtime-component-operator/common"

	routev1 "github.com/openshift/api/route/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	name                     = "my-app"
	namespace                = "runtime"
	appImage                 = "my-image"
	objMeta                  = metav1.ObjectMeta{Name: name, Namespace: namespace}
	labels                   = map[string]string{"key1": "value1"}
	annotations              = map[string]string{"key2": "value2"}
	stack                    = "java-microprofile"
	replicas           int32 = 2
	createKNS                = true
	envFrom                  = []corev1.EnvFromSource{{Prefix: namespace}}
	env                      = []corev1.EnvVar{{Name: namespace}}
	pullPolicy               = corev1.PullAlways
	pullSecret               = "mysecret"
	serviceAccountName       = "service-account"

	// Autoscaling & Resource
	targetCPUPer int32 = 30
	autoscaling        = &appstacksv1.RuntimeComponentAutoScaling{
		TargetCPUUtilizationPercentage: &targetCPUPer,
		MinReplicas:                    &replicas,
		MaxReplicas:                    3,
	}
	resLimits = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU: {},
	}
	resourceContraints = &corev1.ResourceRequirements{Limits: resLimits}

	// Service
	targetPort           int32 = 3333
	additionalTargetPort int32 = 9000
	nodePort             int32 = 3011
	serviceClusterIPType       = corev1.ServiceTypeClusterIP
	serviceNodePortType        = corev1.ServiceTypeNodePort
	svcPortName                = "myservice"
	service                    = &appstacksv1.RuntimeComponentService{Type: &serviceClusterIPType, Port: 8443}
	ports                      = []corev1.ServicePort{{Name: "https", Port: 9080, TargetPort: intstr.FromInt(9000)}, {Port: targetPort}}

	// Deployment
	deploymentAnnos = map[string]string{"depAnno": "depAnno"}
	deployment      = &appstacksv1.RuntimeComponentDeployment{Annotations: deploymentAnnos}

	// StatefulSet
	ssAnnos     = map[string]string{"setAnno": "setAnno"}
	statefulSet = &appstacksv1.RuntimeComponentStatefulSet{Annotations: ssAnnos}

	// Storage & Volume
	volumeCT = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc", Namespace: namespace},
		TypeMeta:   metav1.TypeMeta{Kind: "StatefulSet"}}
	storage     = appstacksv1.RuntimeComponentStorage{Size: "10Mi", MountPath: "/mnt/data", VolumeClaimTemplate: volumeCT}
	arch        = []string{"ppc64le"}
	volume      = corev1.Volume{Name: "runtime-volume"}
	volumeMount = corev1.VolumeMount{Name: volumeCT.Name, MountPath: storage.MountPath}

	// Probe
	readinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet:   &corev1.HTTPGetAction{},
			TCPSocket: &corev1.TCPSocketAction{},
		},
	}
	livenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet:   &corev1.HTTPGetAction{},
			TCPSocket: &corev1.TCPSocketAction{},
		},
	}
	startupProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet:   &corev1.HTTPGetAction{},
			TCPSocket: &corev1.TCPSocketAction{},
		},
	}
	probes = &appstacksv1.RuntimeComponentProbes{
		Readiness: readinessProbe,
		Liveness:  livenessProbe,
		Startup:   startupProbe}

	// Route & Ingress
	expose     = true
	notExposed = false
	key        = "key"
	crt        = "crt"
	ca         = "ca"
	destCACert = "destCACert"

	// Fake client with Secrets
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysecret",
			Namespace: namespace,
		},
		Type: "Opaque",
		Data: map[string][]byte{"key": []byte("value")},
	}
	secret2 = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-new-secret",
			Namespace: namespace,
		},
		Type: "Opaque",
		Data: map[string][]byte{"key": []byte("value")},
	}
	secretObjs = []client.Object{secret, secret2}
	fclSecret  = fakeclient.NewClientBuilder().WithObjects(secretObjs...).Build()

	// Fake client with ServiceAccount
	imagePullSecret = corev1.LocalObjectReference{Name: "mysecret"}
	serviceAccount  = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            serviceAccountName,
			Namespace:       namespace,
			ResourceVersion: "9999",
		},
		ImagePullSecrets: []corev1.LocalObjectReference{imagePullSecret},
	}
	SAObjs = []client.Object{serviceAccount, secret}
	fclSA  = fakeclient.NewClientBuilder().WithObjects(SAObjs...).Build()
)

type Test struct {
	test     string
	expected interface{}
	actual   interface{}
}

func TestCustomizeDeployment(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	// Test Recreate update strategy configuration
	deploymentConfig := &appstacksv1.RuntimeComponentDeployment{
		UpdateStrategy: &appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType},
	}
	var replicas int32 = 1
	spec := appstacksv1.RuntimeComponentSpec{Service: service, Deployment: deploymentConfig, Replicas: &replicas}
	dp, runtime := &appsv1.Deployment{}, createRuntimeComponent(objMeta, spec)
	CustomizeDeployment(dp, runtime)
	updateStrategy1 := dp.Spec.Strategy.Type

	// Test Rolling update strategy (default) configuration
	spec.Deployment = &appstacksv1.RuntimeComponentDeployment{Annotations: deploymentAnnos}
	dp, runtime = &appsv1.Deployment{}, createRuntimeComponent(objMeta, spec)
	CustomizeDeployment(dp, runtime)
	updateStrategy2 := dp.Spec.Strategy.Type

	testCD := []Test{
		{"Deployment replicas", replicas, *dp.Spec.Replicas},
		{"Deployment labels", name, dp.Labels["app.kubernetes.io/instance"]},
		{"Deployment annotations", deploymentAnnos, dp.Annotations},
		{"Deployment recreate update strategy", appsv1.RecreateDeploymentStrategyType, updateStrategy1},
		{"Deployment rolling update strategy", appsv1.RollingUpdateDeploymentStrategyType, updateStrategy2},
	}
	verifyTests(testCD, t)
}
func TestCustomizeStatefulSet(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	// Test OnDelete update strategy configuration
	statefulsetConfig := &appstacksv1.RuntimeComponentStatefulSet{
		UpdateStrategy: &appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.OnDeleteStatefulSetStrategyType},
	}
	var replicas int32 = 1
	spec := appstacksv1.RuntimeComponentSpec{Service: service, StatefulSet: statefulsetConfig, Replicas: &replicas}
	ss, runtime := &appsv1.StatefulSet{}, createRuntimeComponent(objMeta, spec)
	CustomizeStatefulSet(ss, runtime)
	updateStrategy1 := ss.Spec.UpdateStrategy.Type

	// Test rolling update strategy (default)
	spec.StatefulSet = &appstacksv1.RuntimeComponentStatefulSet{Annotations: ssAnnos}
	ss, runtime = &appsv1.StatefulSet{}, createRuntimeComponent(objMeta, spec)
	CustomizeStatefulSet(ss, runtime)
	updateStrategy2 := ss.Spec.UpdateStrategy.Type

	testCS := []Test{
		{"StatefulSet replicas", replicas, *ss.Spec.Replicas},
		{"StatefulSet service name", name + "-headless", ss.Spec.ServiceName},
		{"StatefulSet labels", name, ss.Labels["app.kubernetes.io/instance"]},
		{"StatefulSet annotations", ssAnnos, ss.Annotations},
		{"StatefulSet ondelete update strategy", appsv1.OnDeleteStatefulSetStrategyType, updateStrategy1},
		{"StatefulSet rolling update strategy", appsv1.RollingUpdateStatefulSetStrategyType, updateStrategy2},
	}
	verifyTests(testCS, t)
}

func routeTerminationTypeTestHelper(termination routev1.TLSTerminationType, insecureEdgeTerminationPolicy routev1.InsecureEdgeTerminationPolicyType) *routev1.Route {
	// Set various Termination and InsecureEdgeTerminationPolicy for Route
	spec := appstacksv1.RuntimeComponentSpec{Service: service}
	common.Config[common.OpConfigDefaultHostname] = "defaultHostName"
	routeConfig := &appstacksv1.RuntimeComponentRoute{Termination: &termination, InsecureEdgeTerminationPolicy: &insecureEdgeTerminationPolicy}
	spec.Route = routeConfig
	spec.Service.PortName = svcPortName

	route, runtime := &routev1.Route{}, createRuntimeComponent(objMeta, spec)
	CustomizeRoute(route, runtime, key, crt, ca, destCACert)

	return route
}

func TestCustomizeRoute(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	// Test default/empty Route configurations
	spec := appstacksv1.RuntimeComponentSpec{Service: service}
	route, runtime := &routev1.Route{}, createRuntimeComponent(objMeta, spec)
	CustomizeRoute(route, runtime, "", "", "", "")

	testCR := []Test{
		{"Route labels", name, route.Labels["app.kubernetes.io/instance"]},
		{"Route target kind", "Service", route.Spec.To.Kind},
		{"Route target name", name, route.Spec.To.Name},
		{"Route target weight", int32(100), *route.Spec.To.Weight},
		{"Route service target port", intstr.FromString(strconv.Itoa(int(runtime.Spec.Service.Port)) + "-tcp"), route.Spec.Port.TargetPort},
	}
	verifyTests(testCR, t)

	// Test Route configurations with Reencrypt termination and Allow termination policy
	insecureEdgeTerminationPolicy := routev1.InsecureEdgeTerminationPolicyAllow
	termination := routev1.TLSTerminationReencrypt
	reencryptRoute := routeTerminationTypeTestHelper(termination, insecureEdgeTerminationPolicy)

	// Test Route configurations with Passthrough termination and None termination policy
	termination = routev1.TLSTerminationPassthrough
	insecureEdgeTerminationPolicy = routev1.InsecureEdgeTerminationPolicyNone
	passthroughRoute := routeTerminationTypeTestHelper(termination, insecureEdgeTerminationPolicy)

	// Test Route configurations with Edge termination and Redirect termination policy
	insecureEdgeTerminationPolicy = routev1.InsecureEdgeTerminationPolicyRedirect
	termination = routev1.TLSTerminationEdge
	edgeRoute := routeTerminationTypeTestHelper(termination, insecureEdgeTerminationPolicy)

	testCR = []Test{
		{"Route host", name + "-" + namespace + "." + "defaultHostName", reencryptRoute.Spec.Host},
		{"Route target port", intstr.FromString(svcPortName), reencryptRoute.Spec.Port.TargetPort},
		{"Allow termination policy", routev1.InsecureEdgeTerminationPolicyAllow, reencryptRoute.Spec.TLS.InsecureEdgeTerminationPolicy},
		{"Route encryption termination", routev1.TLSTerminationReencrypt, reencryptRoute.Spec.TLS.Termination},
		{"Route encryption termination cert", crt, reencryptRoute.Spec.TLS.Certificate},
		{"Route encryption termination CAcert", ca, reencryptRoute.Spec.TLS.CACertificate},
		{"Route encryption termination key", key, reencryptRoute.Spec.TLS.Key},
		{"Route encryption termination destCACert", destCACert, reencryptRoute.Spec.TLS.DestinationCACertificate},

		{"None termination policy", routev1.InsecureEdgeTerminationPolicyNone, passthroughRoute.Spec.TLS.InsecureEdgeTerminationPolicy},
		{"Route passthrough termination", routev1.TLSTerminationPassthrough, passthroughRoute.Spec.TLS.Termination},
		{"Route passthrough termination cert", "", passthroughRoute.Spec.TLS.Certificate},
		{"Route passthrough termination CAcert", "", passthroughRoute.Spec.TLS.CACertificate},
		{"Route passthrough termination key", "", passthroughRoute.Spec.TLS.Key},
		{"Route passthrough termination destCACert", "", passthroughRoute.Spec.TLS.DestinationCACertificate},

		{"Redirect termination policy", routev1.InsecureEdgeTerminationPolicyRedirect, edgeRoute.Spec.TLS.InsecureEdgeTerminationPolicy},
		{"Route edge termination", routev1.TLSTerminationEdge, edgeRoute.Spec.TLS.Termination},
		{"Route edge termination cert", crt, edgeRoute.Spec.TLS.Certificate},
		{"Route edge termination CAcert", ca, edgeRoute.Spec.TLS.CACertificate},
		{"Route edge termination key", key, edgeRoute.Spec.TLS.Key},
		{"Route edge termination destCACert", "", edgeRoute.Spec.TLS.DestinationCACertificate},
	}
	verifyTests(testCR, t)
}

func TestErrorIsNoMatchesForKind(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	newError := errors.New("test error")
	errorValue := ErrorIsNoMatchesForKind(newError, "kind", "version")

	testCR := []Test{
		{"Error", false, errorValue},
	}
	verifyTests(testCR, t)
}

func optionalTargetPortFunctionalityTests() []Test {
	// Test Service with target port
	spec := appstacksv1.RuntimeComponentSpec{Service: service}
	spec.Service.TargetPort = &targetPort
	svc, runtime := &corev1.Service{}, createRuntimeComponent(objMeta, spec)

	CustomizeService(svc, runtime)
	testCS := []Test{
		{"Service number of exposed ports", 1, len(svc.Spec.Ports)},
		{"Service first exposed port", runtime.Spec.Service.Port, svc.Spec.Ports[0].Port},
		{"Service first exposed target port", intstr.FromInt(int(*runtime.Spec.Service.TargetPort)), svc.Spec.Ports[0].TargetPort},
		{"Service type", *runtime.Spec.Service.Type, svc.Spec.Type},
		{"Service selector", name, svc.Spec.Selector["app.kubernetes.io/instance"]},
	}
	return testCS
}

func optionalNodePortFunctionalityTests() []Test {
	// Test Service with nodeport
	serviceType := corev1.ServiceTypeNodePort
	service := &appstacksv1.RuntimeComponentService{Type: &serviceType, Port: 8443, NodePort: &nodePort}
	spec := appstacksv1.RuntimeComponentSpec{Service: service}
	svc, runtime := &corev1.Service{}, createRuntimeComponent(objMeta, spec)

	CustomizeService(svc, runtime)
	testCS := []Test{
		{"Service number of exposed ports", 1, len(svc.Spec.Ports)},
		{"Service first exposed port", runtime.Spec.Service.Port, svc.Spec.Ports[0].Port},
		{"Service first exposed target port", intstr.FromInt(int(runtime.Spec.Service.Port)), svc.Spec.Ports[0].TargetPort},
		{"Service type", *runtime.Spec.Service.Type, svc.Spec.Type},
		{"Service selector", name, svc.Spec.Selector["app.kubernetes.io/instance"]},
		{"Service nodePort port", *runtime.Spec.Service.NodePort, svc.Spec.Ports[0].NodePort},
	}
	return testCS
}

func additionalPortFunctionalityTests(t *testing.T) {
	// Test Service with additional ports
	spec := appstacksv1.RuntimeComponentSpec{Service: service}
	svc, runtime := &corev1.Service{}, createRuntimeComponent(objMeta, spec)
	runtime.Spec.Service.Ports = ports
	CustomizeService(svc, runtime)

	testCS := []Test{
		{"Service number of exposed ports", 3, len(svc.Spec.Ports)},
		{"Second exposed port", ports[0].Port, svc.Spec.Ports[1].Port},
		{"Second exposed target port", additionalTargetPort, svc.Spec.Ports[1].TargetPort.IntVal},
		{"Second exposed port name", ports[0].Name, svc.Spec.Ports[1].Name},
		{"Second nodeport", ports[0].NodePort, svc.Spec.Ports[1].NodePort},
		{"Third exposed port", ports[1].Port, svc.Spec.Ports[2].Port},
		{"Third exposed port name", fmt.Sprint(ports[1].Port) + "-tcp", svc.Spec.Ports[2].Name},
		{"Third nodeport", ports[1].NodePort, svc.Spec.Ports[2].NodePort},
	}
	verifyTests(testCS, t)

	// Test Service with additional nodeport
	runtime.Spec.Service.Ports = runtime.Spec.Service.Ports[:len(runtime.Spec.Service.Ports)-1]
	runtime.Spec.Service.Ports[0].NodePort = 3000
	runtime.Spec.Service.Type = &serviceNodePortType
	CustomizeService(svc, runtime)

	testCS = []Test{
		{"Service number of exposed ports", 2, len(svc.Spec.Ports)},
		{"First nodeport", runtime.Spec.Service.Ports[0].NodePort, svc.Spec.Ports[1].NodePort},
		{"Port type", serviceNodePortType, svc.Spec.Type},
	}
	verifyTests(testCS, t)

	// Test Service with no more additional ports
	runtime.Spec.Service.Ports = nil
	runtime.Spec.Service.PortName = svcPortName
	CustomizeService(svc, runtime)

	testCS = []Test{
		{"Service number of exposed ports", 1, len(svc.Spec.Ports)},
		{"Service port name", svcPortName, svc.Spec.Ports[0].Name},
	}
	verifyTests(testCS, t)
}

func TestCustomizeService(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1.RuntimeComponentSpec{Service: service}
	svc, runtime := &corev1.Service{}, createRuntimeComponent(objMeta, spec)

	CustomizeService(svc, runtime)
	testCS := []Test{
		{"Service number of exposed ports", 1, len(svc.Spec.Ports)},
		{"Service first exposed port", runtime.Spec.Service.Port, svc.Spec.Ports[0].Port},
		{"Service first exposed target port", intstr.FromInt(int(runtime.Spec.Service.Port)), svc.Spec.Ports[0].TargetPort},
		{"Service type", *runtime.Spec.Service.Type, svc.Spec.Type},
		{"Service selector", name, svc.Spec.Selector["app.kubernetes.io/instance"]},
	}
	verifyTests(testCS, t)

	// Verify behaviour of optional target port functionality
	verifyTests(optionalTargetPortFunctionalityTests(), t)

	// verify optional nodePort functionality in NodePort service
	verifyTests(optionalNodePortFunctionalityTests(), t)

	// Verify behaviour of additional ports functionality
	additionalPortFunctionalityTests(t)
}

func TestCustomizeProbes(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	// Test nil Probes
	var nilProbe *corev1.Probe
	spec := appstacksv1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Service:          service,
	}
	pts, runtime := &corev1.PodTemplateSpec{}, createRuntimeComponent(objMeta, spec)
	CustomizePodSpec(pts, runtime)
	nilLivenessProbe := pts.Spec.Containers[0].LivenessProbe
	nilReadinessProbe := pts.Spec.Containers[0].ReadinessProbe
	nilStartupProbe := pts.Spec.Containers[0].StartupProbe

	// Test LivenessProbe without probe handler (default probe handler)
	livenessProbeConfig := &corev1.Probe{
		InitialDelaySeconds: 60,
		TimeoutSeconds:      2,
		PeriodSeconds:       10,
		SuccessThreshold:    10,
		FailureThreshold:    3}

	// Test empty ReadinessProbe (default probe config)
	defaultLivenessProbeConfig := common.GetDefaultMicroProfileLivenessProbe(runtime)
	defaultReadinessProbeConfig := common.GetDefaultMicroProfileReadinessProbe(runtime)
	runtime.Spec.Probes = &appstacksv1.RuntimeComponentProbes{
		Readiness: &corev1.Probe{},
		Liveness:  livenessProbeConfig}
	CustomizePodSpec(pts, runtime)

	defaultReadinessProbe := pts.Spec.Containers[0].ReadinessProbe
	configuredLivenessProbe := pts.Spec.Containers[0].LivenessProbe
	livenessProbeConfig.ProbeHandler = defaultLivenessProbeConfig.ProbeHandler
	emptyStartupProbe := pts.Spec.Containers[0].StartupProbe

	testCP := []Test{
		{"Nil LivenessProbe", nilProbe, nilLivenessProbe},
		{"Nil ReadinessProbe", nilProbe, nilReadinessProbe},
		{"Nil StartupProbe", nilProbe, nilStartupProbe},

		{"Configured LivenessProbe", livenessProbeConfig, configuredLivenessProbe},
		{"Default ReadinessProbe", defaultReadinessProbeConfig, defaultReadinessProbe},
		{"Empty StartupProbe", nilProbe, emptyStartupProbe},
	}
	verifyTests(testCP, t)
}

func TestCustomizeNetworkPolicy(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	isOpenshift := true
	npConfig := &appstacksv1.RuntimeComponentNetworkPolicy{}

	spec := appstacksv1.RuntimeComponentSpec{
		Expose:        &expose,
		Service:       service,
		NetworkPolicy: npConfig,
	}
	networkPolicy, runtime := &networkingv1.NetworkPolicy{}, createRuntimeComponent(objMeta, spec)

	// NetworkPolicy for OpenShift when exposed
	CustomizeNetworkPolicy(networkPolicy, isOpenshift, runtime)
	openshiftNP := networkPolicy

	// NetworkPolicy for Non-OpenShift when exposed
	runtime.Spec.Service.Ports = ports
	CustomizeNetworkPolicy(networkPolicy, !isOpenshift, runtime)
	nonOpenshiftNP := networkPolicy

	// NetworkPolicy for Non-OpenShift when not exposed
	runtime.Spec.NetworkPolicy = &appstacksv1.RuntimeComponentNetworkPolicy{
		NamespaceLabels: &map[string]string{"namespace": "test"},
		FromLabels:      &map[string]string{"foo": "bar"},
	}
	runtime.Spec.Service.Ports = nil
	runtime.Spec.Expose = &notExposed
	CustomizeNetworkPolicy(networkPolicy, !isOpenshift, runtime)
	notExposedNP := networkPolicy

	runtime.Spec.NetworkPolicy = &appstacksv1.RuntimeComponentNetworkPolicy{
		NamespaceLabels: &map[string]string{},
		FromLabels:      &map[string]string{},
	}

	CustomizeNetworkPolicy(networkPolicy, isOpenshift, runtime)
	allowAllNP := networkPolicy

	testCNP := []Test{
		{"OpenShift NetworkPolicy", name, openshiftNP.Labels["app.kubernetes.io/instance"]},
		{"Non OpenShift NetworkPolicy", "runtime-component-operator", nonOpenshiftNP.Labels["app.kubernetes.io/managed-by"]},
		{"Non OpenShift - not exposed NetworkPolicy", "runtime-component-operator", notExposedNP.Labels["app.kubernetes.io/managed-by"]},
		{"Allow All NetworkPolicy", &metav1.LabelSelector{}, allowAllNP.Spec.Ingress[0].From[0].NamespaceSelector},
	}
	verifyTests(testCNP, t)
}

// Partial test for unittest TestCustomizeAffinity bewlow
func partialTestCustomizeNodeAffinity(t *testing.T) {
	// required during scheduling ignored during execution
	rDSIDE := corev1.NodeSelector{
		NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "key",
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{"large", "medium"},
					},
				},
			},
		},
	}
	// preferred during scheduling ignored during execution
	pDSIDE := []corev1.PreferredSchedulingTerm{
		{
			Weight: int32(20),
			Preference: corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "failure-domain.beta.kubernetes.io/zone",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"zoneB"},
					},
				},
			},
		},
	}
	labels := map[string]string{
		"customNodeLabel": "label1, label2",
	}
	affinityConfig := appstacksv1.RuntimeComponentAffinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution:  &rDSIDE,
			PreferredDuringSchedulingIgnoredDuringExecution: pDSIDE,
		},
		NodeAffinityLabels: labels,
	}
	spec := appstacksv1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Affinity:         &affinityConfig,
	}
	affinity, runtime := &corev1.Affinity{}, createRuntimeComponent(objMeta, spec)
	CustomizeAffinity(affinity, runtime)

	expectedMatchExpressions := []corev1.NodeSelectorRequirement{
		rDSIDE.NodeSelectorTerms[0].MatchExpressions[0],
		{
			Key:      "customNodeLabel",
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{"label1", "label2"},
		},
	}
	testCNA := []Test{
		{"Node Affinity - Required Match Expressions", expectedMatchExpressions,
			affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions},
		{"Node Affinity - Prefered Match Expressions",
			pDSIDE[0].Preference.MatchExpressions,
			affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions},
	}
	verifyTests(testCNA, t)

	// Test nil Affinity configuration
	runtime.Spec.Affinity = &appstacksv1.RuntimeComponentAffinity{
		NodeAffinityLabels: labels,
	}
	CustomizeAffinity(affinity, runtime)

	testCNA = []Test{
		{"Nil Node Affinity", expectedMatchExpressions[1], affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0]},
	}
	verifyTests(testCNA, t)
}

// Partial test for unittest TestCustomizeAffinity bewlow
func partialTestCustomizePodAffinity(t *testing.T) {
	selectorA := makeInLabelSelector("service", []string{"Service-A"})
	selectorB := makeInLabelSelector("service", []string{"Service-B"})
	// required during scheduling ignored during execution
	rDSIDE := []corev1.PodAffinityTerm{
		{LabelSelector: &selectorA, TopologyKey: "failure-domain.beta.kubernetes.io/zone"},
	}
	// preferred during scheduling ignored during execution
	pDSIDE := []corev1.WeightedPodAffinityTerm{
		{
			Weight: int32(20),
			PodAffinityTerm: corev1.PodAffinityTerm{
				LabelSelector: &selectorB, TopologyKey: "kubernetes.io/hostname",
			},
		},
	}
	affinityConfig := appstacksv1.RuntimeComponentAffinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: rDSIDE,
		},
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: pDSIDE,
		},
	}
	spec := appstacksv1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Affinity:         &affinityConfig,
	}
	affinity, runtime := &corev1.Affinity{}, createRuntimeComponent(objMeta, spec)
	CustomizeAffinity(affinity, runtime)

	testCPA := []Test{
		{"Pod Affinity - Required Affinity Term", rDSIDE,
			affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution},
		{"Pod AntiAffinity - Preferred Affinity Term", pDSIDE,
			affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution},
	}
	verifyTests(testCPA, t)
}

func TestCustomizeAffinity(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)
	partialTestCustomizeNodeAffinity(t)
	partialTestCustomizePodAffinity(t)
}

func TestCustomizePodSpecAnnotations(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Service:          service,
		Resources:        resourceContraints,
		Probes:           probes,
		VolumeMounts:     []corev1.VolumeMount{volumeMount},
		PullPolicy:       &pullPolicy,
		Env:              env,
		EnvFrom:          envFrom,
		Volumes:          []corev1.Volume{volume},
	}

	// No dep or set, annotation should be empty
	pts1, runtime1 := &corev1.PodTemplateSpec{}, createRuntimeComponent(objMeta, spec)
	CustomizePodSpec(pts1, runtime1)
	annolen1 := len(pts1.Annotations)
	testAnnotations1 := []Test{
		{"Shouldn't be any annotations", 0, annolen1},
	}
	verifyTests(testAnnotations1, t)

	// dep but not set, annotation should be dep annotations
	spec.Deployment = deployment
	pts2, runtime2 := &corev1.PodTemplateSpec{}, createRuntimeComponent(objMeta, spec)
	CustomizePodSpec(pts2, runtime2)
	annolen2 := len(pts2.Annotations)
	anno2 := pts2.Annotations["depAnno"]
	testAnnotations2 := []Test{
		{"Wrong annotations", "depAnno", anno2},
		{"Wrong number of annotations", 1, annolen2},
	}
	verifyTests(testAnnotations2, t)

	// set but not dep, annotation should be set annotations
	spec.Deployment = nil
	spec.StatefulSet = statefulSet
	pts3, runtime3 := &corev1.PodTemplateSpec{}, createRuntimeComponent(objMeta, spec)
	CustomizePodSpec(pts3, runtime3)
	annolen3 := len(pts3.Annotations)
	anno3 := pts3.Annotations["setAnno"]
	testAnnotations3 := []Test{
		{"Wrong annotations", "setAnno", anno3},
		{"Wrong number of annotations", 1, annolen3},
	}
	verifyTests(testAnnotations3, t)

	// dep and set, annotation should be set annotations
	spec.Deployment = deployment
	pts4, runtime4 := &corev1.PodTemplateSpec{}, createRuntimeComponent(objMeta, spec)
	CustomizePodSpec(pts4, runtime4)
	annolen4 := len(pts4.Annotations)
	anno4 := pts4.Annotations["setAnno"]
	testAnnotations4 := []Test{
		{"Wrong annotations", "setAnno", anno4},
		{"Wrong number of annotations", 1, annolen4},
	}
	verifyTests(testAnnotations4, t)

}

func TestCustomizePodSpec(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Service:          service,
		Resources:        resourceContraints,
		Probes:           probes,
		VolumeMounts:     []corev1.VolumeMount{volumeMount},
		PullPolicy:       &pullPolicy,
		Env:              env,
		EnvFrom:          envFrom,
		Volumes:          []corev1.Volume{volume},
	}
	pts, runtime := &corev1.PodTemplateSpec{}, createRuntimeComponent(objMeta, spec)
	// else cond
	CustomizePodSpec(pts, runtime)
	noCont := len(pts.Spec.Containers)
	noPorts := len(pts.Spec.Containers[0].Ports)
	ptsSAN := pts.Spec.ServiceAccountName
	affinityConfig := appstacksv1.RuntimeComponentAffinity{
		Architecture: arch,
	}
	// if cond
	spec = appstacksv1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Service: &appstacksv1.RuntimeComponentService{
			Type:       &serviceClusterIPType,
			Port:       8443,
			TargetPort: &targetPort,
			PortName:   svcPortName,
		},
		Resources:          resourceContraints,
		Probes:             probes,
		VolumeMounts:       []corev1.VolumeMount{volumeMount},
		PullPolicy:         &pullPolicy,
		PullSecret:         &pullSecret,
		Env:                env,
		EnvFrom:            envFrom,
		Volumes:            []corev1.Volume{volume},
		ServiceAccountName: &serviceAccountName,
		Affinity:           &affinityConfig,
		SecurityContext:    &corev1.SecurityContext{},
	}
	runtime = createRuntimeComponent(objMeta, spec)
	testServiceAccountPullSecretExists(t, runtime)
	CustomizePodSpec(pts, runtime)
	ptsCSAN := pts.Spec.ServiceAccountName

	// Affinity tests
	affArchs := pts.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
	weight := pts.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight
	prefAffArchs := pts.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Values[0]
	assignedTPort := pts.Spec.Containers[0].Ports[0].ContainerPort
	portName := pts.Spec.Containers[0].Ports[0].Name

	testCPS := []Test{
		{"No containers", 1, noCont},
		{"No port", 1, noPorts},
		{"No ServiceAccountName", name, ptsSAN},
		{"ServiceAccountName available", serviceAccountName, ptsCSAN},
		{"Service port name", svcPortName, portName},
	}
	verifyTests(testCPS, t)

	testCA := []Test{
		{"Archs", arch[0], affArchs},
		{"Weight", int32(1), int32(weight)},
		{"Archs", arch[0], prefAffArchs},
		{"No target port", targetPort, assignedTPort},
	}
	verifyTests(testCA, t)
}

func testServiceAccountPullSecretExists(t *testing.T, runtime *appstacksv1.RuntimeComponent) {
	ServiceAccountPullSecretExists(runtime, fclSA)

	testCSA := []Test{
		{"ServiceAccount Resource Version", serviceAccount.ResourceVersion, runtime.Status.References[common.StatusReferenceSAResourceVersion]},
	}
	verifyTests(testCSA, t)
}

func TestCustomizePersistence(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	runtimeStatefulSet := &appstacksv1.RuntimeComponentStatefulSet{Storage: &storage}
	spec := appstacksv1.RuntimeComponentSpec{StatefulSet: runtimeStatefulSet}
	statefulSet, runtime := &appsv1.StatefulSet{}, createRuntimeComponent(objMeta, spec)
	statefulSet.Spec.Template.Spec.Containers = []corev1.Container{{}}
	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{}
	// if vct == 0, runtimeVCT != nil, not found
	CustomizePersistence(statefulSet, runtime)
	ssK := statefulSet.Spec.VolumeClaimTemplates[0].TypeMeta.Kind
	ssMountPath := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath

	//reset
	storageNilVCT := appstacksv1.RuntimeComponentStorage{Size: "10Mi", MountPath: "/mnt/data", ClassName: "storageClassName", VolumeClaimTemplate: nil}
	runtimeStatefulSet = &appstacksv1.RuntimeComponentStatefulSet{Storage: &storageNilVCT}
	spec = appstacksv1.RuntimeComponentSpec{StatefulSet: runtimeStatefulSet}
	statefulSet, runtime = &appsv1.StatefulSet{}, createRuntimeComponent(objMeta, spec)

	statefulSet.Spec.Template.Spec.Containers = []corev1.Container{{}}
	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = append(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMount)
	//runtimeVCT == nil, found
	CustomizePersistence(statefulSet, runtime)
	ssVolumeMountName := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name
	size := statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
	testCP := []Test{
		{"Persistence kind with VCT", volumeCT.TypeMeta.Kind, ssK},
		{"PVC size", storage.Size, size.String()},
		{"Mount path", storage.MountPath, ssMountPath},
		{"Volume Mount Name", volumeCT.Name, ssVolumeMountName},
		{"Storage Class Name", "storageClassName", *statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName},
	}
	verifyTests(testCP, t)
}

func TestCustomizeServiceAccount(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1.RuntimeComponentSpec{PullSecret: &pullSecret}
	sa, runtime := &corev1.ServiceAccount{}, createRuntimeComponent(objMeta, spec)
	CustomizeServiceAccount(sa, runtime, fclSecret)
	emptySAIPS := sa.ImagePullSecrets[0].Name

	newSecret := "my-new-secret"
	spec = appstacksv1.RuntimeComponentSpec{PullSecret: &newSecret}
	runtime = createRuntimeComponent(objMeta, spec)
	CustomizeServiceAccount(sa, runtime, fclSecret)

	testCSA := []Test{
		{"ServiceAccount image pull secrets is empty", pullSecret, emptySAIPS},
		{"ServiceAccount image pull secrets", newSecret, sa.ImagePullSecrets[1].Name},
	}
	verifyTests(testCSA, t)

	wrongPullSecret := "wrong-pull-secret"
	runtime.Spec.PullSecret = &wrongPullSecret
	CustomizeServiceAccount(sa, runtime, fclSecret)

	testCSA = []Test{
		{"ServiceAccount image pull secret is deleted", 1, len(sa.ImagePullSecrets)},
	}
	verifyTests(testCSA, t)

}

func TestCustomizeKnativeService(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Service:          service,
		Probes:           probes,
		PullPolicy:       &pullPolicy,
		Env:              env,
		EnvFrom:          envFrom,
		Volumes:          []corev1.Volume{volume},
	}
	ksvc, runtime := &servingv1.Service{}, createRuntimeComponent(objMeta, spec)

	CustomizeKnativeService(ksvc, runtime)
	ksvcNumPorts := len(ksvc.Spec.Template.Spec.Containers[0].Ports)
	ksvcSAN := ksvc.Spec.Template.Spec.ServiceAccountName

	ksvcLPPort := ksvc.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Port
	ksvcLPTCP := ksvc.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket.Port
	ksvcRPPort := ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port
	ksvcRPTCP := ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe.TCPSocket.Port
	ksvcSPPort := ksvc.Spec.Template.Spec.Containers[0].StartupProbe.HTTPGet.Port
	ksvcSPTCP := ksvc.Spec.Template.Spec.Containers[0].StartupProbe.TCPSocket.Port
	ksvcLabelNoExpose := ksvc.Labels["serving.knative.dev/visibility"]

	spec = appstacksv1.RuntimeComponentSpec{
		ApplicationImage:   appImage,
		Service:            service,
		PullPolicy:         &pullPolicy,
		Env:                env,
		EnvFrom:            envFrom,
		Volumes:            []corev1.Volume{volume},
		ServiceAccountName: &serviceAccountName,
		Probes:             probes,
		Expose:             &expose,
	}
	runtime = createRuntimeComponent(objMeta, spec)
	CustomizeKnativeService(ksvc, runtime)
	ksvcLabelTrueExpose := ksvc.Labels["serving.knative.dev/visibility"]

	fls := false
	runtime.Spec.Expose = &fls
	CustomizeKnativeService(ksvc, runtime)
	ksvcLabelFalseExpose := ksvc.Labels["serving.knative.dev/visibility"]

	testCKS := []Test{
		{"ksvc container ports", 1, ksvcNumPorts},
		{"ksvc ServiceAccountName is nil", name, ksvcSAN},
		{"ksvc ServiceAccountName not nil", *runtime.Spec.ServiceAccountName, ksvc.Spec.Template.Spec.ServiceAccountName},
		{"liveness probe port", intstr.IntOrString{}, ksvcLPPort},
		{"liveness probe TCP socket port", intstr.IntOrString{}, ksvcLPTCP},
		{"Readiness probe port", intstr.IntOrString{}, ksvcRPPort},
		{"Readiness probe TCP socket port", intstr.IntOrString{}, ksvcRPTCP},
		{"Startup probe port", intstr.IntOrString{}, ksvcSPPort},
		{"Startup probe TCP socket port", intstr.IntOrString{}, ksvcSPTCP},
		{"expose not set", "cluster-local", ksvcLabelNoExpose},
		{"expose set to true", "", ksvcLabelTrueExpose},
		{"expose set to false", "cluster-local", ksvcLabelFalseExpose},
	}
	verifyTests(testCKS, t)
}

func TestCustomizeHPA(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1.RuntimeComponentSpec{Autoscaling: autoscaling}
	hpa, runtime := &autoscalingv1.HorizontalPodAutoscaler{}, createRuntimeComponent(objMeta, spec)
	CustomizeHPA(hpa, runtime)
	nilSTRKind := hpa.Spec.ScaleTargetRef.Kind

	runtimeStatefulSet := &appstacksv1.RuntimeComponentStatefulSet{Storage: &storage}
	spec = appstacksv1.RuntimeComponentSpec{Autoscaling: autoscaling, StatefulSet: runtimeStatefulSet}
	runtime = createRuntimeComponent(objMeta, spec)
	CustomizeHPA(hpa, runtime)
	STRKind := hpa.Spec.ScaleTargetRef.Kind

	testCHPA := []Test{
		{"Max replicas", autoscaling.MaxReplicas, hpa.Spec.MaxReplicas},
		{"Min replicas", *autoscaling.MinReplicas, *hpa.Spec.MinReplicas},
		{"Target CPU utilization", *autoscaling.TargetCPUUtilizationPercentage, *hpa.Spec.TargetCPUUtilizationPercentage},
		{"", name, hpa.Spec.ScaleTargetRef.Name},
		{"", "apps/v1", hpa.Spec.ScaleTargetRef.APIVersion},
		{"Storage not enabled", "Deployment", nilSTRKind},
		{"Storage enabled", "StatefulSet", STRKind},
	}
	verifyTests(testCHPA, t)
}

func TestValidate(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1.RuntimeComponentSpec{
		StatefulSet: &appstacksv1.RuntimeComponentStatefulSet{
			Storage: &appstacksv1.RuntimeComponentStorage{},
		},
	}
	runtime := createRuntimeComponent(objMeta, spec)
	valid1, _ := Validate(runtime)

	spec = appstacksv1.RuntimeComponentSpec{
		StatefulSet: &appstacksv1.RuntimeComponentStatefulSet{
			Storage: &appstacksv1.RuntimeComponentStorage{
				Size: "size",
			},
		},
	}
	runtime = createRuntimeComponent(objMeta, spec)
	valid2, _ := Validate(runtime)

	spec = appstacksv1.RuntimeComponentSpec{StatefulSet: &appstacksv1.RuntimeComponentStatefulSet{Storage: &storage}}
	runtime = createRuntimeComponent(objMeta, spec)
	valid3, _ := Validate(runtime)

	testValidate := []Test{
		{"StatefulSet storage validation fail from empty size", false, valid1},
		{"StatefulSet storage validation fail from size parsing error", false, valid2},
		{"StatefulSet storage validation passed", true, valid3},
	}
	verifyTests(testValidate, t)
}

func TestCustomizeServiceMonitor(t *testing.T) {

	logger := zap.New()
	logf.SetLogger(logger)
	spec := appstacksv1.RuntimeComponentSpec{Service: service}

	params := map[string][]string{
		"params": {"param1", "param2"},
	}
	targetPortConfig := intstr.FromInt(9000)

	// Endpoint for runtime
	endpointApp := &prometheusv1.Endpoint{
		Port:            "web",
		TargetPort:      &targetPortConfig,
		Scheme:          "myScheme",
		Interval:        "myInterval",
		Path:            "myPath",
		TLSConfig:       &prometheusv1.TLSConfig{},
		BasicAuth:       &prometheusv1.BasicAuth{},
		Params:          params,
		ScrapeTimeout:   "myScrapeTimeout",
		BearerTokenFile: "myBearerTokenFile",
	}
	endpointsApp := make([]prometheusv1.Endpoint, 1)
	endpointsApp[0] = *endpointApp

	// Endpoint for sm
	endpointsSM := make([]prometheusv1.Endpoint, 0)

	labelMap := map[string]string{"app": "my-app"}
	selector := &metav1.LabelSelector{MatchLabels: labelMap}
	smspec := &prometheusv1.ServiceMonitorSpec{Endpoints: endpointsSM, Selector: *selector}

	sm, runtime := &prometheusv1.ServiceMonitor{Spec: *smspec}, createRuntimeComponent(objMeta, spec)
	runtime.Spec.Monitoring = &appstacksv1.RuntimeComponentMonitoring{Labels: labelMap, Endpoints: endpointsApp}

	CustomizeServiceMonitor(sm, runtime)

	labelMatches := map[string]string{
		"monitor.rc.app.stacks/enabled": "true",
		"app.kubernetes.io/instance":    name,
	}

	allSMLabels := runtime.GetLabels()
	for key, value := range runtime.Spec.Monitoring.Labels {
		allSMLabels[key] = value
	}

	// Expected values
	appPort := runtime.Spec.Monitoring.Endpoints[0].Port
	appScheme := runtime.Spec.Monitoring.Endpoints[0].Scheme
	appInterval := runtime.Spec.Monitoring.Endpoints[0].Interval
	appPath := runtime.Spec.Monitoring.Endpoints[0].Path
	appTLSConfig := runtime.Spec.Monitoring.Endpoints[0].TLSConfig
	appBasicAuth := runtime.Spec.Monitoring.Endpoints[0].BasicAuth
	appParams := runtime.Spec.Monitoring.Endpoints[0].Params
	appScrapeTimeout := runtime.Spec.Monitoring.Endpoints[0].ScrapeTimeout
	appBearerTokenFile := runtime.Spec.Monitoring.Endpoints[0].BearerTokenFile

	testSM := []Test{
		{"Service Monitor label for app.kubernetes.io/instance", name, sm.Labels["app.kubernetes.io/instance"]},
		{"Service Monitor selector match labels", labelMatches, sm.Spec.Selector.MatchLabels},
		{"Service Monitor endpoints port", appPort, sm.Spec.Endpoints[0].Port},
		{"Service Monitor all labels", allSMLabels, sm.Labels},
		{"Service Monitor endpoints scheme", appScheme, sm.Spec.Endpoints[0].Scheme},
		{"Service Monitor endpoints interval", appInterval, sm.Spec.Endpoints[0].Interval},
		{"Service Monitor endpoints path", appPath, sm.Spec.Endpoints[0].Path},
		{"Service Monitor endpoints TLSConfig", appTLSConfig, sm.Spec.Endpoints[0].TLSConfig},
		{"Service Monitor endpoints basicAuth", appBasicAuth, sm.Spec.Endpoints[0].BasicAuth},
		{"Service Monitor endpoints params", appParams, sm.Spec.Endpoints[0].Params},
		{"Service Monitor endpoints scrapeTimeout", appScrapeTimeout, sm.Spec.Endpoints[0].ScrapeTimeout},
		{"Service Monitor endpoints bearerTokenFile", appBearerTokenFile, sm.Spec.Endpoints[0].BearerTokenFile},
	}

	verifyTests(testSM, t)

	var nilTargetPortConfig *intstr.IntOrString
	runtime.Spec.Monitoring.Endpoints[0].Port = ""
	runtime.Spec.Monitoring.Endpoints[0].TargetPort = nilTargetPortConfig
	CustomizeServiceMonitor(sm, runtime)
	smPort := sm.Spec.Endpoints[0].Port

	runtime.Spec.Service.PortName = ""
	CustomizeServiceMonitor(sm, runtime)
	smPortTCP := sm.Spec.Endpoints[0].Port

	runtime.Spec.Monitoring = &appstacksv1.RuntimeComponentMonitoring{Labels: labelMap}
	CustomizeServiceMonitor(sm, runtime)
	serverName := name + "." + namespace + ".svc"

	testSM = []Test{
		{"Service Monitor endpoints port", svcPortName, smPort},
		{"Service Monitor endpoints port without port name", strconv.Itoa(int(runtime.Spec.Service.Port)) + "-tcp", smPortTCP},
		{"Service Monitor server name", serverName, sm.Spec.Endpoints[0].TLSConfig.ServerName},
	}

	verifyTests(testSM, t)
}

func TestGetCondition(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)
	status := &appstacksv1.RuntimeComponentStatus{
		Conditions: []appstacksv1.StatusCondition{
			{
				Status: corev1.ConditionTrue,
				Type:   appstacksv1.StatusConditionTypeReconciled,
			},
		},
	}
	conditionType := appstacksv1.StatusConditionTypeReconciled
	cond := GetCondition(conditionType, status)
	testGC := []Test{{"Set status condition", status.Conditions[0].Status, cond.Status}}
	verifyTests(testGC, t)

	status = &appstacksv1.RuntimeComponentStatus{}
	cond = GetCondition(conditionType, status)
	testGC = []Test{{"Set status condition", 0, len(status.Conditions)}}
	verifyTests(testGC, t)
}

func TestSetCondition(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)
	status := &appstacksv1.RuntimeComponentStatus{
		Conditions: []appstacksv1.StatusCondition{
			{Type: appstacksv1.StatusConditionTypeReconciled},
		},
	}
	condition := appstacksv1.StatusCondition{
		Status: corev1.ConditionTrue,
		Type:   appstacksv1.StatusConditionTypeReconciled,
	}
	SetCondition(condition, status)
	testSC := []Test{{"Set status condition", condition.Status, status.Conditions[0].Status}}
	verifyTests(testSC, t)
}

func TestGetWatchNamespaces(t *testing.T) {
	// Set the logger to development mode for verbose logs
	logger := zap.New()
	logf.SetLogger(logger)

	os.Setenv("WATCH_NAMESPACE", "")
	namespaces, err := GetWatchNamespaces()
	configMapConstTests := []Test{
		{"namespaces", []string{""}, namespaces},
		{"error", nil, err},
	}
	verifyTests(configMapConstTests, t)

	os.Setenv("WATCH_NAMESPACE", "ns1")
	namespaces, err = GetWatchNamespaces()
	configMapConstTests = []Test{
		{"namespaces", []string{"ns1"}, namespaces},
		{"error", nil, err},
	}
	verifyTests(configMapConstTests, t)

	os.Setenv("WATCH_NAMESPACE", "ns1,ns2,ns3")
	namespaces, err = GetWatchNamespaces()
	configMapConstTests = []Test{
		{"namespaces", []string{"ns1", "ns2", "ns3"}, namespaces},
		{"error", nil, err},
	}
	verifyTests(configMapConstTests, t)

	os.Setenv("WATCH_NAMESPACE", " ns1   ,  ns2,  ns3  ")
	namespaces, err = GetWatchNamespaces()
	configMapConstTests = []Test{
		{"namespaces", []string{"ns1", "ns2", "ns3"}, namespaces},
		{"error", nil, err},
	}
	verifyTests(configMapConstTests, t)
}

func TestBuildServiceBindingSecretName(t *testing.T) {
	// Set the logger to development mode for verbose logs
	logger := zap.New()
	logf.SetLogger(logger)

	sbSecretName := BuildServiceBindingSecretName(name, namespace)
	sbSecretNameTests := []Test{
		{"Service binding secret name", namespace + "-" + name, sbSecretName},
	}
	verifyTests(sbSecretNameTests, t)
}

func TestAppendIfNotSubstring(t *testing.T) {
	// Set the logger to development mode for verbose logs
	logger := zap.New()
	logf.SetLogger(logger)

	subStr := "c"
	str := "a,b"

	result1 := AppendIfNotSubstring(subStr, "")
	result2 := AppendIfNotSubstring(subStr, str)
	result3 := AppendIfNotSubstring(subStr, result2)
	subStrTest := []Test{
		{"Substring check when string is empty", subStr, result1},
		{"Substring check when not substring", str + "," + subStr, result2},
		{"Substring check when substring", str + "," + subStr, result3},
	}
	verifyTests(subStrTest, t)
}

func TestEnsureOwnerRef(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1.RuntimeComponentSpec{Service: service}
	runtime := createRuntimeComponent(objMeta, spec)

	newOwnerRef := metav1.OwnerReference{APIVersion: "test", Kind: "test", Name: "testRef"}
	EnsureOwnerRef(runtime, newOwnerRef)

	testOR := []Test{
		{"OpenShiftAnnotations", runtime.GetOwnerReferences()[0], newOwnerRef},
	}
	verifyTests(testOR, t)
}

func TestGetOpenShiftAnnotations(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1.RuntimeComponentSpec{Service: service}
	runtime := createRuntimeComponent(objMeta, spec)

	annos := map[string]string{
		"image.opencontainers.org/source": "source",
	}
	runtime.Annotations = annos

	result := GetOpenShiftAnnotations(runtime)

	annos = map[string]string{
		"app.openshift.io/vcs-uri": "source",
	}
	testOSA := []Test{
		{"OpenShiftAnnotations", annos["app.openshift.io/vcs-uri"], result["app.openshift.io/vcs-uri"]},
	}
	verifyTests(testOSA, t)
}

func TestIsClusterWide(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	namespaces := []string{"namespace"}
	result := IsClusterWide(namespaces)

	testCW := []Test{
		{"One namespace", false, result},
	}
	verifyTests(testCW, t)

	namespaces = []string{""}
	result = IsClusterWide(namespaces)

	testCW = []Test{
		{"All namespaces", true, result},
	}
	verifyTests(testCW, t)

	namespaces = []string{"namespace1", "namespace2"}
	result = IsClusterWide(namespaces)

	testCW = []Test{
		{"Two namespaces", false, result},
	}
	verifyTests(testCW, t)
}

func TestCustomizeIngress(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	var ISPathType networkingv1.PathType = networkingv1.PathType("ImplementationSpecific")
	var prefixPathType networkingv1.PathType = networkingv1.PathType("Prefix")
	ing := networkingv1.Ingress{}

	route := appstacksv1.RuntimeComponentRoute{}
	spec := appstacksv1.RuntimeComponentSpec{Service: service, Route: &route}
	runtime := createRuntimeComponent(objMeta, spec)
	CustomizeIngress(&ing, runtime)
	defaultPathType := *ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].PathType

	route = appstacksv1.RuntimeComponentRoute{Host: "routeHost", Path: "myPath", PathType: prefixPathType, Annotations: annotations}
	spec = appstacksv1.RuntimeComponentSpec{Service: service, Route: &route}
	runtime = createRuntimeComponent(objMeta, spec)
	CustomizeIngress(&ing, runtime)

	testIng := []Test{
		{"Ingress Annotations", annotations, ing.Annotations},
		{"Ingress Route Host", "routeHost", ing.Spec.Rules[0].Host},
		{"Ingress Route Path", "myPath", ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path},
		{"Ingress Route PathType", prefixPathType, *ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].PathType},
		{"Ingress Route Default PathType", ISPathType, defaultPathType},
		{"Ingress Route ServiceName", name, ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name},
		{"Ingress Route Service Port Name", strconv.Itoa(int(runtime.Spec.Service.Port)) + "-tcp", ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Name},
		{"Ingress TLS", 0, len(ing.Spec.TLS)},
	}
	verifyTests(testIng, t)

	certSecretRef := "my-ref"
	route = appstacksv1.RuntimeComponentRoute{Host: "routeHost", Path: "myPath", CertificateSecretRef: &certSecretRef}

	CustomizeIngress(&ing, runtime)

	testIng = []Test{
		{"Ingress TLS SecretName", certSecretRef, ing.Spec.TLS[0].SecretName},
	}
	verifyTests(testIng, t)
}

// Helper Functions
// Unconditionally set the proper tags for an enabled runtime omponent
func createAppDefinitionTags(app *appstacksv1.RuntimeComponent) (map[string]string, map[string]string) {
	// The purpose of this function demands all fields configured
	if app.Spec.ApplicationVersion == "" {
		app.Spec.ApplicationVersion = "v1alpha"
	}
	// set fields
	label := map[string]string{
		"kappnav.app.auto-create": "true",
	}
	annotations := map[string]string{
		"kappnav.app.auto-create.name":          app.Spec.ApplicationName,
		"kappnav.app.auto-create.kinds":         "Deployment, StatefulSet, Service, Route, Ingress, ConfigMap",
		"kappnav.app.auto-create.label":         "app.kubernetes.io/part-of",
		"kappnav.app.auto-create.labels-values": app.Spec.ApplicationName,
		"kappnav.app.auto-create.version":       app.Spec.ApplicationVersion,
	}
	return label, annotations
}
func createRuntimeComponent(objMeta metav1.ObjectMeta, spec appstacksv1.RuntimeComponentSpec) *appstacksv1.RuntimeComponent {
	app := &appstacksv1.RuntimeComponent{
		ObjectMeta: metav1.ObjectMeta{Name: objMeta.GetName(), Namespace: objMeta.GetNamespace()},
		Spec:       spec,
	}
	return app
}

// Used in TestCustomizeAffinity to make an IN selector with paramenters key and values.
func makeInLabelSelector(key string, values []string) metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      key,
				Operator: metav1.LabelSelectorOpIn,
				Values:   values,
			},
		},
	}
}

func verifyTests(tests []Test, t *testing.T) {
	for _, tt := range tests {
		if !reflect.DeepEqual(tt.expected, tt.actual) {
			t.Errorf("%s test expected: (%v) actual: (%v)", tt.test, tt.expected, tt.actual)
		}
	}
}
