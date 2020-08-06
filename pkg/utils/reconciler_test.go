package utils

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/application-stacks/runtime-component-operator/pkg/common"

	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	coretesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	applicationsv1beta1 "sigs.k8s.io/application/pkg/apis/app/v1beta1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	defaultMeta = metav1.ObjectMeta{
		Name:      "app",
		Namespace: "runtimecomponent",
	}
	spec = appstacksv1beta1.RuntimeComponentSpec{}
)

const (
	tlsCrt    = "faketlscrt"
	tlsKey    = "faketlskey"
	caCrt     = "fakecacrt"
	destCACrt = "fakedestcacrt"
)

func TestGetDiscoveryClient(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	newDC, err := r.GetDiscoveryClient()

	if newDC == nil {
		t.Fatalf("GetDiscoverClient did not create a new discovery client. newDC: (%v) err: (%v)", newDC, err)
	}
}

func TestCreateOrUpdate(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: defaultMeta}

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	err := r.CreateOrUpdate(serviceAccount, runtimecomponent, func() error {
		CustomizeServiceAccount(serviceAccount, runtimecomponent)
		return nil
	})

	testCOU := []Test{{"CreateOrUpdate error is nil", nil, err}}
	verifyTests(testCOU, t)
}

func TestDeleteResources(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)
	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	r.SetDiscoveryClient(createFakeDiscoveryClient())
	nsn := types.NamespacedName{Name: "app", Namespace: "runtimecomponent"}

	sa := &corev1.ServiceAccount{ObjectMeta: defaultMeta}
	ss := &appsv1.StatefulSet{ObjectMeta: defaultMeta}
	roList := []runtime.Object{sa, ss}

	if err := r.GetClient().Create(context.TODO(), sa); err != nil {
		t.Fatalf("Create ServiceAccount: (%v)", err)
	}

	if err := r.GetClient().Get(context.TODO(), nsn, sa); err != nil {
		t.Fatalf("Get ServiceAccount (%v)", err)
	}

	if err := r.GetClient().Create(context.TODO(), ss); err != nil {
		t.Fatalf("Create StatefulSet: (%v)", err)
	}

	if err := r.GetClient().Get(context.TODO(), nsn, ss); err != nil {
		t.Fatalf("Get StatefulSet (%v)", err)
	}

	// Delete Resources
	r.DeleteResources(roList)

	if err := r.GetClient().Get(context.TODO(), nsn, sa); err == nil {
		t.Fatalf("ServiceAccount was not deleted")
	}

	if err := r.GetClient().Get(context.TODO(), nsn, ss); err == nil {
		t.Fatalf("StatefulSet was not deleted")
	}
}

func TestGetOpConfigMap(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data: map[string]string{
			stack: `{"expose":true, "service":{"port": 3000,"type": "ClusterIP"}}`,
		},
	}

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	if err := r.GetClient().Create(context.TODO(), configMap); err != nil {
		t.Fatalf("Create configMap: (%v)", err)
	}

	cm, err := r.GetOpConfigMap(name, namespace)

	testGAOCM := []Test{
		{"GetOpConfigMap error is nil", nil, err},
		{"GetOpConfigMap ConfigMap is correct", true, reflect.DeepEqual(cm.Data, configMap.Data)},
	}
	verifyTests(testGAOCM, t)
}

func TestManageError(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	err := fmt.Errorf("test-error")

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	rec, err := r.ManageError(err, common.StatusConditionTypeReconciled, runtimecomponent)

	testME := []Test{
		{"ManageError Requeue", true, rec.Requeue},
		{"ManageError New Condition Status", corev1.ConditionFalse, runtimecomponent.Status.Conditions[0].Status},
	}
	verifyTests(testME, t)
}

func TestManageSuccess(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)
	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimecomponent)

	testMS := []Test{
		{"ManageSuccess New Condition Status", corev1.ConditionTrue, runtimecomponent.Status.Conditions[0].Status},
	}
	verifyTests(testMS, t)
}

func TestIsGroupVersionSupported(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))
	fakeDiscoveryClient := &fakediscovery.FakeDiscovery{
		Fake: &coretesting.Fake{Resources: []*metav1.APIResourceList{
			{
				GroupVersion: "abc/v1",
				APIResources: []metav1.APIResource{
					{Kind: "Test", Name: "tests", Group: "abc", Version: "v1"},
				}}}},
	}
	r.SetDiscoveryClient(fakeDiscoveryClient)

	ok, err := r.IsGroupVersionSupported("abc/v1", "Test")
	if err != nil && ok {
		t.Fatalf("Group version should be supported: (%v)", err)
	}

	ok, err = r.IsGroupVersionSupported("abc/v1", "Abc")
	if err == nil && ok {
		t.Fatalf("Group version should not be supported")
	}

	ok, err = r.IsGroupVersionSupported("abc/v2", "Test")
	if err == nil && ok {
		t.Fatalf("Group version should not be supported")
	}
}

// testGetSvcTLSValues test part of the function GetRouteTLSValues in reconciler.go.
func testGetSvcTLSValues(t *testing.T) {
	// Configure the runtime component
	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	expose := true
	runtimecomponent.Spec.Expose = &expose
	runtimecomponent.Spec.Service = &appstacksv1beta1.RuntimeComponentService{
		Certificate: &appstacksv1beta1.Certificate{},
		Port:        3000,
	}

	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimecomponent)

	// Deploy the expected secret
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)
	secret := makeCertSecret("my-app-svc-tls", namespace)
	if err := cl.Create(context.TODO(), secret); err != nil {
		t.Fatal(err)
	}

	// Use the reconciler to retrieve the secret
	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))
	key, cert, ca, destCa, err := r.GetRouteTLSValues(runtimecomponent)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the results where only destCa should be retrieved.
	testSTV := []Test{
		{"Svc TLS Value - Key", "", key},
		{"Svc TLS Value - Cert", "", cert},
		{"Svc TLS Value - CA Cert", "", ca},
		{"Svc TLS Value - Dest CA Cert", caCrt, destCa},
	}
	verifyTests(testSTV, t)
}

// testGetRouteTLSValues test the function GetRouteTLSValues in reconciler.go.
func testGetRouteTLSValues(t *testing.T) {
	// Configure the rumtime component
	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	terminationPolicy := routev1.TLSTerminationReencrypt
	secretRefName := "my-app-route-tls"
	runtimecomponent.Spec.Expose = &expose
	runtimecomponent.Spec.Route = &appstacksv1beta1.RuntimeComponentRoute{
		Host:                 "myapp.mycompany.com",
		Termination:          &terminationPolicy,
		CertificateSecretRef: &secretRefName,
	}
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimecomponent)

	// Create a fake client and a reconciler
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)
	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	// Make and deploy the secret for later retrieval
	secret := makeCertSecret(secretRefName, namespace)
	if err := cl.Create(context.TODO(), secret); err != nil {
		t.Fatal(err)
	}

	// Use the reconciler to retrieve the secret
	key, cert, ca, destCa, err := r.GetRouteTLSValues(runtimecomponent)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the result
	testRTV := []Test{
		{"Route TLS Value - Key", key, tlsKey},
		{"Route TLS Value - Cert", cert, tlsCrt},
		{"Route TLS Value - CA Cert", ca, caCrt},
		{"Route TLS Value - Dest CA Cert", destCa, destCACrt},
	}
	verifyTests(testRTV, t)
}

func TestGetRouteTLSValues(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	// Test two scenarios: retrieving secret from service and retrieving secret from route
	testGetSvcTLSValues(t)
	testGetRouteTLSValues(t)
}

func TestGetSelectorLabelsFromApplications(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	// Setup scheme
	s := scheme.Scheme
	s.AddKnownTypes(applicationsv1beta1.SchemeGroupVersion, &applicationsv1beta1.Application{})
	s.AddKnownTypes(applicationsv1beta1.SchemeGroupVersion, &applicationsv1beta1.ApplicationList{})

	// Configure the application
	labelMap := map[string]string{
		"test-key": "test-value",
	}
	application := &applicationsv1beta1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"kappnav.component.namespaces": namespace,
			},
		},
		Spec: applicationsv1beta1.ApplicationSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelMap,
			},
		},
	}

	// Create the fake client and the reconciler
	cl := fakeclient.NewFakeClient()
	rcl := fakeclient.NewFakeClient()
	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	// Deploy the application
	err := cl.Create(context.TODO(), application)
	if err != nil {
		t.Fatal(err)
	}

	// Get selector labels
	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	returnedLabelMap, err := r.GetSelectorLabelsFromApplications(runtimecomponent)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the result
	testMS := []Test{
		{"SelectorLabels", labelMap, returnedLabelMap},
	}
	verifyTests(testMS, t)
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
	}

	return fakeDiscoveryClient
}

// makeCertSecret returns a pointer to a simple Secret object with fake values inside.
func makeCertSecret(n string, ns string) *corev1.Secret {
	data := map[string][]byte{
		"ca.crt":     []byte(caCrt),
		"tls.crt":    []byte(tlsCrt),
		"tls.key":    []byte(tlsKey),
		"destCA.crt": []byte(destCACrt),
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n,
			Namespace: ns,
		},
		Type: "kubernetes.io/tls",
		Data: data,
	}
	return &secret
}
