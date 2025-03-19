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

var (
	// Status test variables
	st_replicas int32 = 3
)

func TestCheckApplicationStatus(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	// Setup fake client and reconciler base
	spec := appstacksv1.RuntimeComponentSpec{Replicas: &st_replicas}
	r, runtimecomponent := setupFakeClientWithRC(spec)

	// Overall application status should report ApplicationNotReconciled with no successful reconciliation
	_, newCondition := r.CheckApplicationStatus(runtimecomponent)
	notReconciled := newCondition.GetReason()

	// ResourcesReady condition should report MinimumReplicasUnavailable with less number of ready replicas
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:        3,
			ReadyReplicas:   3,
			UpdatedReplicas: 3,
		},
	}
	r.CreateOrUpdate(deploy, runtimecomponent, func() error {
		CustomizeDeployment(deploy, runtimecomponent)
		return nil
	})

	// Run successful reconciliation and check ready application status
	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimecomponent)
	_, newCondition = r.CheckApplicationStatus(runtimecomponent)
	reconciledMessage := newCondition.GetMessage()

	testAS := []Test{
		{test: "Not reconciled", expected: "ApplicationNotReconciled", actual: notReconciled},
		{test: "Reconciled and ready", expected: "Application is reconciled and resources are ready.", actual: reconciledMessage},
	}

	verifyTests(testAS, t)
}

// Test areReplicasReady for Deployment resource status check
func TestDeploymentReplicasReady(t *testing.T) {

	// Setup fake client and reconciler base
	// Set RuntimeComponent to use Deployment
	spec = appstacksv1.RuntimeComponentSpec{Replicas: &st_replicas}
	r, runtimecomponent := setupFakeClientWithRC(spec)

	// Report successful reconciliation to check for ResourcesReady condition
	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimecomponent)
	conditionType := common.StatusConditionTypeResourcesReady
	newCondition := runtimecomponent.GetStatus().NewCondition(conditionType)

	// ResourcesReady condition should report NotCreated with no Deployment created
	resourceCondition := r.areReplicasReady(runtimecomponent, newCondition)
	dpNotCreated := resourceCondition.GetReason()

	// Create Deployment with less ready replicas than expected
	// ResourcesReady condition should report MinimumReplicasUnavailable
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
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

	// Update Deployment with all more replicas than expected
	// ResourcesReady condition should report ReplicaSetUpdating
	deploy.Status = appsv1.DeploymentStatus{
		Replicas:        4,
		ReadyReplicas:   3,
		UpdatedReplicas: 3,
	}
	r.GetClient().Status().Update(context.Background(), deploy)

	resourceCondition = r.areReplicasReady(runtimecomponent, newCondition)
	dpReplicaSetUpdating := resourceCondition.GetReason()

	// Update Deployment with all ready replicas
	// ResourcesReady condition should report MinimumReplicasAvailable
	deploy.Status = appsv1.DeploymentStatus{
		Replicas:        3,
		ReadyReplicas:   3,
		UpdatedReplicas: 3,
	}
	r.GetClient().Status().Update(context.Background(), deploy)

	resourceCondition = r.areReplicasReady(runtimecomponent, newCondition)
	dpReady := resourceCondition.GetReason()

	// Test Deployment resource status
	testDR := []Test{
		{test: "Deployment not created", expected: "NotCreated", actual: dpNotCreated},
		{test: "Deployment replicas unavailable", expected: "MinimumReplicasUnavailable", actual: dpReplicasUnavailable},
		{test: "Deployment ReplicaSet updating", expected: "ReplicaSetUpdating", actual: dpReplicaSetUpdating},
		{test: "Deployment ready", expected: "MinimumReplicasAvailable", actual: dpReady},
	}

	verifyTests(testDR, t)
}

// Test areReplicasReady for StatefulSet resource status check
func TestStatefulSetReplicasReady(t *testing.T) {
	// Setup fake client and reconciler base
	// Set RuntimeComponent to use StatefulSet
	spec = appstacksv1.RuntimeComponentSpec{Replicas: &st_replicas, StatefulSet: &appstacksv1.RuntimeComponentStatefulSet{}}
	r, runtimecomponent := setupFakeClientWithRC(spec)

	// Report successful reconciliation to check for ResourcesReady condition
	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimecomponent)
	conditionType := common.StatusConditionTypeResourcesReady
	newCondition := runtimecomponent.GetStatus().NewCondition(conditionType)

	// ResourcesReady condition should report NotCreated with no StatefulSet created
	resourceCondition := r.areReplicasReady(runtimecomponent, newCondition)
	ssNotCreated := resourceCondition.GetReason()

	// Create StatefulSet with less ready replicas than expected
	// ResourcesReady condition should report MinimumReplicasUnavailable
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
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

	// Update StatefulSet with all ready replicas
	// ResourcesReady condition should report MinimumReplicasAvailable with all ready replicas
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

	// Test StatefulSet resource status
	testSR := []Test{
		{test: "StatefulSet not created", expected: "NotCreated", actual: ssNotCreated},
		{test: "StatefulSet replicas unavailable", expected: "MinimumReplicasUnavailable", actual: ssReplicasUnavailable},
		{test: "StatefulSet ready", expected: "MinimumReplicasAvailable", actual: ssReady},
	}

	verifyTests(testSR, t)
}

// Test isKnativeReady for Knative resource status check
func TestKnativeReady(t *testing.T) {

	// Setup fake client and reconciler base
	// Set RuntimeComponent to use KnativeService
	createKnativeService := true
	spec = appstacksv1.RuntimeComponentSpec{Replicas: &st_replicas, CreateKnativeService: &createKnativeService}
	r, runtimecomponent := setupFakeClientWithRC(spec)

	// Report successful reconciliation to check for ResourcesReady condition
	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimecomponent)
	conditionType := common.StatusConditionTypeResourcesReady
	newCondition := runtimecomponent.GetStatus().NewCondition(conditionType)

	// ResourcesReady condition should report ServiceNotCreated with no KnativeService created
	resourceCondition := r.isKnativeReady(runtimecomponent, newCondition)
	ksNotCreated := resourceCondition.GetReason()

	// Create KnativeService and add the type to the fake client
	// ResourcesReady condition should report ServiceStatusNotFound with empty KnativeService status
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

	// Unable to update status for KnativeService through fake client because it is not a known type
	// Deleting and re-creating KnativeService to update its status conditions
	// Update KnativeService with Not Ready condition
	// ResourcesReady condition should report ServiceNotReady
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

	r.CreateOrUpdate(knative2, runtimecomponent, func() error {
		return nil
	})

	resourceCondition = r.isKnativeReady(runtimecomponent, newCondition)
	ksServiceNotReady := resourceCondition.GetReason()

	// Update KnativeService with Ready condition
	// There is no reason field reported - using message instead for this case
	// ResourcesReady condition should report "Knative service is ready." message
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

	r.CreateOrUpdate(knative3, runtimecomponent, func() error {
		return nil
	})

	resourceCondition = r.isKnativeReady(runtimecomponent, newCondition)
	ksServiceReadyMsg := resourceCondition.GetMessage()

	// Test KnatvieService resource status
	testKS := []Test{
		{test: "Knative not created", expected: "ServiceNotCreated", actual: ksNotCreated},
		{test: "Knative Status not created", expected: "ServiceStatusNotFound", actual: ksStatusNotCreated},
		{test: "Knative Status not ready", expected: "ServiceNotReady", actual: ksServiceNotReady},
		{test: "Knative Status ready", expected: "Knative service is ready.", actual: ksServiceReadyMsg},
	}

	verifyTests(testKS, t)
}

// Setup fake client with RuntimeComponent kind
// Creates a RuntimeComponent object with input spec
func setupFakeClientWithRC(spec appstacksv1.RuntimeComponentSpec) (ReconcilerBase, *appstacksv1.RuntimeComponent) {
	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)
	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	return r, runtimecomponent
}
