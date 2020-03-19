package e2e

import (
	goctx "context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	"github.com/application-stacks/runtime-component-operator/test/util"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	e2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	dynclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	runtimeProvider = "runtime-provider"
	runtimeConsumer = "runtime-consumer"
	runtimeSecret   = "my-secret"
	username        = "admin"
	password        = "adminpass"
	context         = "my-context"
	port            = "3000"
	mount           = "sample"
	namespace       = ""
)

// RuntimeServiceBindingTest verify behaviour of service binding feature
func RuntimeServiceBindingTest(t *testing.T) {
	os.Setenv("WATCH_NAMESPACE", namespace)
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

	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, ns, "runtime-component-operator", 1, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, ns, err)
	}

	// run tests for same namespace
	err = sameNamespaceTest(t, f, ctx, ns)
	if err != nil {
		t.Fatal(err)
	}

	// run test for different namespace
	err = diffNamespaceTest(t, f, ctx)
	if err != nil {
		t.Fatal(err)
	}

}

func createSecret(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, ns string) error {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      runtimeSecret,
			Namespace: ns,
		},
		Data: map[string][]byte{
			"username": []byte(username),
			"password": []byte(password),
		},
		Type: corev1.SecretTypeOpaque,
	}

	err := f.Client.Create(goctx.TODO(), &secret, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		return err
	}

	return nil
}

func createProviderService(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, ns string) error {
	runtime := util.MakeBasicRuntimeComponent(t, f, runtimeProvider, ns, 1)
	runtime.Spec.Service.Provides = &v1beta1.ServiceBindingProvides{
		Category: "openapi",
		Context:  "/" + context,
		Auth: &v1beta1.ServiceBindingAuth{
			Username: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: runtimeSecret},
				Key:                  "username",
			},
			Password: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: runtimeSecret},
				Key:                  "password",
			},
		},
	}

	err := f.Client.Create(goctx.TODO(), runtime, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, ns, runtimeProvider, 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	return nil
}

//
func createConsumeService(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, ns string) error {
	runtime := util.MakeBasicRuntimeComponent(t, f, runtimeConsumer, ns, 1)
	runtime.Spec.Service.Consumes = []v1beta1.ServiceBindingConsumes{
		v1beta1.ServiceBindingConsumes{
			Name:      runtimeProvider,
			Namespace: ns,
			Category:  "openapi",
			MountPath: "/" + mount,
		},
	}

	err := f.Client.Create(goctx.TODO(), runtime, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, ns, runtimeConsumer, 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	return nil
}

func sameNamespaceTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, ns string) error {
	err := createSecret(t, f, ctx, ns)
	if err != nil {
		util.FailureCleanup(t, f, ns, err)
	}

	err = createProviderService(t, f, ctx, ns)
	if err != nil {
		util.FailureCleanup(t, f, ns, err)
	}

	err = createConsumeService(t, f, ctx, ns)
	if err != nil {
		util.FailureCleanup(t, f, ns, err)
	}

	// Get consumer pod
	pods, err := getPods(f, ctx, runtimeConsumer, ns)
	if err != nil {
		util.FailureCleanup(t, f, ns, err)
	}
	podName := pods.Items[0].GetName()

	// Go inside the pod the pod for Consume service and check values are set
	out, err := exec.Command("kubectl", "exec", "-n", ns, "-it", podName, "--", "ls", "../sample/"+ns+"/"+runtimeProvider).Output()
	err = util.CommandError(t, err, out)
	if err != nil {
		t.Fatal("Directory not made")
	}
	directories := strings.Split(string(out), "\n")

	// Set values to check
	valuePairs := map[string]string{
		"context":  context,
		"hostname": runtimeProvider + "." + ns + ".svc.cluster.local",
		"password": password,
		"port":     port,
		"protocol": "http",
		"url":      "http://" + runtimeProvider + "." + ns + ".svc.cluster.local:" + port + "/" + context,
		"username": username,
	}

	for i := 0; i < len(directories)-1; i++ {
		checkSecret(t, ns, podName, directories[i], valuePairs)
	}

	return nil
}

func checkSecret(t *testing.T, ns string, podName string, directory string, valuePairs map[string]string) {
	out, err := exec.Command("kubectl", "exec", "-n", ns, "-it", podName, "--", "cat", "../"+mount+"/"+ns+"/"+runtimeProvider+"/"+directory).Output()
	err = util.CommandError(t, err, out)
	if err != nil {
		t.Fatal(directory + " is not set")
	}

	if valuePairs[directory] != string(out) {
		t.Fatalf("The value is not set correctly. Expected: %s. Actual: %s", valuePairs[directory], string(out))
	}

	t.Log(string(out))
}

func getPods(f *framework.Framework, ctx *framework.TestCtx, target string, ns string) (*corev1.PodList, error) {
	key := map[string]string{"app.kubernetes.io/name": target}

	options := &dynclient.ListOptions{
		LabelSelector: labels.Set(key).AsSelector(),
		Namespace:     ns,
	}

	podList := &corev1.PodList{}

	err := f.Client.List(goctx.TODO(), podList, options)
	if err != nil {
		return nil, err
	}

	return podList, nil
}

func diffNamespaceTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {

	ns2 := "e2e-svc-binding"
	// Make a new namespace
	out, err := exec.Command("kubectl", "create", "namespace", ns2).Output()
	err = util.CommandError(t, err, out)
	if err != nil {
		t.Fatal("New namespace not made")
	}

	err = createConsumeService(t, f, ctx, ns2)
	if err != nil {
		util.FailureCleanup(t, f, ns2, err)
	}

	// Get consumer pod
	pods, err := getPods(f, ctx, runtimeConsumer, ns2)
	if err != nil {
		util.FailureCleanup(t, f, ns2, err)
	}
	podName := pods.Items[0].GetName()

	// Go inside the pod the pod for Consume service and check values are set
	out, err = exec.Command("kubectl", "exec", "-n", ns2, "-it", podName, "--", "ls", "../sample/"+ns2+"/"+runtimeProvider).Output()
	err = util.CommandError(t, err, out)
	if err != nil {
		t.Fatal("Directory not made")
	}
	directories := strings.Split(string(out), "\n")

	// Set values to check
	valuePairs := map[string]string{
		"context":  context,
		"hostname": runtimeProvider + "." + ns2 + ".svc.cluster.local",
		"password": password,
		"port":     port,
		"protocol": "http",
		"url":      "http://" + runtimeProvider + "." + ns2 + ".svc.cluster.local:" + port + "/" + context,
		"username": username,
	}

	for i := 0; i < len(directories)-1; i++ {
		checkSecret(t, ns2, podName, directories[i], valuePairs)
	}

	out, err := exec.Command("kubectl", "delete", "namespace", ns2).Output()
	err = util.CommandError(t, err, out)
	if err != nil {
		t.Fatal("New namespace not deleted")
	}
	return nil
}
