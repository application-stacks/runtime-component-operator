package e2e

import (
	"testing"

	"github.com/application-stacks/operator/test/util"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	e2eutil "github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
)

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


}
