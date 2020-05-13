package e2e

import (
	goctx "context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/application-stacks/runtime-component-operator/test/util"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	e2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	corev1 "k8s.io/api/core/v1"
	dynclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ifNotPresentMessage = "Container image \"navidsh/demo-day\" already present on machine"
	alwaysMessage       = "pulling image \"navidsh/demo-day\""
	neverMessage        = "Container image \"navidsh/demo-day-fake\" is not present with pull policy of Never"
)

// RuntimePullPolicyTest checks that the configured pull policy is applied to deployment
func RuntimePullPolicyTest(t *testing.T) {
	ctx, err := util.InitializeContext(t, cleanupTimeout, retryInterval)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Cleanup()

	f := framework.Global
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("could not get namespace: %v", err)
	}
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "runtime-component-operator", 1, retryInterval, operatorTimeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
	timestamp := time.Now().UTC()
	t.Logf("%s - Starting runtime pull policy test...", timestamp)

	if err = testPullPolicyAlways(t, f, namespace, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = testPullPolicyIfNotPresent(t, f, namespace, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = testPullPolicyNever(t, f, namespace, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
}

func testPullPolicyAlways(t *testing.T, f *framework.Framework, namespace string, ctx *framework.TestCtx) error {
	replicas := int32(1)
	policy := corev1.PullAlways

	runtimeComponent := util.MakeBasicRuntimeComponent(t, f, "example-runtime-pullpolicy-always", namespace, replicas)
	runtimeComponent.Spec.PullPolicy = &policy

	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err := f.Client.Create(goctx.TODO(), runtimeComponent, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	// wait for example-runtime-pullpolicy to reach 1 replica
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "example-runtime-pullpolicy-always", 1, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	timestamp := time.Now().UTC()
	t.Logf("%s - Deployment created, verifying pull policy...", timestamp)

	return searchEventMessages(t, f, alwaysMessage, namespace)
}

func testPullPolicyIfNotPresent(t *testing.T, f *framework.Framework, namespace string, ctx *framework.TestCtx) error {
	replicas := int32(1)

	runtimeComponent := util.MakeBasicRuntimeComponent(t, f, "example-runtime-pullpolicy-ifnotpresent", namespace, replicas)

	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err := f.Client.Create(goctx.TODO(), runtimeComponent, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	// wait for example-runtime-pullpolicy to reach 1 replica
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "example-runtime-pullpolicy-ifnotpresent", 1, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	timestamp := time.Now().UTC()
	t.Logf("%s - Deployment created, verifying pull policy...", timestamp)

	return searchEventMessages(t, f, ifNotPresentMessage, namespace)
}

func searchEventMessages(t *testing.T, f *framework.Framework, key string, namespace string) error {
	options := &dynclient.ListOptions{
		Namespace: namespace,
	}

	eventlist := &corev1.EventList{}
	err := f.Client.List(goctx.TODO(), eventlist, options)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("***** Logging events in namespace: %s", namespace)
	for i := len(eventlist.Items) - 1; i >= 0; i-- {
		if strings.Contains(eventlist.Items[i].Message, "navidsh/demo-day") {
			if eventlist.Items[i].Message == key {
				return nil
			}
		}
		t.Log("------------------------------------------------------------")
		t.Log(eventlist.Items[i].Message)
	}

	return errors.New("The pull policy was not correctly set")

}

func testPullPolicyNever(t *testing.T, f *framework.Framework, namespace string, ctx *framework.TestCtx) error {
	replicas := int32(1)
	policy := corev1.PullNever

	runtimeComponent := util.MakeBasicRuntimeComponent(t, f, "example-runtime-pullpolicy-never", namespace, replicas)
	runtimeComponent.Spec.PullPolicy = &policy
	runtimeComponent.Spec.ApplicationImage = "navidsh/demo-day-fake"

	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err := f.Client.Create(goctx.TODO(), runtimeComponent, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	for i := 0; i < 5; i++ {
		time.Sleep(time.Millisecond * 1000)
	}

	timestamp := time.Now().UTC()
	t.Logf("%s - Deployment created, verifying pull policy...", timestamp)

	return searchEventMessages(t, f, neverMessage, namespace)
}
