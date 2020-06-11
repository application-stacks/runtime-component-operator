package e2e

import (
	"os"
	"testing"
	
	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/application-stacks/runtime-component-operator/pkg/apis"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestRuntimeComponent ... end to end tests
func TestRuntimeComponent(t *testing.T) {
	runtimeComponentList := &appstacksv1beta1.RuntimeComponentList{
		TypeMeta: metav1.TypeMeta{
			Kind: "RuntimeComponent",
		},
	}

	cluster := os.Getenv("CLUSTER_ENV")
	t.Logf("running e2e tests as '%s'", cluster)

	err := framework.AddToFrameworkScheme(apis.AddToScheme, runtimeComponentList)
	if err != nil {
		t.Fatalf("Failed to add CR scheme to framework: %v", err)
	}

	// Basic tests that are runnable locally in minishift/kube
	t.Run("RuntimePullPolicyTest", RuntimePullPolicyTest)
	t.Run("RuntimeBasicTest", RuntimeBasicTest)
	t.Run("RuntimeProbeTest", RuntimeProbeTest)
	t.Run("RuntimeAutoScalingTest", RuntimeAutoScalingTest)
	t.Run("RuntimeStorageTest", RuntimeBasicStorageTest)
	t.Run("RuntimePersistenceTest", RuntimePersistenceTest)

	if cluster != "local" {
		// only test non-OCP features on minikube
		if cluster == "minikube" {
			testIndependantFeatures(t)
			return
		}

		// test all features that require some configuration
		testAdvancedFeatures(t)
		// test featurest hat require OCP
		if cluster == "ocp" {
			testOCPFeatures(t)
		}
	}
}

func testAdvancedFeatures(t *testing.T) {
	// These features require a bit of configuration
	// which makes them less ideal for quick minikube tests
	t.Run("RuntimeServiceMonitorTest", RuntimeServiceMonitorTest)
	t.Run("RuntimeKnativeTest", RuntimeKnativeTest)
	t.Run("RuntimeServiceBindingTest", RuntimeServiceBindingTest)
	t.Run("RuntimeCertManagerTest", RuntimeCertManagerTest)
}

// Verify functionality that is tied to OCP
func testOCPFeatures(t *testing.T) {
	t.Run("RuntimeImageStreamTest", RuntimeImageStreamTest)
}

// Verify functionality that is not expected to run on OCP
func testIndependantFeatures(t *testing.T) {
	// TODO: implement test for ingress
}
