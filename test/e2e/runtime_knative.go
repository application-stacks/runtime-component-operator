package e2e

import (
	goctx "context"
	"errors"
	"testing"
	"time"

	"github.com/application-stacks/runtime-component-operator/test/util"
	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	e2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// RuntimeKnativeTest : Create application with knative service enabled to verify feature
func RuntimeKnativeTest(t *testing.T) {
	ctx, err := util.InitializeContext(t, cleanupTimeout, retryInterval)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Cleanup()
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Couldn't get namespace: %v", err)
	}

	t.Logf("Namespace: %s", namespace)

	f := framework.Global

	// catch cases where running tests locally with a cluster that does not have knative
	if !isKnativeInstalled(t, f) {
		t.Log("Knative is not installed on this cluster, skipping RuntimeKnativeTest...")
		return
	}

	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "runtime-component-operator", 1, retryInterval, operatorTimeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	testKnIsFalse(t, f, ctx, namespace)
	testKnIsTrueAndTurnOff(t, f, ctx, namespace)

}

func isKnativeInstalled(t *testing.T, f *framework.Framework) bool {
	ns := &corev1.NamespaceList{}
	err := f.Client.List(goctx.TODO(), ns)
	if err != nil {
		t.Fatalf("Error occurred while trying to find knative-serving %v", err)
	}
	for _, val := range ns.Items {
		if val.Name == "knative-serving" {
			return true
		}
	}
	return false
}

func testKnIsFalse(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, namespace string) {
	knativeBool := false
	applicationName := "example-runtime-knative-0"
	exampleRuntime := util.MakeBasicRuntimeComponent(t, f, applicationName, namespace, 1)
	exampleRuntime.Spec.CreateKnativeService = &knativeBool

	// Create application deployment and wait
	err := f.Client.Create(goctx.TODO(), exampleRuntime, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second, RetryInterval: time.Second})
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	// Wait for deployment.
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, applicationName, 1, retryInterval, operatorTimeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	// Any other checks?
	isDeployed, err := util.IsKnativeServiceDeployed(t, f, namespace, applicationName)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
	if isDeployed {
		util.FailureCleanup(t, f, namespace, errors.New("knative service is deployed when CreateKnativeService is set to false"))
	}
}

func testKnIsTrueAndTurnOff(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, namespace string) {
	knativeBool := true
	applicationName := "example-runtime-knative-1"
	exampleRuntime := util.MakeBasicRuntimeComponent(t, f, applicationName, namespace, 1)
	exampleRuntime.Spec.CreateKnativeService = &knativeBool

	// Create application deployment and wait
	err := f.Client.Create(goctx.TODO(), exampleRuntime, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second, RetryInterval: time.Second})
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	err = util.WaitForKnativeDeployment(t, f, namespace, applicationName, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	dep := &appsv1.Deployment{}
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: applicationName, Namespace: namespace}, dep)
	if err != nil {
		if apierrors.IsNotFound(err) {
			t.Logf("Deployment successfully cleared when Knative is enabled.")
		} else {
			util.FailureCleanup(t, f, namespace, err)
		}
	}

	// Turn the runtime component off / set CreateKnativeService to false.
	target := types.NamespacedName{Name: applicationName, Namespace: namespace}
	err = util.UpdateApplication(f, target, func(r *appstacksv1beta1.RuntimeComponent) {
		knativeBool = false
		r.Spec.CreateKnativeService = &knativeBool
	})
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
	
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, applicationName, 1, retryInterval, operatorTimeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
	dep = &appsv1.Deployment{}
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: applicationName, Namespace: namespace}, dep)
	if err != nil {
		t.Log(err)
	}
	t.Log(dep)
}
