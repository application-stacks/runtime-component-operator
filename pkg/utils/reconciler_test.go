package utils

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/application-stacks/operator/pkg/common"

	appstacksv1beta1 "github.com/application-stacks/operator/pkg/apis/appstacks/v1beta1"
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
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	defaultMeta = metav1.ObjectMeta{
		Name:      "app",
		Namespace: "runtimeapp",
	}
	spec = appstacksv1beta1.RuntimeApplicationSpec{}
)

func TestGetDiscoveryClient(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	runtimeapp := createRuntimeApp(name, namespace, spec)
	objs, s := []runtime.Object{runtimeapp}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimeapp)
	cl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	newDC, err := r.GetDiscoveryClient()

	if newDC == nil {
		t.Fatalf("GetDiscoverClient did not create a new discovery client. newDC: (%v) err: (%v)", newDC, err)
	}
}

func TestCreateOrUpdate(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: defaultMeta}

	runtimeapp := createRuntimeApp(name, namespace, spec)
	objs, s := []runtime.Object{runtimeapp}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimeapp)
	cl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	err := r.CreateOrUpdate(serviceAccount, runtimeapp, func() error {
		CustomizeServiceAccount(serviceAccount, runtimeapp)
		return nil
	})

	testCOU := []Test{{"CreateOrUpdate error is nil", nil, err}}
	verifyTests(testCOU, t)
}

func TestDeleteResources(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	runtimeapp := createRuntimeApp(name, namespace, spec)
	objs, s := []runtime.Object{runtimeapp}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimeapp)
	cl := fakeclient.NewFakeClient(objs...)
	r := NewReconcilerBase(cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	r.SetDiscoveryClient(createFakeDiscoveryClient())
	nsn := types.NamespacedName{Name: "app", Namespace: "runtimeapp"}

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

	runtimeapp := createRuntimeApp(name, namespace, spec)
	objs, s := []runtime.Object{runtimeapp}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimeapp)
	cl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(cl, s, &rest.Config{}, record.NewFakeRecorder(10))

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

	runtimeapp := createRuntimeApp(name, namespace, spec)
	objs, s := []runtime.Object{runtimeapp}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimeapp)
	cl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	rec, err := r.ManageError(err, common.StatusConditionTypeReconciled, runtimeapp)

	testME := []Test{
		{"ManageError Requeue", true, rec.Requeue},
		{"ManageError New Condition Status", corev1.ConditionFalse, runtimeapp.Status.Conditions[0].Status},
	}
	verifyTests(testME, t)
}

func TestManageSuccess(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	runtimeapp := createRuntimeApp(name, namespace, spec)
	objs, s := []runtime.Object{runtimeapp}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimeapp)
	cl := fakeclient.NewFakeClient(objs...)
	r := NewReconcilerBase(cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimeapp)

	testMS := []Test{
		{"ManageSuccess New Condition Status", corev1.ConditionTrue, runtimeapp.Status.Conditions[0].Status},
	}
	verifyTests(testMS, t)
}

func TestIsGroupVersionSupported(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	runtimeapp := createRuntimeApp(name, namespace, spec)
	objs, s := []runtime.Object{runtimeapp}, scheme.Scheme
	s.AddKnownTypes(appstacksv1beta1.SchemeGroupVersion, runtimeapp)
	cl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(cl, s, &rest.Config{}, record.NewFakeRecorder(10))
	fakeDiscoveryClient := &fakediscovery.FakeDiscovery{
		Fake: &coretesting.Fake{Resources: []*metav1.APIResourceList{{GroupVersion: "v1"}}},
	}
	r.SetDiscoveryClient(fakeDiscoveryClient)

	_, err := r.IsGroupVersionSupported("v1")
	if err != nil {
		t.Fatalf("Group version should be supported: (%v)", err)
	}

	_, err = r.IsGroupVersionSupported("v2")
	if err == nil {
		t.Fatalf("Group version should not be supported")
	}
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
