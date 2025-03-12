package utils

import (
	"context"
	"testing"

	appstacksv1 "github.com/application-stacks/runtime-component-operator/api/v1"
	"github.com/application-stacks/runtime-component-operator/common"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	"knative.dev/pkg/apis"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestCheckApplicationStatus(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	// Setup fake client and reconciler base
	replicas := int32(3)
	spec = appstacksv1.RuntimeComponentSpec{Replicas: &replicas}
	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	// Overall application status should return ApplicationNotReconciled
	_, newCondition := r.CheckApplicationStatus(runtimecomponent)
	notReconciled := newCondition.GetReason()

	// Test Deployment resource status check
	dpNotCreated, dpReplicasUnavailable, dpReady := testDeploymentReplicasReady(r, runtimecomponent)

	// Test StatefulSet resource status check
	ssNotCreated, ssReplicasUnavailable, ssReady := testStatefulSetReplicasReady(r, runtimecomponent)

	// Test Knative resource status check
	ksNotCreated, ksStatusNotCreated, ksServiceNotReady, ksServiceReadyMsg := testKnativeReady(r, runtimecomponent)

	testAS := []Test{
		{test: "Not reconciled", expected: "ApplicationNotReconciled", actual: notReconciled},
		{test: "Deployment not created", expected: "NotCreated", actual: dpNotCreated},
		{test: "Deployment replicas unavailable", expected: "MinimumReplicasUnavailable", actual: dpReplicasUnavailable},
		{test: "Deployment ready", expected: "MinimumReplicasAvailable", actual: dpReady},
		{test: "StatefulSet not created", expected: "NotCreated", actual: ssNotCreated},
		{test: "StatefulSet replicas unavailable", expected: "MinimumReplicasUnavailable", actual: ssReplicasUnavailable},
		{test: "StatefulSet ready", expected: "MinimumReplicasAvailable", actual: ssReady},
		{test: "Knative not created", expected: "ServiceNotCreated", actual: ksNotCreated},
		{test: "Knative Status not created", expected: "ServiceStatusNotFound", actual: ksStatusNotCreated},
		{test: "Knative Status not ready", expected: "ServiceNotReady", actual: ksServiceNotReady},
		{test: "Knative Status ready", expected: "Knative service is ready.", actual: ksServiceReadyMsg},
	}

	verifyTests(testAS, t)
}

// Partial test for areReplicasReady
func testDeploymentReplicasReady(r ReconcilerBase, runtimecomponent *appstacksv1.RuntimeComponent) (string, string, string) {

	// Report successful reconciliation
	// Overall application status should return NotCreated with no resource created.
	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimecomponent)
	conditionType := common.StatusConditionTypeResourcesReady
	newCondition := runtimecomponent.GetStatus().NewCondition(conditionType)

	resourceCondition := r.areReplicasReady(runtimecomponent, newCondition)
	dpNotCreated := resourceCondition.GetReason()

	objMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: objMeta,
		Status: appsv1.DeploymentStatus{
			Replicas:        3,
			ReadyReplicas:   1,
			UpdatedReplicas: 1,
		},
	}

	r.CreateOrUpdate(deploy, runtimecomponent, func() error {
		CustomizeDeployment(deploy, runtimecomponent)
		return nil
	})

	resourceCondition = r.areReplicasReady(runtimecomponent, newCondition)
	dpReplicasUnavailable := resourceCondition.GetReason()

	deploy.Status = appsv1.DeploymentStatus{
		Replicas:        3,
		ReadyReplicas:   3,
		UpdatedReplicas: 3,
	}

	r.GetClient().Status().Update(context.Background(), deploy)

	resourceCondition = r.areReplicasReady(runtimecomponent, newCondition)
	dpReady := resourceCondition.GetReason()

	return dpNotCreated, dpReplicasUnavailable, dpReady
}

// Partial test for areReplicasReady
func testStatefulSetReplicasReady(r ReconcilerBase, runtimecomponent *appstacksv1.RuntimeComponent) (string, string, string) {
	runtimecomponent.Spec.StatefulSet = &appstacksv1.RuntimeComponentStatefulSet{}

	// Report successful reconciliation
	// Overall application status should return NotCreated with no resource created.
	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimecomponent)
	conditionType := common.StatusConditionTypeResourcesReady
	newCondition := runtimecomponent.GetStatus().NewCondition(conditionType)

	resourceCondition := r.areReplicasReady(runtimecomponent, newCondition)
	ssNotCreated := resourceCondition.GetReason()

	objMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Status: appsv1.StatefulSetStatus{
			Replicas:        3,
			ReadyReplicas:   1,
			UpdatedReplicas: 1,
		},
	}

	r.CreateOrUpdate(statefulset, runtimecomponent, func() error {
		CustomizeStatefulSet(statefulset, runtimecomponent)
		return nil
	})

	resourceCondition = r.areReplicasReady(runtimecomponent, newCondition)
	ssReplicasUnavailable := resourceCondition.GetReason()

	statefulset.Status = appsv1.StatefulSetStatus{
		Replicas:        3,
		ReadyReplicas:   3,
		UpdatedReplicas: 3,
	}
	r.GetClient().Status().Update(context.Background(), statefulset)

	resourceCondition = r.areReplicasReady(runtimecomponent, newCondition)
	ssReady := resourceCondition.GetReason()

	return ssNotCreated, ssReplicasUnavailable, ssReady
}

// Partial test for isKnativeReady
func testKnativeReady(r ReconcilerBase, runtimecomponent *appstacksv1.RuntimeComponent) (string, string, string, string) {
	createKnativeService := true
	runtimecomponent.Spec.CreateKnativeService = &createKnativeService

	// Report successful reconciliation
	// Overall application status should return NotCreated with no resource created.
	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimecomponent)
	conditionType := common.StatusConditionTypeResourcesReady
	newCondition := runtimecomponent.GetStatus().NewCondition(conditionType)

	resourceCondition := r.isKnativeReady(runtimecomponent, newCondition)
	ksNotCreated := resourceCondition.GetReason()

	knative1 := &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	r.scheme.AddKnownTypes(schema.GroupVersion{Group: "serving.knative.dev", Version: "v1"}, knative1)
	r.CreateOrUpdate(knative1, runtimecomponent, func() error {
		return nil
	})
	resourceCondition = r.isKnativeReady(runtimecomponent, newCondition)
	ksStatusNotCreated := resourceCondition.GetReason()

	// Unable to update status for Knative through fake client
	// Deleting and re-creating Knative service with status conditions
	r.DeleteResource(knative1)

	knative2 := &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	condition := apis.Condition{
		Type:   apis.ConditionReady,
		Status: v1.ConditionFalse,
	}
	conditions := apis.Conditions{condition}
	knative2.Status.SetConditions(conditions)

	r.scheme.AddKnownTypes(schema.GroupVersion{Group: "serving.knative.dev", Version: "v1"}, knative2)
	r.CreateOrUpdate(knative2, runtimecomponent, func() error {
		return nil
	})

	resourceCondition = r.isKnativeReady(runtimecomponent, newCondition)
	ksServiceNotReady := resourceCondition.GetReason()

	r.DeleteResource(knative2)

	knative3 := &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	condition = apis.Condition{
		Type:   apis.ConditionReady,
		Status: v1.ConditionTrue,
	}
	conditions = apis.Conditions{condition}
	knative3.Status.SetConditions(conditions)

	r.scheme.AddKnownTypes(schema.GroupVersion{Group: "serving.knative.dev", Version: "v1"}, knative3)
	r.CreateOrUpdate(knative3, runtimecomponent, func() error {
		return nil
	})

	resourceCondition = r.isKnativeReady(runtimecomponent, newCondition)
	ksServiceReadyMsg := resourceCondition.GetMessage()

	return ksNotCreated, ksStatusNotCreated, ksServiceNotReady, ksServiceReadyMsg
}
