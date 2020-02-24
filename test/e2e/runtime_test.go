package e2e

import (
	"testing"

	"github.com/application-runtimes/operator/pkg/apis"
	runtimeappv1beta1 "github.com/application-runtimes/operator/pkg/apis/runtimeapp/v1beta1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestRuntimeApplication ... end to end tests
func TestRuntimeApplication(t *testing.T) {
	runtimeApplicationList := &runtimeappv1beta1.RuntimeApplicationList{
		TypeMeta: metav1.TypeMeta{
			Kind: "RuntimeApplication",
		},
	}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, runtimeApplicationList)
	if err != nil {
		t.Fatalf("Failed to add CR scheme to framework: %v", err)
	}

	// t.Run("RuntimePullPolicyTest", RuntimePullPolicyTest)
	// t.Run("RuntimeBasicTest", RuntimeBasicTest)
	// t.Run("RuntimeStorageTest", RuntimeBasicStorageTest)
	// t.Run("RuntimePersistenceTest", RuntimePersistenceTest)
	t.Run("RuntimeProbeTest", RuntimeProbeTest)
	t.Run("RuntimeAutoScalingTest", RuntimeAutoScalingTest)
	t.Run("RuntimeServiceMonitorTest", RuntimeServiceMonitorTest)
	t.Run("RuntimeKnativeTest", RuntimeKnativeTest)
}
