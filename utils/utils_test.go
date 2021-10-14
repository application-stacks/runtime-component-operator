package utils

import (
	"os"
	"reflect"
	"strconv"
	"testing"

	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/api/v1beta1"
	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	name               = "my-app"
	namespace          = "runtime"
	stack              = "java-microprofile"
	appImage           = "my-image"
	replicas     int32 = 2
	expose             = true
	createKNS          = true
	targetCPUPer int32 = 30
	targetPort   int32 = 3333
	nodePort     int32 = 3011
	autoscaling        = &appstacksv1beta1.RuntimeComponentAutoScaling{
		TargetCPUUtilizationPercentage: &targetCPUPer,
		MinReplicas:                    &replicas,
		MaxReplicas:                    3,
	}
	envFrom            = []corev1.EnvFromSource{{Prefix: namespace}}
	env                = []corev1.EnvVar{{Name: namespace}}
	pullPolicy         = corev1.PullAlways
	pullSecret         = "mysecret"
	serviceAccountName = "service-account"
	serviceType        = corev1.ServiceTypeClusterIP
	service            = &appstacksv1beta1.RuntimeComponentService{Type: &serviceType, Port: 8443}
	deploymentAnnos    = map[string]string{"depAnno": "depAnno"}
	deployment         = &appstacksv1beta1.RuntimeComponentDeployment{Annotations: deploymentAnnos}
	ssAnnos            = map[string]string{"setAnno": "setAnno"}
	statefulSet        = &appstacksv1beta1.RuntimeComponentStatefulSet{Annotations: ssAnnos}
	volumeCT           = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc", Namespace: namespace},
		TypeMeta:   metav1.TypeMeta{Kind: "StatefulSet"}}
	storage        = appstacksv1beta1.RuntimeComponentStorage{Size: "10Mi", MountPath: "/mnt/data", VolumeClaimTemplate: volumeCT}
	arch           = []string{"ppc64le"}
	readinessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet:   &corev1.HTTPGetAction{},
			TCPSocket: &corev1.TCPSocketAction{},
		},
	}
	livenessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet:   &corev1.HTTPGetAction{},
			TCPSocket: &corev1.TCPSocketAction{},
		},
	}
	startupProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet:   &corev1.HTTPGetAction{},
			TCPSocket: &corev1.TCPSocketAction{},
		},
	}
	volume      = corev1.Volume{Name: "runtime-volume"}
	volumeMount = corev1.VolumeMount{Name: volumeCT.Name, MountPath: storage.MountPath}
	resLimits   = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU: {},
	}
	resourceContraints = &corev1.ResourceRequirements{Limits: resLimits}
)

type Test struct {
	test     string
	expected interface{}
	actual   interface{}
}

func TestCustomizeRoute(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)
	spec := appstacksv1beta1.RuntimeComponentSpec{Service: service}
	route, runtime := &routev1.Route{}, createRuntimeComponent(name, namespace, spec)

	CustomizeRoute(route, runtime, "", "", "", "")

	//TestGetLabels
	testCR := []Test{
		{"Route labels", name, route.Labels["app.kubernetes.io/instance"]},
		{"Route target kind", "Service", route.Spec.To.Kind},
		{"Route target name", name, route.Spec.To.Name},
		{"Route target weight", int32(100), *route.Spec.To.Weight},
		{"Route service target port", intstr.FromString(strconv.Itoa(int(runtime.Spec.Service.Port)) + "-tcp"), route.Spec.Port.TargetPort},
	}

	verifyTests(testCR, t)
}

func TestCustomizeService(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1beta1.RuntimeComponentSpec{Service: service}
	svc, runtime := &corev1.Service{}, createRuntimeComponent(name, namespace, spec)

	CustomizeService(svc, runtime)
	testCS := []Test{
		{"Service number of exposed ports", 1, len(svc.Spec.Ports)},
		{"Sercice first exposed port", runtime.Spec.Service.Port, svc.Spec.Ports[0].Port},
		{"Service first exposed target port", intstr.FromInt(int(runtime.Spec.Service.Port)), svc.Spec.Ports[0].TargetPort},
		{"Service type", *runtime.Spec.Service.Type, svc.Spec.Type},
		{"Service selector", name, svc.Spec.Selector["app.kubernetes.io/instance"]},
	}
	verifyTests(testCS, t)

	// Verify behaviour of optional target port functionality
	verifyTests(optionalTargetPortFunctionalityTests(), t)

	// verify optional nodePort functionality in NodePort service
	verifyTests(optionalNodePortFunctionalityTests(), t)
}

func optionalTargetPortFunctionalityTests() []Test {
	spec := appstacksv1beta1.RuntimeComponentSpec{Service: service}
	spec.Service.TargetPort = &targetPort
	svc, runtime := &corev1.Service{}, createRuntimeComponent(name, namespace, spec)

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
	serviceType := corev1.ServiceTypeNodePort
	service := &appstacksv1beta1.RuntimeComponentService{Type: &serviceType, Port: 8443, NodePort: &nodePort}
	spec := appstacksv1beta1.RuntimeComponentSpec{Service: service}
	svc, runtime := &corev1.Service{}, createRuntimeComponent(name, namespace, spec)

	CustomizeService(svc, runtime)
	testCS := []Test{
		{"Service number of exposed ports", 1, len(svc.Spec.Ports)},
		{"Sercice first exposed port", runtime.Spec.Service.Port, svc.Spec.Ports[0].Port},
		{"Service first exposed target port", intstr.FromInt(int(runtime.Spec.Service.Port)), svc.Spec.Ports[0].TargetPort},
		{"Service type", *runtime.Spec.Service.Type, svc.Spec.Type},
		{"Service selector", name, svc.Spec.Selector["app.kubernetes.io/instance"]},
		{"Service nodePort port", *runtime.Spec.Service.NodePort, svc.Spec.Ports[0].NodePort},
	}
	return testCS
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
	affinityConfig := appstacksv1beta1.RuntimeComponentAffinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution:  &rDSIDE,
			PreferredDuringSchedulingIgnoredDuringExecution: pDSIDE,
		},
		NodeAffinityLabels: labels,
	}
	spec := appstacksv1beta1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Affinity:         &affinityConfig,
	}
	affinity, runtime := &corev1.Affinity{}, createRuntimeComponent(name, namespace, spec)
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
	affinityConfig := appstacksv1beta1.RuntimeComponentAffinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: rDSIDE,
		},
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: pDSIDE,
		},
	}
	spec := appstacksv1beta1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Affinity:         &affinityConfig,
	}
	affinity, runtime := &corev1.Affinity{}, createRuntimeComponent(name, namespace, spec)
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

	spec := appstacksv1beta1.RuntimeComponentSpec{
		ApplicationImage:    appImage,
		Service:             service,
		ResourceConstraints: resourceContraints,
		ReadinessProbe:      readinessProbe,
		LivenessProbe:       livenessProbe,
		StartupProbe:        startupProbe,
		VolumeMounts:        []corev1.VolumeMount{volumeMount},
		PullPolicy:          &pullPolicy,
		Env:                 env,
		EnvFrom:             envFrom,
		Volumes:             []corev1.Volume{volume},
	}

	// No dep or set, annotation should be empty
	pts1, runtime1 := &corev1.PodTemplateSpec{}, createRuntimeComponent(name, namespace, spec)
	CustomizePodSpec(pts1, runtime1)
	annolen1 := len(pts1.Annotations)
	testAnnotations1 := []Test{
		{"Shouldn't be any annotations", 0, annolen1},
	}
	verifyTests(testAnnotations1, t)

	// dep but not set, annotation should be dep annotations
	spec.Deployment = deployment
	pts2, runtime2 := &corev1.PodTemplateSpec{}, createRuntimeComponent(name, namespace, spec)
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
	pts3, runtime3 := &corev1.PodTemplateSpec{}, createRuntimeComponent(name, namespace, spec)
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
	pts4, runtime4 := &corev1.PodTemplateSpec{}, createRuntimeComponent(name, namespace, spec)
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

	spec := appstacksv1beta1.RuntimeComponentSpec{
		ApplicationImage:    appImage,
		Service:             service,
		ResourceConstraints: resourceContraints,
		ReadinessProbe:      readinessProbe,
		LivenessProbe:       livenessProbe,
		StartupProbe:        startupProbe,
		VolumeMounts:        []corev1.VolumeMount{volumeMount},
		PullPolicy:          &pullPolicy,
		Env:                 env,
		EnvFrom:             envFrom,
		Volumes:             []corev1.Volume{volume},
	}
	pts, runtime := &corev1.PodTemplateSpec{}, createRuntimeComponent(name, namespace, spec)
	// else cond
	CustomizePodSpec(pts, runtime)
	noCont := len(pts.Spec.Containers)
	noPorts := len(pts.Spec.Containers[0].Ports)
	ptsSAN := pts.Spec.ServiceAccountName
	// if cond
	spec = appstacksv1beta1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Service: &appstacksv1beta1.RuntimeComponentService{
			Type:       &serviceType,
			Port:       8443,
			TargetPort: &targetPort,
		},
		ResourceConstraints: resourceContraints,
		ReadinessProbe:      readinessProbe,
		LivenessProbe:       livenessProbe,
		StartupProbe:        startupProbe,
		VolumeMounts:        []corev1.VolumeMount{volumeMount},
		PullPolicy:          &pullPolicy,
		Env:                 env,
		EnvFrom:             envFrom,
		Volumes:             []corev1.Volume{volume},
		Architecture:        arch,
		ServiceAccountName:  &serviceAccountName,
	}
	runtime = createRuntimeComponent(name, namespace, spec)
	CustomizePodSpec(pts, runtime)
	ptsCSAN := pts.Spec.ServiceAccountName

	// affinity tests
	affArchs := pts.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
	weight := pts.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight
	prefAffArchs := pts.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Values[0]
	assignedTPort := pts.Spec.Containers[0].Ports[0].ContainerPort

	testCPS := []Test{
		{"No containers", 1, noCont},
		{"No port", 1, noPorts},
		{"No ServiceAccountName", name, ptsSAN},
		{"ServiceAccountName available", serviceAccountName, ptsCSAN},
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

func TestCustomizePersistence(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	runtimeStatefulSet := &appstacksv1beta1.RuntimeComponentStatefulSet{Storage: &storage}
	spec := appstacksv1beta1.RuntimeComponentSpec{StatefulSet: runtimeStatefulSet}
	statefulSet, runtime := &appsv1.StatefulSet{}, createRuntimeComponent(name, namespace, spec)
	statefulSet.Spec.Template.Spec.Containers = []corev1.Container{{}}
	statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{}
	// if vct == 0, runtimeVCT != nil, not found
	CustomizePersistence(statefulSet, runtime)
	ssK := statefulSet.Spec.VolumeClaimTemplates[0].TypeMeta.Kind
	ssMountPath := statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath

	//reset
	storageNilVCT := appstacksv1beta1.RuntimeComponentStorage{Size: "10Mi", MountPath: "/mnt/data", VolumeClaimTemplate: nil}
	runtimeStatefulSet = &appstacksv1beta1.RuntimeComponentStatefulSet{Storage: &storageNilVCT}
	spec = appstacksv1beta1.RuntimeComponentSpec{StatefulSet: runtimeStatefulSet}
	statefulSet, runtime = &appsv1.StatefulSet{}, createRuntimeComponent(name, namespace, spec)

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
	}
	verifyTests(testCP, t)
}

func TestCustomizeServiceAccount(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1beta1.RuntimeComponentSpec{PullSecret: &pullSecret}
	sa, runtime := &corev1.ServiceAccount{}, createRuntimeComponent(name, namespace, spec)
	CustomizeServiceAccount(sa, runtime)
	emptySAIPS := sa.ImagePullSecrets[0].Name

	newSecret := "my-new-secret"
	spec = appstacksv1beta1.RuntimeComponentSpec{PullSecret: &newSecret}
	runtime = createRuntimeComponent(name, namespace, spec)
	CustomizeServiceAccount(sa, runtime)

	testCSA := []Test{
		{"ServiceAccount image pull secrets is empty", pullSecret, emptySAIPS},
		{"ServiceAccount image pull secrets", newSecret, sa.ImagePullSecrets[0].Name},
	}
	verifyTests(testCSA, t)
}

func TestCustomizeKnativeService(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	spec := appstacksv1beta1.RuntimeComponentSpec{
		ApplicationImage: appImage,
		Service:          service,
		LivenessProbe:    livenessProbe,
		ReadinessProbe:   readinessProbe,
		StartupProbe:     startupProbe,
		PullPolicy:       &pullPolicy,
		Env:              env,
		EnvFrom:          envFrom,
		Volumes:          []corev1.Volume{volume},
	}
	ksvc, runtime := &servingv1.Service{}, createRuntimeComponent(name, namespace, spec)

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

	spec = appstacksv1beta1.RuntimeComponentSpec{
		ApplicationImage:   appImage,
		Service:            service,
		PullPolicy:         &pullPolicy,
		Env:                env,
		EnvFrom:            envFrom,
		Volumes:            []corev1.Volume{volume},
		ServiceAccountName: &serviceAccountName,
		LivenessProbe:      livenessProbe,
		ReadinessProbe:     readinessProbe,
		StartupProbe:       startupProbe,
		Expose:             &expose,
	}
	runtime = createRuntimeComponent(name, namespace, spec)
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

	spec := appstacksv1beta1.RuntimeComponentSpec{Autoscaling: autoscaling}
	hpa, runtime := &autoscalingv1.HorizontalPodAutoscaler{}, createRuntimeComponent(name, namespace, spec)
	CustomizeHPA(hpa, runtime)
	nilSTRKind := hpa.Spec.ScaleTargetRef.Kind

	runtimeStatefulSet := &appstacksv1beta1.RuntimeComponentStatefulSet{Storage: &storage}
	spec = appstacksv1beta1.RuntimeComponentSpec{Autoscaling: autoscaling, StatefulSet: runtimeStatefulSet}
	runtime = createRuntimeComponent(name, namespace, spec)
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

func TestCustomizeServiceMonitor(t *testing.T) {

	logger := zap.New()
	logf.SetLogger(logger)
	spec := appstacksv1beta1.RuntimeComponentSpec{Service: service}

	params := map[string][]string{
		"params": {"param1", "param2"},
	}

	// Endpoint for runtime
	endpointApp := &prometheusv1.Endpoint{
		Port:            "web",
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

	sm, runtime := &prometheusv1.ServiceMonitor{Spec: *smspec}, createRuntimeComponent(name, namespace, spec)
	runtime.Spec.Monitoring = &appstacksv1beta1.RuntimeComponentMonitoring{Labels: labelMap, Endpoints: endpointsApp}

	CustomizeServiceMonitor(sm, runtime)

	labelMatches := map[string]string{
		"monitor.app.stacks/enabled": "true",
		"app.kubernetes.io/instance": name,
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
}

func TestGetCondition(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)
	status := &appstacksv1beta1.RuntimeComponentStatus{
		Conditions: []appstacksv1beta1.StatusCondition{
			{
				Status: corev1.ConditionTrue,
				Type:   appstacksv1beta1.StatusConditionTypeReconciled,
			},
		},
	}
	conditionType := appstacksv1beta1.StatusConditionTypeReconciled
	cond := GetCondition(conditionType, status)
	testGC := []Test{{"Set status condition", status.Conditions[0].Status, cond.Status}}
	verifyTests(testGC, t)
}

func TestSetCondition(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)
	status := &appstacksv1beta1.RuntimeComponentStatus{
		Conditions: []appstacksv1beta1.StatusCondition{
			{Type: appstacksv1beta1.StatusConditionTypeReconciled},
		},
	}
	condition := appstacksv1beta1.StatusCondition{
		Status: corev1.ConditionTrue,
		Type:   appstacksv1beta1.StatusConditionTypeReconciled,
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

// Helper Functions
// Unconditionally set the proper tags for an enabled runtime omponent
func createAppDefinitionTags(app *appstacksv1beta1.RuntimeComponent) (map[string]string, map[string]string) {
	// The purpose of this function demands all fields configured
	if app.Spec.Version == "" {
		app.Spec.Version = "v1alpha"
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
		"kappnav.app.auto-create.version":       app.Spec.Version,
	}
	return label, annotations
}
func createRuntimeComponent(n, ns string, spec appstacksv1beta1.RuntimeComponentSpec) *appstacksv1beta1.RuntimeComponent {
	app := &appstacksv1beta1.RuntimeComponent{
		ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns},
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
		if !reflect.DeepEqual(tt.actual, tt.expected) {
			t.Errorf("%s test expected: (%v) actual: (%v)", tt.test, tt.expected, tt.actual)
		}
	}
}
