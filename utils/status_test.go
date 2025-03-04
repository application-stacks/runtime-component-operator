package utils

import (
	"testing"

	appstacksv1 "github.com/application-stacks/runtime-component-operator/api/v1"
	"github.com/application-stacks/runtime-component-operator/common"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

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
	notReconciledReason := newCondition.GetReason()

	// Report successful reconciliation
	// Overall application status should return NotCreated with no resource created.
	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimecomponent)
	_, newCondition = r.CheckApplicationStatus(runtimecomponent)
	notCreatedReason := newCondition.GetReason()

	objMeta := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: objMeta,
		Status: appsv1.DeploymentStatus{
			Replicas:        3,
			ReadyReplicas:   2,
			UpdatedReplicas: 2,
		},
	}
	r.CreateOrUpdate(deploy, runtimecomponent, func() error {
		CustomizeDeployment(deploy, runtimecomponent)
		return nil
	})

	_, newCondition = r.CheckApplicationStatus(runtimecomponent)
	replicasUnavailable := newCondition.GetReason()

	testAS := []Test{
		{test: "placeholder", expected: "placeholder", actual: "placeholder"},
		{test: "Not reconciled reason", expected: "ApplicationNotReconciled", actual: notReconciledReason},
		{test: "Deployment not created reason", expected: "NotCreated", actual: notCreatedReason},
		{test: "Deployment not created reason", expected: "MinimumReplicasUnavailable", actual: replicasUnavailable},
	}

	verifyTests(testAS, t)

	// runtimecomponent.GetStatus().SetCondition(newCondition)
	// runtimecomponent.Status = appstacksv1.RuntimeComponentStatus{
	// 	Conditions: ,
	// }
	// log1 := logger.WithValues("newCondition", newCondition, "runtimecomponent", runtimecomponent)
	// log1.Info("HHHEEERRREE")

}
