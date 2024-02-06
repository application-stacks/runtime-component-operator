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

package controllers

import (
	"context"
	"fmt"
	"os"

	"github.com/application-stacks/runtime-component-operator/common"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	appstacksutils "github.com/application-stacks/runtime-component-operator/utils"
	"github.com/go-logr/logr"

	ctrl "sigs.k8s.io/controller-runtime"

	appstacksv1 "github.com/application-stacks/runtime-component-operator/api/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OperatorFullName  = "Runtime Component Operator"
	OperatorName      = "runtime-component-operator"
	OperatorShortName = "rco"
	APIName           = "RuntimeComponent"
)

// RuntimeComponentReconciler reconciles a RuntimeComponent object
type RuntimeComponentReconciler struct {
	appstacksutils.ReconcilerBase
	Log             logr.Logger
	watchNamespaces []string
}

// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=restricted,verbs=use,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=rc.app.stacks,resources=runtimecomponents;runtimecomponents/status;runtimecomponents/finalizers,verbs=get;list;watch;create;update;patch;delete,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;delete,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers;statefulsets,verbs=update,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=core,resources=services;secrets;serviceaccounts;configmaps,verbs=get;list;watch;create;update;delete,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;delete,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;networkpolicies,verbs=get;list;watch;create;update;delete,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes;routes/custom-host,verbs=get;list;watch;create;update;delete,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=image.openshift.io,resources=imagestreams;imagestreamtags,verbs=get;list;watch,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services,verbs=get;list;watch;create;update;delete,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;delete,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates;issuers,verbs=get;list;watch;create;update;delete,namespace=runtime-component-operator

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RuntimeComponentReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {

	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconcile " + APIName + " - starting")

	if ns, err := r.CheckOperatorNamespace(r.watchNamespaces); err != nil {
		return reconcile.Result{}, err
	} else {
		r.UpdateConfigMap(OperatorName, ns)
	}

	// Fetch the RuntimeComponent instance
	instance := &appstacksv1.RuntimeComponent{}
	if err := r.GetClient().Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if kerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	isKnativeSupported, err := r.IsGroupVersionSupported(servingv1.SchemeGroupVersion.String(), "Service")
	if err != nil {
		r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	} else if !isKnativeSupported && instance.Spec.CreateKnativeService != nil && *instance.Spec.CreateKnativeService {
		reqLogger.V(1).Info(fmt.Sprintf("%s is not supported on the cluster", servingv1.SchemeGroupVersion.String()))
	}

	// Check if there is an existing Deployment, Statefulset or Knative service by this name
	// not managed by this operator
	if err = appstacksutils.CheckForNameConflicts(APIName, instance.Name, instance.Namespace, r.GetClient(), request, isKnativeSupported); err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	// initialize the RuntimeComponent instance
	instance.Initialize()
	// If there's any validation error, don't bother with requeuing
	if _, err = appstacksutils.Validate(instance); err != nil {
		reqLogger.Error(err, "Error validating "+APIName)
		r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		return reconcile.Result{}, nil
	}

	if r.IsOpenShift() {
		// The order of items passed to the MergeMaps matters here! Annotations from GetOpenShiftAnnotations have higher importance. Otherwise,
		// it is not possible to override converted annotations.
		instance.Annotations = appstacksutils.MergeMaps(instance.Annotations, appstacksutils.GetOpenShiftAnnotations(instance))
	}

	if err = r.GetClient().Update(context.TODO(), instance); err != nil {
		reqLogger.Error(err, "Error updating "+APIName)
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	defaultMeta := metav1.ObjectMeta{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}

	imageReferenceOld := instance.Status.ImageReference
	if err = r.UpdateImageReference(instance); err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if imageReferenceOld != instance.Status.ImageReference {
		reqLogger.Info("Updating status.imageReference", "status.imageReference", instance.Status.ImageReference)
		if err = r.UpdateStatus(instance); err != nil {
			reqLogger.Error(err, "Error updating "+APIName+" status")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
	}

	if err = r.UpdateServiceAccount(instance, defaultMeta); err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	// If Knative is supported and being used, delete other resources and create/update Knative service
	// Otherwise, delete Knative service
	createKnativeService := instance.GetCreateKnativeService() != nil && *instance.GetCreateKnativeService()
	err = r.UpdateKnativeService(instance, defaultMeta, isKnativeSupported, createKnativeService)
	if createKnativeService {
		if err != nil {
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		} else {
			instance.Status.Versions.Reconciled = appstacksutils.RCOOperandVersion
			reqLogger.Info("Reconcile " + APIName + " - completed")
			return r.ManageSuccess(common.StatusConditionTypeReconciled, instance)
		}
	}

	useCertmanager, err := r.UpdateSvcCertSecret(instance, OperatorShortName, OperatorFullName, OperatorName)
	if err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if err = r.UpdateService(instance, defaultMeta, useCertmanager); err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if err = r.UpdateTLSReference(instance); err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if err = r.UpdateNetworkPolicy(instance, defaultMeta); err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if err = r.ReconcileBindings(instance); err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if instance.Spec.StatefulSet != nil {
		if err = r.UpdateStatefulSet(instance, defaultMeta); err != nil {
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
	} else {
		if err = r.UpdateDeployment(instance, defaultMeta); err != nil {
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
	}

	if err = r.UpdateAutoscaling(instance, defaultMeta); err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if err = r.UpdateRouteOrIngress(instance, defaultMeta); err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if err = r.UpdateServiceMonitor(instance, defaultMeta); err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	instance.Status.Versions.Reconciled = appstacksutils.RCOOperandVersion
	reqLogger.Info("Reconcile " + APIName + " - completed")
	return r.ManageSuccess(common.StatusConditionTypeReconciled, instance)
}

// SetupWithManager initializes reconciler
func (r *RuntimeComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {

	mgr.GetFieldIndexer().IndexField(context.Background(), &appstacksv1.RuntimeComponent{}, indexFieldImageStreamName, func(obj client.Object) []string {
		instance := obj.(*appstacksv1.RuntimeComponent)
		image, err := imageutil.ParseDockerImageReference(instance.Spec.ApplicationImage)
		if err == nil {
			imageNamespace := image.Namespace
			if imageNamespace == "" {
				imageNamespace = instance.Namespace
			}
			fullName := fmt.Sprintf("%s/%s", imageNamespace, image.Name)
			return []string{fullName}
		}
		return nil
	})

	watchNamespaces, err := appstacksutils.GetWatchNamespaces()
	if err != nil {
		r.Log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	watchNamespacesMap := make(map[string]bool)
	for _, ns := range watchNamespaces {
		watchNamespacesMap[ns] = true
	}
	isClusterWide := appstacksutils.IsClusterWide(watchNamespaces)

	pred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() && (isClusterWide || watchNamespacesMap[e.ObjectNew.GetNamespace()])
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Object.GetNamespace()]
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Object.GetNamespace()]
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Object.GetNamespace()]
		},
	}

	predSubResource := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return (isClusterWide || watchNamespacesMap[e.ObjectOld.GetNamespace()])
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Object.GetNamespace()]
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	predSubResWithGenCheck := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			return (isClusterWide || watchNamespacesMap[e.ObjectOld.GetNamespace()]) && e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Object.GetNamespace()]
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	b := ctrl.NewControllerManagedBy(mgr).For(&appstacksv1.RuntimeComponent{}, builder.WithPredicates(pred)).
		Owns(&corev1.Service{}, builder.WithPredicates(predSubResource)).
		Owns(&corev1.Secret{}, builder.WithPredicates(predSubResource)).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(predSubResWithGenCheck)).
		Owns(&appsv1.StatefulSet{}, builder.WithPredicates(predSubResWithGenCheck)).
		Owns(&autoscalingv1.HorizontalPodAutoscaler{}, builder.WithPredicates(predSubResource))

	ok, _ := r.IsGroupVersionSupported(routev1.SchemeGroupVersion.String(), "Route")
	if ok {
		b = b.Owns(&routev1.Route{}, builder.WithPredicates(predSubResource))
	}
	ok, _ = r.IsGroupVersionSupported(networkingv1.SchemeGroupVersion.String(), "Ingress")
	if ok {
		b = b.Owns(&networkingv1.Ingress{}, builder.WithPredicates(predSubResource))
	}
	ok, _ = r.IsGroupVersionSupported(servingv1.SchemeGroupVersion.String(), "Service")
	if ok {
		b = b.Owns(&servingv1.Service{}, builder.WithPredicates(predSubResource))
	}
	ok, _ = r.IsGroupVersionSupported(prometheusv1.SchemeGroupVersion.String(), "ServiceMonitor")
	if ok {
		b = b.Owns(&prometheusv1.ServiceMonitor{}, builder.WithPredicates(predSubResource))
	}
	ok, _ = r.IsGroupVersionSupported(imagev1.SchemeGroupVersion.String(), "ImageStream")
	if ok {
		b = b.Watches(&source.Kind{Type: &imagev1.ImageStream{}}, &EnqueueRequestsForCustomIndexField{
			Matcher: &ImageStreamMatcher{
				Klient:          mgr.GetClient(),
				WatchNamespaces: watchNamespaces,
			},
		})
	}
	return b.Complete(r)
}
