package e2e

import (
	goctx "context"
	"testing"

	"github.com/application-stacks/operator/pkg/apis/appstacks/v1beta1"
	"github.com/application-stacks/operator/test/util"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	e2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RuntimeServiceBindingTest verify behaviour of service binding feature
func RuntimeServiceBindingTest(t *testing.T) {
	ctx, err := util.InitializeContext(t, cleanupTimeout, retryInterval)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Cleanup()

	ns, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Couldn't get namespace: %v", err)
	}

	t.Logf("Namespace: %s", ns)

	f := framework.Global

	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, ns, "application-stacks-operator", 1, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, ns, err)
	}

	err = createSecret(t, f, ctx, ns, "my-secret")
	if err != nil {
		util.FailureCleanup(t, f, ns, err)
	}

	err = createProviderService(t, f, ctx, ns, "runtime-provider", "my-secret")
	if err != nil {
		util.FailureCleanup(t, f, ns, err)
	}

	err = createConsumeService(t, f, ctx, ns, "runtime-consumer", "my-secret")
	if err != nil {
		util.FailureCleanup(t, f, ns, err)
	}

	err = verifyServices(t, f, ctx)
}

func createSecret(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, ns, n string) error {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n,
			Namespace: ns,
		},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("adminpass"),
		},
	}

	err := f.Client.Create(goctx.TODO(), &secret, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		return err
	}

	return nil
}

func createProviderService(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, ns, n, secretName string) error {
	runtime := util.MakeBasicRuntimeComponent(t, f, n, ns, 1)
	runtime.Spec.Service.Provides = &v1beta1.ServiceBindingProvides{
		Category: "openapi",
		Context:  "/my-context",
		Auth: &v1beta1.ServiceBindingAuth{
			Username: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "username",
			},
			Password: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "password",
			},
		},
	}

	err := f.Client.Create(goctx.TODO(), runtime, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, ns, n, 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	return nil
}

//
func createConsumeService(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, ns, n, secretName string) error {
	runtime := util.MakeBasicRuntimeComponent(t, f, n, ns, 1)
	runtime.Spec.Service.Consumes = []v1beta1.ServiceBindingConsumes{
		v1beta1.ServiceBindingConsumes{
			Name:      "runtime-provider",
			Namespace: ns,
			Category:  "openapi",
			MountPath: "/sample",
		},
	}

	err := f.Client.Create(goctx.TODO(), runtime, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, ns, n, 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	return nil
}

func verifyConsumerService(t *testing.T, f *framework.Framework, ns string) error {
	return nil
}
