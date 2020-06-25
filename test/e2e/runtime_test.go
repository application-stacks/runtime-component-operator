package e2e

import (
	"os"
	"testing"

	"github.com/application-stacks/runtime-component-operator/pkg/apis"
	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Test struct {
	Test func(*testing.T)
	Name string
}

var tests []Test = []Test{
	{Name: "RuntimePullPolicyTest", Test: RuntimePullPolicyTest},
	{Name: "RuntimeBasicTest", Test: RuntimeBasicTest},
	{Name: "RuntimeProbeTest", Test: RuntimeProbeTest},
	{Name: "RuntimeAutoScalingTest", Test: RuntimeAutoScalingTest},
	{Name: "RuntimeBasicStorageTest", Test: RuntimeBasicStorageTest},
	{Name: "RuntimePersistenceTest", Test: RuntimePersistenceTest},
}

// NOTE: on the use of goroutines, concurrency puts a strain on the 3.11 cluster
// in particular on the docker registry. Otherwise each test could run at the same time.
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

	// basic tests that are runnable locally in minishift/kube
	go testBasicFeatures(t)

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

// Long running tests that should be in routines:
// ServiceBindingTest - 258
// AutoScalingTest - 163
// CertManager - 93
func testBasicFeatures(t *testing.T) {
	// t.Run("RuntimePullPolicyTest", RuntimePullPolicyTest)
	// t.Run("RuntimeBasicTest", RuntimeBasicTest)
	// t.Run("RuntimeProbeTest", RuntimeProbeTest)
	// // This test is long, create go routine
	// go t.Run("RuntimeAutoScalingTest", RuntimeAutoScalingTest)
	// t.Run("RuntimeStorageTest", RuntimeBasicStorageTest)
	// t.Run("RuntimePersistenceTest", RuntimePersistenceTest)
	for _, test := range tests {
		go t.Run(test.Name, test.Test)
	}
}

func testAdvancedFeatures(t *testing.T) {
	// These features require a bit of configuration
	// which makes them less ideal for quick minikube tests

	// create routines for the longest tests
	go t.Run("RuntimeServiceBindingTest", RuntimeServiceBindingTest)
	go t.Run("RuntimeCertManagerTest", RuntimeCertManagerTest)

	t.Run("RuntimeKnativeTest", RuntimeKnativeTest)
	t.Run("RuntimeServiceMonitorTest", RuntimeServiceMonitorTest)
}

// Verify functionality that is tied to OCP
func testOCPFeatures(t *testing.T) {
	t.Run("RuntimeImageStreamTest", RuntimeImageStreamTest)
}

// Verify functionality that is not expected to run on OCP
func testIndependantFeatures(t *testing.T) {
	// TODO: implement test for ingress
}
