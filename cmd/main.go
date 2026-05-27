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
	"github.com/application-stacks/runtime-component-operator/common"
	"github.com/application-stacks/runtime-component-operator/internal/controller"
	"github.com/application-stacks/runtime-component-operator/utils"
	"github.com/awnumar/memguard"
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
	memguard.CatchInterrupt()
	defer memguard.Purge()

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	// var metricsCertPath, metricsCertName, metricsCertKey string
	// var webhookCertPath, webhookCertName, webhookCertKey string
	// var secureMetrics bool
	// var enableHTTP2 bool
	// var tlsOpts []func(*tls.Config)
	// flag.BoolVar(&secureMetrics, "metrics-secure", true,
	// 	"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	// flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	// flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	// flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	// flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
	// 	"The directory that contains the metrics server certificate.")
	// flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	// flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	// flag.BoolVar(&enableHTTP2, "enable-http2", false,
	// 	"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	opts := zap.Options{
		Level:           common.LevelFunc,
		StacktraceLevel: common.StackLevelFunc,
		Development:     true,
	}
	// opts.BindFlags(flag.CommandLine)
	flag.Parse()

	utils.CreateConfigMap(controller.OperatorName)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// disableHTTP2 := func(c *tls.Config) {
	// 	setupLog.Info("disabling http/2")
	// 	c.NextProtos = []string{"http/1.1"}
	// }

	// if !enableHTTP2 {
	// 	tlsOpts = append(tlsOpts, disableHTTP2)
	// }

	// // Create watchers for metrics and webhooks certificates
	// var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// // Initial webhook TLS options
	// webhookTLSOpts := tlsOpts

	// if len(webhookCertPath) > 0 {
	// 	setupLog.Info("Initializing webhook certificate watcher using provided certificates",
	// 		"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

	// 	var err error
	// 	webhookCertWatcher, err = certwatcher.New(
	// 		filepath.Join(webhookCertPath, webhookCertName),
	// 		filepath.Join(webhookCertPath, webhookCertKey),
	// 	)
	// 	if err != nil {
	// 		setupLog.Error(err, "Failed to initialize webhook certificate watcher")
	// 		os.Exit(1)
	// 	}

	// 	webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
	// 		config.GetCertificate = webhookCertWatcher.GetCertificate
	// 	})
	// }

	// see https://github.com/operator-framework/operator-sdk/issues/1813
	leaseDuration := 30 * time.Second
	renewDeadline := 20 * time.Second

	watchNamespace, err := getWatchNamespace()
	if err != nil {
		setupLog.Error(err, "unable to get WatchNamespace, "+
			"the manager will watch and manage resources in all Namespaces")
	}

	metricsServerOptions := metricsserver.Options{
		BindAddress: metricsAddr,
		// SecureServing: secureMetrics,
		// TLSOpts:       tlsOpts,
	}

	// if secureMetrics {
	// 	metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	// }

	// if len(metricsCertPath) > 0 {
	// 	setupLog.Info("Initializing metrics certificate watcher using provided certificates",
	// 		"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

	// 	var err error
	// 	metricsCertWatcher, err = certwatcher.New(
	// 		filepath.Join(metricsCertPath, metricsCertName),
	// 		filepath.Join(metricsCertPath, metricsCertKey),
	// 	)
	// 	if err != nil {
	// 		setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
	// 		os.Exit(1)
	// 	}

	// 	metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
	// 		config.GetCertificate = metricsCertWatcher.GetCertificate
	// 	})
	// }

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsServerOptions,
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
	// if metricsCertWatcher != nil {
	// 	setupLog.Info("Adding metrics certificate watcher to manager")
	// 	if err := mgr.Add(metricsCertWatcher); err != nil {
	// 		setupLog.Error(err, "unable to add metrics certificate watcher to manager")
	// 		os.Exit(1)
	// 	}
	// }

	// if webhookCertWatcher != nil {
	// 	setupLog.Info("Adding webhook certificate watcher to manager")
	// 	if err := mgr.Add(webhookCertWatcher); err != nil {
	// 		setupLog.Error(err, "unable to add webhook certificate watcher to manager")
	// 		os.Exit(1)
	// 	}
	// }

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
