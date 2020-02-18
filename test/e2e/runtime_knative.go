package e2e

import (
	goctx "context"
	"testing"
	"time"

	"github.com/application-runtimes/operator/test/util"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	e2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	corev1 "k8s.io/api/core/v1"
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

	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "application-runtime-operator", 1, retryInterval, operatorTimeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
	knativeBool := true
	applicationName := "example-runtime-knative"

	exampleRuntime := util.MakeBasicRuntimeApplication(t, f, applicationName, namespace, 1)
	exampleRuntime.Spec.CreateKnativeService = &knativeBool

	// Create application deployment and wait
	err = f.Client.Create(goctx.TODO(), exampleRuntime, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second, RetryInterval: time.Second})
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	err = util.WaitForKnativeDeployment(t, f, namespace, applicationName, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
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
