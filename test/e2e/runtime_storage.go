package e2e

import (
	goctx "context"
	"testing"
	"time"

	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	e2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/application-stacks/runtime-component-operator/test/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

// RuntimeBasicStorageTest check that when persistence is configured that a statefulset is deployed
func RuntimeBasicStorageTest(t *testing.T) {
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

	exampleRuntime := util.MakeBasicRuntimeComponent(t, f, "example-runtime-storage", namespace, 1)
	exampleRuntime.Spec.Storage = &appstacksv1beta1.RuntimeComponentStorage{
		Size:      "10Mi",
		MountPath: "/mnt/data",
	}

	err = f.Client.Create(goctx.TODO(), exampleRuntime, &framework.CleanupOptions{
		TestContext:   ctx,
		Timeout:       time.Second * 5,
		RetryInterval: time.Second * 1,
	})
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
	err = util.WaitForStatefulSet(t, f.KubeClient, namespace, "example-runtime-storage", 1, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
	// verify that removing the storage config returns it to a deployment not a stateful set
	if err = updateStorageConfig(t, f, ctx, exampleRuntime); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
}

func updateStorageConfig(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, app *appstacksv1beta1.RuntimeComponent) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return err
	}

	target := types.NamespacedName{Name: app.Name, Namespace: namespace}

	err = util.UpdateApplication(f, target, func(r *appstacksv1beta1.RuntimeComponent) {
		// remove storage definition to return it to a deployment
		r.Spec.Storage = nil
		r.Spec.VolumeMounts = nil
	})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, app.Name, 1, retryInterval, timeout)
	if err != nil {
		return err
	}
	return nil
}

// RuntimePersistenceTest Verify the volume persistence claims.
func RuntimePersistenceTest(t *testing.T) {
	ctx, err := util.InitializeContext(t, cleanupTimeout, retryInterval)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Cleanup()

	f := framework.Global

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}

	RequestLimits := map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceStorage: resource.MustParse("1Gi"),
	}

	// Create PVC and mount for our statefulset.
	exampleRuntime := util.MakeBasicRuntimeComponent(t, f, "example-runtime-persistence", namespace, 1)
	exampleRuntime.Spec.Storage = &appstacksv1beta1.RuntimeComponentStorage{
		VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
			metav1.TypeMeta{},
			metav1.ObjectMeta{
				Name: "pvc",
			},
			corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: RequestLimits,
				},
			},
			corev1.PersistentVolumeClaimStatus{},
		},
	}
	exampleRuntime.Spec.VolumeMounts = []corev1.VolumeMount{corev1.VolumeMount{
		Name:      "pvc",
		MountPath: "/data",
	}}

	err = f.Client.Create(goctx.TODO(), exampleRuntime, &framework.CleanupOptions{
		TestContext:   ctx,
		Timeout:       cleanupTimeout,
		RetryInterval: cleanupRetryInterval,
	})
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	err = util.WaitForStatefulSet(t, f.KubeClient, namespace, "example-runtime-persistence", 1, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	// again remove the storage configuration and see that it deploys correctly.
	if err = updateStorageConfig(t, f, ctx, exampleRuntime); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
}
