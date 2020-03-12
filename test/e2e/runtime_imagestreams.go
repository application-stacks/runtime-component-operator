package e2e

import (
	goctx "context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/application-stacks/operator/test/util"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"k8s.io/apimachinery/pkg/types"
)

//RuntimeImageStreamTest ...
func RuntimeImageStreamTest(t *testing.T) {
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

	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "application-stacks-operator", 1, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = runtimeImageStreamTest(t, f, ctx); err != nil {
		out, err := exec.Command("oc", "delete", "imagestream", "imagestream-example", "-n", namespace).Output()
		if err != nil {
			t.Fatalf("Failed to delete imagestream: %s", out)
		}
		util.FailureCleanup(t, f, namespace, err)
	}
}

func runtimeImageStreamTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	const name = "runtime-app"
	const imgstreamName = "imagestream-example"

	ns, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	// Create the imagestream
	out, err := exec.Command("oc", "import-image", imgstreamName, "--from=navidsh/demo-day:v0.1.0", "--confirm").Output()
	if err != nil {
		t.Fatalf("Creating the imagestream failed: %s", out)
	}

	// Make an appplication that points to the imagestream
	runtime := util.MakeImageStreamRuntimeComponent(t, f, name, ns, 1, imgstreamName)

	timestamp := time.Now().UTC()
	t.Logf("%s - Creating runtime application...", timestamp)
	err = f.Client.Create(goctx.TODO(), runtime, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second, RetryInterval: time.Second})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, ns, name, 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	// Get the application
	f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: ns}, runtime)
	if err != nil {
		t.Fatal(err)
	}

	firstImage := runtime.Status.ImageReference
	// Update the imagestreamtag
	tag := `{"tag":{"from":{"name": "navidsh/demo-day:v0.2.0"}}}`
	out, err = exec.Command("oc", "patch", "imagestreamtag", imgstreamName+":latest", "-p", tag).Output()
	if err != nil {
		t.Fatalf("Updating the imagestreamtag failed: %s", out)
	}

	time.Sleep(4000 * time.Millisecond)

	// Get the application
	f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: ns}, runtime)
	if err != nil {
		t.Fatal(err)
	}

	secondImage := runtime.Status.ImageReference
	// Check if the image the application is pointing to has been changed
	if firstImage == secondImage {
		t.Fatalf("The docker image has not been updated. It is still %s", firstImage)
	}

	// Update the imagestreamtag again
	tag = `{"tag":{"from":{"name": "navidsh/demo-day:v0.1.0"}}}`
	out, err = exec.Command("oc", "patch", "imagestreamtag", imgstreamName+":latest", "-p", tag).Output()
	if err != nil {
		t.Fatalf("Updating the imagestreamtag failed: %s", out)
	}

	time.Sleep(4000 * time.Millisecond)

	// Get the application
	f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: ns}, runtime)
	if err != nil {
		t.Fatal(err)
	}

	firstImage = runtime.Status.ImageReference
	// Check if the image the application is pointing to has been changed
	if firstImage == secondImage {
		t.Fatalf("The docker image has not been updated. It is still %s", secondImage)
	}

	return nil
}
