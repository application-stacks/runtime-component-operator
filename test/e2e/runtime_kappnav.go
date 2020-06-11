package e2e

import (
	goctx "context"
	"errors"
	"testing"
	"time"

	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	e2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/application-stacks/runtime-component-operator/test/util"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	applicationsv1beta1 "sigs.k8s.io/application/pkg/apis/app/v1beta1"
)

var appName string = "test-app"

// RuntimeKappNavTest : Test kappnav feature set
func RuntimeKappNavTest(t *testing.T) {

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

	// add to scheme to framework can find the resource
	err = applicationsv1beta1.AddToScheme(f.Scheme)
	if err != nil {
		t.Fatal(err)
	}

	// wait for the operator as the following configmaps won't exist until it has deployed
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "runtime-component-operator", 1, retryInterval, operatorTimeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = createKappNavApplication(t, f, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = updateKappNavApplications(t, f, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = useExistingApplications(t, f, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
}

func createKappNavApplication(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	ns, err := ctx.GetNamespace()
	if err != nil {
		return err
	}

	const name string = "example-runtime-kappnav"

	runtime := util.MakeBasicRuntimeComponent(t, f, name, ns, 1)
	runtime.Spec.ApplicationName = appName

	err = f.Client.Create(goctx.TODO(), runtime, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		return err
	}

	// verify readiness of created resource
	err = e2eutil.WaitForDeployment(t, f.KubeClient, ns, name, 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	target := types.NamespacedName{Namespace: ns, Name: name}

	ok, err := verifyKappNavLabels(t, f, target)
	if err != nil {
		return err
	} else if !ok {
		return errors.New("could not find kappnav labels")
	}
	t.Log("kappnav labels found")

	err = util.WaitForApplicationCreated(t, f, types.NamespacedName{Name: appName, Namespace: ns})
	if err != nil {
		return err
	}
	t.Log("related application definition found")

	return nil
}

func updateKappNavApplications(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	ns, err := ctx.GetNamespace()
	if err != nil {
		return err
	}

	const name string = "example-runtime-kappnav"

	target := types.NamespacedName{Namespace: ns, Name: name}

	err = util.UpdateApplication(f, target, func(r *appstacksv1beta1.RuntimeComponent) {
		appDef := false
		r.Spec.CreateAppDefinition = &appDef
	})
	if err != nil {
		return err
	}

	ok, err := verifyKappNavLabels(t, f, target)
	if err != nil {
		return err
	} else if !ok {
		return errors.New("kappnav labels present after disabling")
	}
	t.Log("kappnav labels successfully removed")

	err = util.WaitForApplicationDelete(t, f, types.NamespacedName{Name: appName, Namespace: ns})
	if err != nil {
		return err
	}
	t.Log("created application definition removed")

	return nil
}

func useExistingApplications(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	ns, err := ctx.GetNamespace()
	if err != nil {
		return err
	}

	const name string = "example-runtime-kappnav"
	var existingAppName string = "existing-app"
	// add selector labels to verify that app was actually found
	selectMatchLabels := map[string]string{
		"test-key": "test-value",
	}

	// create existing application
	err = util.CreateApplicationTarget(f, ctx, types.NamespacedName{Name: existingAppName, Namespace: ns}, selectMatchLabels)
	if err != nil {
		return err
	}

	// connect to existing application IN namespace
	target := types.NamespacedName{Namespace: ns, Name: name}

	err = util.UpdateApplication(f, target, func(r *appstacksv1beta1.RuntimeComponent) {
		r.Spec.ApplicationName = existingAppName
	})
	if err != nil {
		return err
	}

	t.Log("waiting 100 seconds")
	time.Sleep(5 * time.Second)

	runtime := &appstacksv1beta1.RuntimeComponent{}
	err = f.Client.Get(goctx.TODO(), target, runtime)
	if err != nil {
		return err
	}

	runtimeLabels := runtime.Labels
	t.Log(runtime)

	if _, ok := runtimeLabels["test-key"]; !ok {
		t.Log(runtimeLabels)
		return errors.New("selector labels from target application not present")
	}
	t.Log("target application correctly applied to the component")

	if runtimeLabels["app.kubernetes.io/part-of"] != existingAppName {
		return errors.New("part-of label not correctly set")
	}
	t.Log("part-of label correctly set")

	return nil
}

func verifyKappNavLabels(t *testing.T, f *framework.Framework, target types.NamespacedName) (bool, error) {
	dep := &appsv1.Deployment{}
	err := f.Client.Get(goctx.TODO(), target, dep)
	if err != nil {
		return false, err
	}

	labels := dep.GetLabels()

	// verify that label present, full set of annos checked by unit tests
	if labels["kappnav.app.auto-create"] != "true" {
		return false, nil
	}

	return true, nil
}
