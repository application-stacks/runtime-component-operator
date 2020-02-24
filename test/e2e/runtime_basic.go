package e2e

import (
	goctx "context"
	"fmt"
	"testing"
	"time"

	runtimeappv1beta1 "github.com/application-runtimes/operator/pkg/apis/runtimeapp/v1beta1"

	"github.com/application-runtimes/operator/test/util"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	e2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"k8s.io/apimachinery/pkg/types"
)

var (
	retryInterval        = time.Second * 5
	operatorTimeout      = time.Minute * 4
	timeout              = time.Minute * 4
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

// RuntimeBasicTest barebones deployment test that makes sure applications will deploy and scale.
func RuntimeBasicTest(t *testing.T) {
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

	// create one replica of the operator deployment in current namespace with provided name
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "application-runtime-operator", 1, retryInterval, operatorTimeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = runtimeBasicScaleTest(t, f, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
}

func runtimeBasicScaleTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	helper := int32(1)

	exampleRuntime := util.MakeBasicRuntimeApplication(t, f, "example-runtime", namespace, helper)

	timestamp := time.Now().UTC()
	t.Logf("%s - Creating basic runtime application for scaling test...", timestamp)
	// Create application deployment and wait
	err = f.Client.Create(goctx.TODO(), exampleRuntime, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second, RetryInterval: time.Second})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "example-runtime", 1, retryInterval, timeout)
	if err != nil {
		return err
	}
	// -- Run all scaling tests below based on the above example deployment of 1 pods ---
	// update the number of replicas and return if failure occurs
	if err = runtimeUpdateScaleTest(t, f, namespace, exampleRuntime); err != nil {
		return err
	}
	timestamp = time.Now().UTC()
	t.Logf("%s - Completed basic runtime scale test", timestamp)
	return err
}

func runtimeUpdateScaleTest(t *testing.T, f *framework.Framework, namespace string, exampleRuntime *runtimeappv1beta1.RuntimeApplication) error {
	target := types.NamespacedName{Name: "example-runtime", Namespace: namespace}

	err := util.UpdateApplication(f, target, func(r *runtimeappv1beta1.RuntimeApplication) {
		helper2 := int32(2)
		r.Spec.Replicas = &helper2
	})
	if err != nil {
		return err
	}

	// wait for example-memcached to reach 2 replicas
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "example-runtime", 2, retryInterval, timeout)
	if err != nil {
		return err
	}
	return err
}
