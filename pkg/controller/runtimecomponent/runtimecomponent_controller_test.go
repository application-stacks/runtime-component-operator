package runtimecomponent

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	appstacksutils "github.com/application-stacks/runtime-component-operator/pkg/utils"
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

	applicationsv1beta1 "sigs.k8s.io/application/pkg/apis/app/v1beta1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	name                       = "app"
	namespace                  = "runtimecomponent"
	appImage                   = "my-image"
	ksvcAppImage               = "ksvc-image"
	defaultMeta                = metav1.ObjectMeta{Name: name, Namespace: namespace}
	replicas             int32 = 3
	autoscaling                = &appstacksv1beta1.RuntimeComponentAutoScaling{MaxReplicas: 3}
	pullPolicy                 = corev1.PullAlways
	serviceType                = corev1.ServiceTypeClusterIP
	service                    = &appstacksv1beta1.RuntimeComponentService{Type: &serviceType, Port: 8080}
	expose                     = true
	serviceAccountName         = "service-account"
	volumeCT                   = &corev1.PersistentVolumeClaim{TypeMeta: metav1.TypeMeta{Kind: "StatefulSet"}}
	storage                    = appstacksv1beta1.RuntimeComponentStorage{Size: "10Mi", MountPath: "/mnt/data", VolumeClaimTemplate: volumeCT}
	createKnativeService       = true
	statefulSetSN              = name + "-headless"
	req                        = reconcile.Request{
		NamespacedName: types.NamespacedName{Name: name, Namespace: namespace},
	}
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

	runtimecomponent := makeBasicRuntimeComponent(name, namespace)

	// Set objects to track in the fake client and register operator types with the runtime scheme.
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme

	// Add third party resrouces to scheme
	addResourcesToScheme(t, s, runtimecomponent)

	// Create a fake client to mock API calls.
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	// Create a ReconcileRuntimeComponent object
	rb := appstacksutils.NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))
	r := &ReconcileRuntimeComponent{ReconcilerBase: rb}
	r.SetDiscoveryClient(createFakeDiscoveryClient())

	// Put test functions in slice
	testFuncs := []func(*testing.T, *ReconcileRuntimeComponent, appstacksutils.ReconcilerBase) error{
		testBasicReconcile,
		testStorage,
		testKnativeService,
		testExposeRoute,
		testAutoscaling,
		testServiceAccount,
		testServiceMonitoring,
	}

	// Execute the tests in order
	for _, testFunc := range testFuncs {
		if err := testFunc(t, r, rb); err != nil {
			t.Fatalf("%v", err)
		}
	}
}

func testBasicReconcile(t *testing.T, r *ReconcileRuntimeComponent, rb appstacksutils.ReconcilerBase) error {
	// Mock request to simulate Reconcile being called on an event for a watched resource
	// then ensure reconcile is successful and does not return an empty result
	res, err := r.Reconcile(req)
	if err = verifyReconcile(res, err); err != nil {
		return err
	}

	// Check if deployment has been created
	dep := &appsv1.Deployment{}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, dep); err != nil {
		return fmt.Errorf("Get Deployment: (%v)", err)
	}

	depTests := []Test{
		{"Basic Reconcile", "app", dep.Spec.Template.Spec.ServiceAccountName},
	}
	if err = verifyTests(depTests); err != nil {
		return err
	}

	return nil
}

func testStorage(t *testing.T, r *ReconcileRuntimeComponent, rb appstacksutils.ReconcilerBase) error {
	runtimecomponent := makeBasicRuntimeComponent(name, namespace)
	// Update runtimecomponentwith values for StatefulSet
	// Update ServiceAccountName for empty case
	runtimecomponent.Spec = appstacksv1beta1.RuntimeComponentSpec{
		Storage:          &storage,
		Replicas:         &replicas,
		ApplicationImage: appImage,
	}
	updateRuntimeComponent(r, runtimecomponent, t)

	// Reconcile again to check for the StatefulSet and updated resources
	res, err := r.Reconcile(req)
	if err = verifyReconcile(res, err); err != nil {
		return err
	}

	// Check if StatefulSet has been created
	statefulSet := &appsv1.StatefulSet{}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, statefulSet); err != nil {
		t.Fatalf("Get StatefulSet: (%v)", err)
	}

	// Storage is enabled so the deployment should be deleted
	dep := &appsv1.Deployment{}
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
	if err = verifyTests(ssTests); err != nil {
		return err
	}

	return nil
}

func testKnativeService(t *testing.T, r *ReconcileRuntimeComponent, rb appstacksutils.ReconcilerBase) error {
	runtimecomponent := makeBasicRuntimeComponent(name, namespace)
	// Enable CreateKnativeService
	runtimecomponent.Spec = appstacksv1beta1.RuntimeComponentSpec{
		CreateKnativeService: &createKnativeService,
		PullPolicy:           &pullPolicy,
		ApplicationImage:     ksvcAppImage,
	}
	updateRuntimeComponent(r, runtimecomponent, t)

	// Reconcile again to check for the KNativeService and updated resources
	res, err := r.Reconcile(req)
	verifyReconcile(res, err)

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
	statefulset := &appsv1.StatefulSet{}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, statefulset); err == nil {
		t.Fatalf("StatefulSet was not deleted")
	}

	// Check updated values in KnativeService
	ksvcTests := []Test{
		{"service image name", ksvcAppImage, ksvc.Spec.Template.Spec.Containers[0].Image},
		{"pull policy", pullPolicy, ksvc.Spec.Template.Spec.Containers[0].ImagePullPolicy},
		{"service account name", name, ksvc.Spec.Template.Spec.ServiceAccountName},
	}
	if err = verifyTests(ksvcTests); err != nil {
		return err
	}
	return nil
}

func testExposeRoute(t *testing.T, r *ReconcileRuntimeComponent, rb appstacksutils.ReconcilerBase) error {
	runtimecomponent := makeBasicRuntimeComponent(name, namespace)

	expose := true
	runtimecomponent.Spec = appstacksv1beta1.RuntimeComponentSpec{
		Expose: &expose,
	}
	updateRuntimeComponent(r, runtimecomponent, t)

	res, err := r.Reconcile(req)
	if err = verifyReconcile(res, err); err != nil {
		return err
	}

	route := &routev1.Route{
		// TypeMeta: metav1.TypeMeta{APIVersion: "route.openshift.io/v1", Kind: "Route"},
	}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, route); err != nil {
		t.Fatalf("Get Route: (%v)", err)
	}

	// Check updated values in Route
	routeTests := []Test{{"target port", intstr.FromString(strconv.Itoa(int(service.Port)) + "-tcp"), route.Spec.Port.TargetPort}}
	if err = verifyTests(routeTests); err != nil {
		return err
	}
	return nil
}

func testAutoscaling(t *testing.T, r *ReconcileRuntimeComponent, rb appstacksutils.ReconcilerBase) error {
	runtimecomponent := makeBasicRuntimeComponent(name, namespace)
	runtimecomponent.Spec = appstacksv1beta1.RuntimeComponentSpec{
		Autoscaling: autoscaling,
	}
	updateRuntimeComponent(r, runtimecomponent, t)

	// Reconcile again to check for hpa and updated resources
	res, err := r.Reconcile(req)
	if err = verifyReconcile(res, err); err != nil {
		return err
	}

	// Create HorizontalPodAutoscaler
	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, hpa); err != nil {
		return fmt.Errorf("Get HPA: (%v)", err)
	}

	// verify that the route has been deleted now that expose is disabled
	route := &routev1.Route{}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, route); err == nil {
		return fmt.Errorf("Failed to delete Route")
	}

	// Check updated values in hpa
	hpaTests := []Test{{"max replicas", autoscaling.MaxReplicas, hpa.Spec.MaxReplicas}}
	if err = verifyTests(hpaTests); err != nil {
		return err
	}
	return nil
}

func testServiceAccount(t *testing.T, r *ReconcileRuntimeComponent, rb appstacksutils.ReconcilerBase) error {
	runtimecomponent := makeBasicRuntimeComponent(name, namespace)
	updateRuntimeComponent(r, runtimecomponent, t)
	res, err := r.Reconcile(req)
	if err = verifyReconcile(res, err); err != nil {
		return err
	}

	serviceaccount := &corev1.ServiceAccount{ObjectMeta: defaultMeta}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, serviceaccount); err != nil {
		return err
	}
	runtimecomponent.Spec = appstacksv1beta1.RuntimeComponentSpec{
		ServiceAccountName: &serviceAccountName,
	}
	updateRuntimeComponent(r, runtimecomponent, t)

	res, err = r.Reconcile(req)
	if err = verifyReconcile(res, err); err != nil {
		return err
	}

	// Check that the default service account was deleted
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, serviceaccount); err == nil {
		return err
	}
	return nil
}

func testServiceMonitoring(t *testing.T, r *ReconcileRuntimeComponent, rb appstacksutils.ReconcilerBase) error {
	runtimecomponent := makeBasicRuntimeComponent(name, namespace)

	// Test with monitoring specified
	runtimecomponent.Spec.Monitoring = &appstacksv1beta1.RuntimeComponentMonitoring{}
	updateRuntimeComponent(r, runtimecomponent, t)
	res, err := r.Reconcile(req)
	if err = verifyReconcile(res, err); err != nil {
		return err
	}

	svc := &corev1.Service{ObjectMeta: defaultMeta}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, svc); err != nil {
		return err
	}

	monitorTests := []Test{
		{"Monitor label assigned", "true", svc.Labels["monitor."+runtimecomponent.GetGroupName()+"/enabled"]},
	}
	if err = verifyTests(monitorTests); err != nil {
		return err
	}

	// Test without monitoring on
	runtimecomponent.Spec.Monitoring = nil
	updateRuntimeComponent(r, runtimecomponent, t)
	res, err = r.Reconcile(req)
	if err = verifyReconcile(res, err); err != nil {
		return err
	}

	svc = &corev1.Service{ObjectMeta: defaultMeta}
	if err = r.GetClient().Get(context.TODO(), req.NamespacedName, svc); err != nil {
		return err
	}

	monitorTests = []Test{
		{"Monitor label unassigned", "", svc.Labels["app."+runtimecomponent.GetGroupName()+"/monitor"]},
	}
	if err = verifyTests(monitorTests); err != nil {
		return err
	}

	return nil
}

func addResourcesToScheme(t *testing.T, s *runtime.Scheme, runtimecomponent *appstacksv1beta1.RuntimeComponent) {
	if err := servingv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add servingv1alpha1 scheme: (%v)", err)
	}

	if err := routev1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add route scheme: (%v)", err)
	}

	if err := imagev1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add image scheme: (%v)", err)
	}

	if err := applicationsv1beta1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add image scheme: (%v)", err)
	}

	if err := certmngrv1alpha2.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add cert-manager scheme: (%v)", err)
	}

	if err := prometheusv1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add prometheus scheme: (%v)", err)
	}

	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimecomponent)
	s.AddKnownTypes(certmngrv1alpha2.SchemeGroupVersion, &certmngrv1alpha2.Certificate{})
	s.AddKnownTypes(prometheusv1.SchemeGroupVersion, &prometheusv1.ServiceMonitor{})
}

// Helper Functions
func makeBasicRuntimeComponent(n, ns string) *appstacksv1beta1.RuntimeComponent {
	spec := appstacksv1beta1.RuntimeComponentSpec{}
	runtime := &appstacksv1beta1.RuntimeComponent{
		ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns},
		Spec:       spec,
	}
	return runtime
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
		{
			GroupVersion: applicationsv1beta1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "applications", Namespaced: true, Kind: "Application", SingularName: "application"},
			},
		},
	}

	return fakeDiscoveryClient
}

func createConfigMap(n, ns string, data map[string]string) *corev1.ConfigMap {
	app := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns},
		Data:       data,
	}
	return app
}

func verifyReconcile(res reconcile.Result, err error) error {
	if err != nil {
		return fmt.Errorf("reconcile: (%v)", err)
	}

	if res != (reconcile.Result{}) {
		return fmt.Errorf("reconcile did not return an empty result (%v)", res)
	}

	return nil
}

func verifyTests(tests []Test) error {
	for _, tt := range tests {
		if tt.actual != tt.expected {
			return fmt.Errorf("%s test expected: (%v) actual: (%v)", tt.test, tt.expected, tt.actual)
		}
	}
	return nil
}

func updateRuntimeComponent(r *ReconcileRuntimeComponent, runtimecomponent *appstacksv1beta1.RuntimeComponent, t *testing.T) {
	if err := r.GetClient().Update(context.TODO(), runtimecomponent); err != nil {
		t.Fatalf("Update runtimecomponent: (%v)", err)
	}
}
