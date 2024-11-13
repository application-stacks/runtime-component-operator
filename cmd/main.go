/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	appstacksv1 "github.com/application-stacks/runtime-component-operator/api/v1"
	"github.com/application-stacks/runtime-component-operator/internal/controller"
	"github.com/application-stacks/runtime-component-operator/utils"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(appstacksv1.AddToScheme(scheme))

	utilruntime.Must(routev1.AddToScheme(scheme))

	utilruntime.Must(prometheusv1.AddToScheme(scheme))

	utilruntime.Must(imagev1.AddToScheme(scheme))

	utilruntime.Must(servingv1.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	//var metricsAddr string
	var enableLeaderElection bool
	//flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	var probeAddr string

	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	utils.CreateConfigMap(controller.OperatorName)

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// see https://github.com/operator-framework/operator-sdk/issues/1813
	leaseDuration := 30 * time.Second
	renewDeadline := 20 * time.Second

	watchNamespace, err := getWatchNamespace()
	if err != nil {
		setupLog.Error(err, "unable to get WatchNamespace, "+
			"the manager will watch and manage resources in all Namespaces")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				Port: 9443,
			},
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "c407d44e.rc.app.stacks",
		LeaseDuration:          &leaseDuration,
		RenewDeadline:          &renewDeadline,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{watchNamespace: cache.Config{}},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.RuntimeComponentReconciler{
		ReconcilerBase: utils.NewReconcilerBase(mgr.GetAPIReader(), mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetEventRecorderFor("runtime-component-operator")),
		Log:            ctrl.Log.WithName("controller").WithName("RuntimeComponent"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RuntimeComponent")
		os.Exit(1)
	}
	if err = (&controller.RuntimeOperationReconciler{
		Client:     mgr.GetClient(),
		Log:        ctrl.Log.WithName("controller").WithName("RuntimeOperation"),
		Scheme:     mgr.GetScheme(),
		Recorder:   mgr.GetEventRecorderFor(""),
		RestConfig: mgr.GetConfig(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RuntimeOperation")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

}

// getWatchNamespace returns the Namespace the operator should be watching for changes
func getWatchNamespace() (string, error) {
	// WatchNamespaceEnvVar is the constant for env variable WATCH_NAMESPACE
	// which specifies the Namespace to watch.
	// An empty value means the operator is running with cluster scope.
	var watchNamespaceEnvVar = "WATCH_NAMESPACE"

	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	return ns, nil
}
