package e2e

import (
	"os"
	"sync"
	"testing"

	"github.com/application-stacks/runtime-component-operator/pkg/apis"
	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Test struct {
	Name string
	Test func(*testing.T)
}

var basicTests []Test = []Test{
	{"RuntimePullPolicyTest", RuntimePullPolicyTest},
	{"RuntimeBasicTest", RuntimeBasicTest},
	{"RuntimeProbeTest", RuntimeProbeTest},
	{"RuntimeAutoScalingTest", RuntimeAutoScalingTest},
	{"RuntimeBasicStorageTest", RuntimeBasicStorageTest},
	{"RuntimePersistenceTest", RuntimePersistenceTest},
}

var advancedTests []Test = []Test{
	{"RuntimeServiceBindingTest", RuntimeServiceBindingTest},
	// {"RuntimeCertManagerTest", RuntimeCertManagerTest},
	{"RuntimeKnativeTest", RuntimeKnativeTest},
	{"RuntimeServiceMonitorTest", RuntimeServiceMonitorTest},
	{"RuntimeKappNavTest", RuntimeKappNavTest},
}

var ocpTests []Test = []Test{
	{"RuntimeImageStreamTest", RuntimeImageStreamTest},
}

// TODO: add tests independant of OpenShift
var independantTests []Test = []Test{}

// NOTE: on the use of goroutines, concurrency puts a strain on the 3.11 cluster
// in particular on the docker registry. Otherwise each test could run at the same time.
// TestRuntimeComponent ... end to end tests
func TestRuntimeComponent(t *testing.T) {
	runtimeComponentList := &appstacksv1beta1.RuntimeComponentList{
		TypeMeta: metav1.TypeMeta{
			Kind: "RuntimeComponent",
		},
	}

	cluster, found := os.LookupEnv("CLUSTER_ENV")
	if !found {
		cluster = "minikube"
	}
	t.Logf("running e2e tests as '%s'", cluster)

	err := framework.AddToFrameworkScheme(apis.AddToScheme, runtimeComponentList)
	if err != nil {
		t.Fatalf("Failed to add CR scheme to framework: %v", err)
	}
	// sync up the test completion so that they all actually finish
	var wg sync.WaitGroup

	// basic tests that are capable of running in a freshly create cluster
	for _, test := range basicTests {
		wg.Add(1)
		go RuntimeTestRunner(&wg, t, test)
	}

	// tests for features that will require cluster configuration
	// i.e. knative requires installations
	if cluster != "minikube" {
		for _, test := range advancedTests {
			wg.Add(1)
			go RuntimeTestRunner(&wg, t, test)
		}
	}

	// tests for features NOT expected to run in OpenShift
	// i.e. Ingress
	if cluster == "minikube" || cluster == "kubernetes" {
		for _, test := range independantTests {
			wg.Add(1)
			go RuntimeTestRunner(&wg, t, test)
		}
	}

	// tests for features that ONLY exist in OpenShift
	// i.e. image streams are only in OpenShift
	if cluster == "ocp" {
		for _, test := range ocpTests {
			wg.Add(1)
			go RuntimeTestRunner(&wg, t, test)
		}
	}
	wg.Wait()
}

func RuntimeTestRunner(wg *sync.WaitGroup, t *testing.T, test Test) {
	defer wg.Done()
	t.Run(test.Name, test.Test)
}
