package runtimeapplication

import (
	"context"
	"os"
	"strconv"
	"testing"

	runtimeappv1beta1 "github.com/application-runtimes/operator/pkg/apis/runtimeapp/v1beta1"
	runtimeapputils "github.com/application-runtimes/operator/pkg/utils"
	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	certmngrv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"

	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	coretesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	name                       = "app"
	namespace                  = "runtimeapp"
	appImage                   = "my-image"
	ksvcAppImage               = "ksvc-image"
	replicas             int32 = 3
	autoscaling                = &runtimeappv1beta1.RuntimeApplicationAutoScaling{MaxReplicas: 3}
	pullPolicy                 = corev1.PullAlways
	serviceType                = corev1.ServiceTypeClusterIP
	service                    = &runtimeappv1beta1.RuntimeApplicationService{Type: &serviceType, Port: 8080}
	expose                     = true
	serviceAccountName         = "service-account"
	volumeCT                   = &corev1.PersistentVolumeClaim{TypeMeta: metav1.TypeMeta{Kind: "StatefulSet"}}
	storage                    = runtimeappv1beta1.RuntimeApplicationStorage{Size: "10Mi", MountPath: "/mnt/data", VolumeClaimTemplate: volumeCT}
	createKnativeService       = true
	statefulSetSN              = name + "-headless"
)

type Test struct {
	test     string
	expected interface{}
	actual   interface{}
}

func TestRuntimeController(t *testing.T) {
	// Set the logger to development mode for verbose logs
	logf.SetLogger(logf.ZapLogger(true))
	os.Setenv("WATCH_NAMESPACE", namespace)

	spec := runtimeappv1beta1.RuntimeApplicationSpec{}
	runtimeapp := createRuntimeApp(name, namespace, spec)

	// Set objects to track in the fake client and register operator types with the runtime scheme.
	objs, s := []runtime.Object{runtimeapp}, scheme.Scheme

	// Add third party resrouces to scheme
	if err := servingv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add servingv1alpha1 scheme: (%v)", err)
	}

	if err := routev1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add route scheme: (%v)", err)
	}

	if err := imagev1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add image scheme: (%v)", err)
	}

	if err := certmngrv1alpha2.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add cert-manager scheme: (%v)", err)
	}

	if err := prometheusv1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add prometheus scheme: (%v)", err)
	}

	s.AddKnownTypes(runtimeappv1beta1.SchemeGroupVersion, runtimeapp)
	s.AddKnownTypes(certmngrv1alpha2.SchemeGroupVersion, &certmngrv1alpha2.Certificate{})
	s.AddKnownTypes(prometheusv1.SchemeGroupVersion, &prometheusv1.ServiceMonitor{})

	// Create a fake client to mock API calls.
	cl := fakeclient.NewFakeClient(objs...)

	rb := runtimeapputils.NewReconcilerBase(cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	// Create a ReconcileRuntimeApplication object
	r := &ReconcileRuntimeApplication{ReconcilerBase: rb}
	r.SetDiscoveryClient(createFakeDiscoveryClient())

	// Mock request to simulate Reconcile being called on an event for a watched resource
	// then ensure reconcile is successful and does not return an empty result
	req := createReconcileRequest(name, namespace)
	res, err := r.Reconcile(req)
	verifyReconcile(res, err, t)

	// Check if deployment has been created
	dep := &appsv1.Deployment{}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, dep); err != nil {
		t.Fatalf("Get Deployment: (%v)", err)
	}

	depTests := []Test{
		{"service account name", "app", dep.Spec.Template.Spec.ServiceAccountName},
	}
	verifyTests("dep", depTests, t)

	// Update runtimeapp with values for StatefulSet
	// Update ServiceAccountName for empty case
	runtimeapp.Spec = runtimeappv1beta1.RuntimeApplicationSpec{
		Storage:          &storage,
		Replicas:         &replicas,
		ApplicationImage: appImage,
	}
	updateRuntimeApp(r, runtimeapp, t)

	// Reconcile again to check for the StatefulSet and updated resources
	res, err = r.Reconcile(req)
	verifyReconcile(res, err, t)

	// Check if StatefulSet has been created
	statefulSet := &appsv1.StatefulSet{}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, statefulSet); err != nil {
		t.Fatalf("Get StatefulSet: (%v)", err)
	}

	// Storage is enabled so the deployment should be deleted
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, dep); err == nil {
		t.Fatalf("Deployment was not deleted")
	}

	// Check updated values in StatefulSet
	ssTests := []Test{
		{"replicas", replicas, *statefulSet.Spec.Replicas},
		{"service image name", appImage, statefulSet.Spec.Template.Spec.Containers[0].Image},
		{"pull policy", name, statefulSet.Spec.Template.Spec.ServiceAccountName},
		{"service account name", statefulSetSN, statefulSet.Spec.ServiceName},
	}
	verifyTests("statefulSet", ssTests, t)

	// Enable CreateKnativeService
	runtimeapp.Spec = runtimeappv1beta1.RuntimeApplicationSpec{
		CreateKnativeService: &createKnativeService,
		PullPolicy:           &pullPolicy,
		ApplicationImage:     ksvcAppImage,
	}
	updateRuntimeApp(r, runtimeapp, t)

	// Reconcile again to check for the KNativeService and updated resources
	res, err = r.Reconcile(req)
	verifyReconcile(res, err, t)

	// Create KnativeService
	ksvc := &servingv1alpha1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "serving.knative.dev/v1alpha1",
			Kind:       "Service",
		},
	}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, ksvc); err != nil {
		t.Fatalf("Get KnativeService: (%v)", err)
	}

	// KnativeService is enabled so non-Knative resources should be deleted
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, statefulSet); err == nil {
		t.Fatalf("StatefulSet was not deleted")
	}

	// Check updated values in KnativeService
	ksvcTests := []Test{
		{"service image name", ksvcAppImage, ksvc.Spec.Template.Spec.Containers[0].Image},
		{"pull policy", pullPolicy, ksvc.Spec.Template.Spec.Containers[0].ImagePullPolicy},
		{"service account name", name, ksvc.Spec.Template.Spec.ServiceAccountName},
	}
	verifyTests("ksvc", ksvcTests, t)

	// Disable Knative and enable Expose to test route
	runtimeapp.Spec = runtimeappv1beta1.RuntimeApplicationSpec{Expose: &expose}
	updateRuntimeApp(r, runtimeapp, t)

	// Reconcile again to check for the route and updated resources
	res, err = r.Reconcile(req)
	verifyReconcile(res, err, t)

	// Create Route
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{APIVersion: "route.openshift.io/v1", Kind: "Route"},
	}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, route); err != nil {
		t.Fatalf("Get Route: (%v)", err)
	}

	// Check updated values in Route
	routeTests := []Test{{"target port", intstr.FromString(strconv.Itoa(int(service.Port)) + "-tcp"), route.Spec.Port.TargetPort}}
	verifyTests("route", routeTests, t)

	// Disable Route/Expose and enable Autoscaling
	runtimeapp.Spec = runtimeappv1beta1.RuntimeApplicationSpec{
		Autoscaling: autoscaling,
	}
	updateRuntimeApp(r, runtimeapp, t)

	// Reconcile again to check for hpa and updated resources
	res, err = r.Reconcile(req)
	verifyReconcile(res, err, t)

	// Create HorizontalPodAutoscaler
	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, hpa); err != nil {
		t.Fatalf("Get HPA: (%v)", err)
	}

	// Expose is disabled so route should be deleted
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, route); err == nil {
		t.Fatal("Route was not deleted")
	}

	// Check updated values in hpa
	hpaTests := []Test{{"max replicas", autoscaling.MaxReplicas, hpa.Spec.MaxReplicas}}
	verifyTests("hpa", hpaTests, t)

	// Remove autoscaling to ensure hpa is deleted
	runtimeapp.Spec.Autoscaling = nil
	updateRuntimeApp(r, runtimeapp, t)

	res, err = r.Reconcile(req)
	verifyReconcile(res, err, t)

	// Autoscaling is disabled so hpa should be deleted
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, hpa); err == nil {
		t.Fatal("hpa was not deleted")
	}

	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, runtimeapp); err != nil {
		t.Fatalf("Get runtimeapp: (%v)", err)
	}

	// Update runtimeapp to ensure it requeues
	runtimeapp.SetGeneration(1)
	updateRuntimeApp(r, runtimeapp, t)

	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
}

// Helper Functions
func createRuntimeApp(n, ns string, spec runtimeappv1beta1.RuntimeApplicationSpec) *runtimeappv1beta1.RuntimeApplication {
	app := &runtimeappv1beta1.RuntimeApplication{
		ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns},
		Spec:       spec,
	}
	return app
}

func createFakeDiscoveryClient() discovery.DiscoveryInterface {
	fakeDiscoveryClient := &fakediscovery.FakeDiscovery{Fake: &coretesting.Fake{}}
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: routev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "routes", Namespaced: true, Kind: "Route"},
			},
		},
		{
			GroupVersion: servingv1alpha1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "services", Namespaced: true, Kind: "Service", SingularName: "service"},
			},
		},
		{
			GroupVersion: certmngrv1alpha2.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "certificates", Namespaced: true, Kind: "Certificate", SingularName: "certificate"},
			},
		},
		{
			GroupVersion: prometheusv1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "servicemonitors", Namespaced: true, Kind: "ServiceMonitor", SingularName: "servicemonitor"},
			},
		},
		{
			GroupVersion: imagev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "imagestreams", Namespaced: true, Kind: "ImageStream", SingularName: "imagestream"},
			},
		},
	}

	return fakeDiscoveryClient
}

func createReconcileRequest(n, ns string) reconcile.Request {
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Name: n, Namespace: ns},
	}
	return req
}

func createConfigMap(n, ns string, data map[string]string) *corev1.ConfigMap {
	app := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns},
		Data:       data,
	}
	return app
}

func verifyReconcile(res reconcile.Result, err error, t *testing.T) {
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	if res != (reconcile.Result{}) {
		t.Errorf("reconcile did not return an empty result (%v)", res)
	}
}

func verifyTests(n string, tests []Test, t *testing.T) {
	for _, tt := range tests {
		if tt.actual != tt.expected {
			t.Errorf("%s %s test expected: (%v) actual: (%v)", n, tt.test, tt.expected, tt.actual)
		}
	}
}

func updateRuntimeApp(r *ReconcileRuntimeApplication, runtimeapp *runtimeappv1beta1.RuntimeApplication, t *testing.T) {
	if err := r.GetClient().Update(context.TODO(), runtimeapp); err != nil {
		t.Fatalf("Update runtimeapp: (%v)", err)
	}
}
