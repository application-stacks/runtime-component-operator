package controllers

import (
	appstacksv1 "github.com/application-stacks/runtime-component-operator/api/v1"
	"github.com/application-stacks/runtime-component-operator/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
)

var (
	name      = "my-app"
	namespace = "runtime"
)

type Test struct {
	test     string
	expected interface{}
	actual   interface{}
}

func TestShouldDeleteRoute(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)
	spec := appstacksv1.RuntimeComponentSpec{}
	runtime := createRuntimeComponent(name, namespace, spec)
	defaultCase := shouldDeleteRoute(runtime)

	// Host exists in spec, no previous host
	runtime.Spec.Route = &appstacksv1.RuntimeComponentRoute{
		Host: "new.host",
	}
	noPrevious := shouldDeleteRoute(runtime)

	// There was previously a hostname, there still is
	runtime.GetStatus().SetReference(common.StatusReferenceRouteHost, "old.host")
	runtime.Spec.Route = &appstacksv1.RuntimeComponentRoute{
		Host: "new.host",
	}
	dontDeleteHost := shouldDeleteRoute(runtime)

	// There was previously a hostname, now there is not
	runtime.Spec.Route = nil
	previousHostExisted := shouldDeleteRoute(runtime)

	// When there is a defaultHost in config.
	// This should be ignored as the route is nil
	common.Config[common.OpConfigDefaultHostname] = "default.host"
	noPreviousWithDefault := shouldDeleteRoute(runtime)

	// If the route object exists with no host,
	// default host is set in config
	// a previous host existed
	// we shouldn't delete
	runtime.Spec.Route = &appstacksv1.RuntimeComponentRoute{
		Path: "dummy/path",
	}
	previousWasDefault := shouldDeleteRoute(runtime)

	// No previous, but default set
	// No previous so shouldn't delete regardless
	runtime.GetStatus().SetReferences(nil)
	noPreviousWithDefaultAndRoute := shouldDeleteRoute(runtime)

	testCR := []Test{
		{test: "default case", expected: false, actual: defaultCase},
		{test: "host is set in spec, no previous host", expected: false, actual: noPrevious},
		{test: "host is set in spec", expected: false, actual: dontDeleteHost},
		{test: "previous host existed", expected: true, actual: previousHostExisted},
		{test: "previous host existed, only default host set", expected: true, actual: noPreviousWithDefault},
		{test: "previous host existed, default host set and route set", expected: false, actual: previousWasDefault},
		{test: "no previous, default host set and route set", expected: false, actual: noPreviousWithDefaultAndRoute},
	}

	verifyTests(testCR, t)
}
func createRuntimeComponent(n, ns string, spec appstacksv1.RuntimeComponentSpec) *appstacksv1.RuntimeComponent {
	app := &appstacksv1.RuntimeComponent{
		ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: ns},
		Spec:       spec,
	}
	return app
}
func verifyTests(tests []Test, t *testing.T) {
	for _, tt := range tests {
		if !reflect.DeepEqual(tt.actual, tt.expected) {
			t.Errorf("%s test expected: (%v) actual: (%v)", tt.test, tt.expected, tt.actual)
		}
	}
}
