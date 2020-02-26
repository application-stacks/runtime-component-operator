package e2e

import (
	goctx "context"
	"errors"
	"testing"
	"time"

	appstacksv1beta1 "github.com/application-stacks/operator/pkg/apis/appstacks/v1beta1"
	"github.com/application-stacks/operator/test/util"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	e2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	k "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "application-stacks-operator", 1, retryInterval, operatorTimeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
	timestamp := time.Now().UTC()
	t.Logf("%s - Starting runtime pull policy test...", timestamp)

	// create one replica of the operator deployment in current namespace with provided name
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "application-stacks-operator", 1, retryInterval, operatorTimeout)
	if err != nil {
		t.Fatal(err)
	}

	replicas := int32(1)
	policy := k.PullAlways

	runtimeApplication := util.MakeBasicRuntimeApplication(t, f, "example-runtime-pullpolicy", namespace, replicas)
	runtimeApplication.Spec.PullPolicy = &policy

	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err = f.Client.Create(goctx.TODO(), runtimeApplication, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	// wait for example-runtime-pullpolicy to reach 2 replicas
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "example-runtime-pullpolicy", 1, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	timestamp = time.Now().UTC()
	t.Logf("%s - Deployment created, verifying pull policy...", timestamp)

	if err = verifyPullPolicy(t, f, runtimeApplication); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
}

func verifyPullPolicy(t *testing.T, f *framework.Framework, app *appstacksv1beta1.RuntimeApplication) error {
	name := app.ObjectMeta.Name
	ns := app.ObjectMeta.Namespace

	deploy, err := f.KubeClient.AppsV1().Deployments(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		t.Logf("Got error when getting PullPolicy %s: %s", name, err)
		return err
	}

	if deploy.Spec.Template.Spec.Containers[0].ImagePullPolicy != "Always" {
		return errors.New("pull policy was not successfully configured from the default value")
	}
	return nil
}
