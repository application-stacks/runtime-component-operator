package e2e

import (
	"testing"

	"github.com/application-stacks/runtime-component-operator/pkg/apis"
	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestRuntimeComponent ... end to end tests
func TestRuntimeComponent(t *testing.T) {
	runtimeComponentList := &appstacksv1beta1.RuntimeComponentList{
		TypeMeta: metav1.TypeMeta{
			Kind: "RuntimeComponent",
		},
	}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, runtimeComponentList)
	if err != nil {
		t.Fatalf("Failed to add CR scheme to framework: %v", err)
	}

	t.Run("RuntimePullPolicyTest", RuntimePullPolicyTest)
	t.Run("RuntimeBasicTest", RuntimeBasicTest)
	t.Run("RuntimeStorageTest", RuntimeBasicStorageTest)
	t.Run("RuntimePersistenceTest", RuntimePersistenceTest)
	t.Run("RuntimeProbeTest", RuntimeProbeTest)
	t.Run("RuntimeAutoScalingTest", RuntimeAutoScalingTest)
	t.Run("RuntimeServiceMonitorTest", RuntimeServiceMonitorTest)
	t.Run("RuntimeKnativeTest", RuntimeKnativeTest)
	t.Run("RuntimeServiceBindingTest", RuntimeServiceBindingTest)
}
