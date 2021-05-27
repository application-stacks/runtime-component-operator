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

	"github.com/apex/log"
	"github.com/application-stacks/runtime-component-operator/common"
	"github.com/pkg/errors"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	"github.com/application-stacks/runtime-component-operator/utils"
	appstacksutils "github.com/application-stacks/runtime-component-operator/utils"
	"github.com/go-logr/logr"

	ctrl "sigs.k8s.io/controller-runtime"

	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/api/v1beta1"
	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	certmngrv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	servingv1alpha1 "knative.dev/serving/pkg/apis/serving/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RuntimeComponentReconciler reconciles a RuntimeComponent object
type RuntimeComponentReconciler struct {
	appstacksutils.ReconcilerBase
	Log             logr.Logger
	watchNamespaces []string
}

// +kubebuilder:rbac:groups=app.stacks,resources=runtimecomponents;runtimecomponents/status,verbs=*
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=*
// +kubebuilder:rbac:groups=core,resources=services;secrets;serviceaccounts;configmaps,verbs=*
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=*
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=*
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes;routes/custom-host,verbs=*
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=*
// +kubebuilder:rbac:groups=certmanager.k8s.io,resources=certificates,verbs=*
// +kubebuilder:rbac:groups=image.openshift.io,resources=imagestream;imagestreamtags,verbs=get;list;watch
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services,verbs=*
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=*
// +kubebuilder:rbac:groups=app.k8s.io,resources=applications,verbs=*
// +kubebuilder:rbac:groups=apps.openshift.io,resources=servicebindingrequests,verbs=*

func (r *RuntimeComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling RuntimeComponent")

	ns, err := utils.GetOperatorNamespace()
	// When running the operator locally, `ns` will be empty string
	if ns == "" {
		// Since this method can be called directly from unit test, populate `watchNamespaces`.
		if r.watchNamespaces == nil {
			r.watchNamespaces, err = appstacksutils.GetWatchNamespaces()
			if err != nil {
				reqLogger.Error(err, "Error getting watch namespace")
				return reconcile.Result{}, err
			}
		}
		// If the operator is running locally, use the first namespace in the `watchNamespaces`
		// `watchNamespaces` must have at least one item
		ns = r.watchNamespaces[0]
	}

	configMap, err := r.GetOpConfigMap("runtime-component-operator", ns)
	if err != nil {
		r.Log.Info("Failed to find runtime-component-operator config map")
		common.Config = common.DefaultOpConfig()
		configMap = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "runtime-component-operator", Namespace: ns}}
		configMap.Data = common.Config
	} else {
		common.Config.LoadFromConfigMap(configMap)
	}

	_, err = controllerutil.CreateOrUpdate(context.TODO(), r.GetClient(), configMap, func() error {
		configMap.Data = common.Config
		return nil
	})

	if err != nil {
		log.Info("Failed to update runtime-component-operator config map")
	}

	// Fetch the RuntimeComponent instance
	instance := &appstacksv1beta1.RuntimeComponent{}
	var ba common.BaseComponent
	ba = instance
	err = r.GetClient().Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// initialize the RuntimeComponent instance
	instance.Initialize()

	_, err = appstacksutils.Validate(instance)
	// If there's any validation error, don't bother with requeuing
	if err != nil {
		reqLogger.Error(err, "Error validating RuntimeComponent")
		r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		return reconcile.Result{}, nil
	}

	if r.IsOpenShift() {
		// The order of items passed to the MergeMaps matters here! Annotations from GetOpenShiftAnnotations have higher importance. Otherwise,
		// it is not possible to override converted annotations.
		instance.Annotations = appstacksutils.MergeMaps(instance.Annotations, appstacksutils.GetOpenShiftAnnotations(instance))
	}

	currentGen := instance.Generation
	err = r.GetClient().Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Error updating RuntimeComponent")
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if currentGen == 1 {
		return reconcile.Result{}, nil
	}

	defaultMeta := metav1.ObjectMeta{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	}

	imageReferenceOld := instance.Status.ImageReference
	instance.Status.ImageReference = instance.Spec.ApplicationImage
	if r.IsOpenShift() {
		image, err := imageutil.ParseDockerImageReference(instance.Spec.ApplicationImage)
		if err == nil {
			isTag := &imagev1.ImageStreamTag{}
			isTagName := imageutil.JoinImageStreamTag(image.Name, image.Tag)
			isTagNamespace := image.Namespace
			if isTagNamespace == "" {
				isTagNamespace = instance.Namespace
			}
			key := types.NamespacedName{Name: isTagName, Namespace: isTagNamespace}
			err = r.GetAPIReader().Get(context.Background(), key, isTag)
			// Call ManageError only if the error type is not found or is not forbidden. Forbidden could happen
			// when the operator tries to call GET for ImageStreamTags on a namespace that doesn't exists (e.g.
			// cannot get imagestreamtags.image.openshift.io in the namespace "navidsh": no RBAC policy matched)
			if err == nil {
				image := isTag.Image
				if image.DockerImageReference != "" {
					instance.Status.ImageReference = image.DockerImageReference
				}
			} else if err != nil && !kerrors.IsNotFound(err) && !kerrors.IsForbidden(err) {
				return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
			}
		}
	}
	if imageReferenceOld != instance.Status.ImageReference {
		reqLogger.Info("Updating status.imageReference", "status.imageReference", instance.Status.ImageReference)
		err = r.UpdateStatus(instance)
		if err != nil {
			reqLogger.Error(err, "Error updating RuntimeComponent status")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
	}

	result, err := r.ReconcileProvides(instance)
	if err != nil || result != (reconcile.Result{}) {
		return result, err
	}

	result, err = r.ReconcileConsumes(instance)
	if err != nil || result != (reconcile.Result{}) {
		return result, err
	}
	result, err = r.ReconcileCertificate(instance)
	if err != nil || result != (reconcile.Result{}) {
		return result, nil
	}

	if r.IsServiceBindingSupported() {
		result, err = r.ReconcileBindings(instance)
		if err != nil || result != (reconcile.Result{}) {
			return result, err
		}
	} else if instance.Spec.Bindings != nil {
		return r.ManageError(errors.New("failed to reconcile as the operator failed to find Service Binding CRDs"), common.StatusConditionTypeReconciled, instance)
	}
	resolvedBindingSecret, err := r.GetResolvedBindingSecret(ba)
	if err != nil {
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if instance.Spec.ServiceAccountName == nil || *instance.Spec.ServiceAccountName == "" {
		serviceAccount := &corev1.ServiceAccount{ObjectMeta: defaultMeta}
		err = r.CreateOrUpdate(serviceAccount, instance, func() error {
			appstacksutils.CustomizeServiceAccount(serviceAccount, instance)
			return nil
		})
		if err != nil {
			reqLogger.Error(err, "Failed to reconcile ServiceAccount")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
	} else {
		serviceAccount := &corev1.ServiceAccount{ObjectMeta: defaultMeta}
		err = r.DeleteResource(serviceAccount)
		if err != nil {
			reqLogger.Error(err, "Failed to delete ServiceAccount")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
	}

	isKnativeSupported, err := r.IsGroupVersionSupported(servingv1alpha1.SchemeGroupVersion.String(), "Service")
	if err != nil {
		r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	} else if !isKnativeSupported {
		reqLogger.V(1).Info(fmt.Sprintf("%s is not supported on the cluster", servingv1alpha1.SchemeGroupVersion.String()))
	}

	if instance.Spec.CreateKnativeService != nil && *instance.Spec.CreateKnativeService {
		// Clean up non-Knative resources
		resources := []client.Object{
			&corev1.Service{ObjectMeta: defaultMeta},
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: instance.Name + "-headless", Namespace: instance.Namespace}},
			&appsv1.Deployment{ObjectMeta: defaultMeta},
			&appsv1.StatefulSet{ObjectMeta: defaultMeta},
			&autoscalingv1.HorizontalPodAutoscaler{ObjectMeta: defaultMeta},
		}
		err = r.DeleteResources(resources)
		if err != nil {
			reqLogger.Error(err, "Failed to clean up non-Knative resources")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}

		if ok, _ := r.IsGroupVersionSupported(networkingv1beta1.SchemeGroupVersion.String(), "Ingress"); ok {
			r.DeleteResource(&networkingv1beta1.Ingress{ObjectMeta: defaultMeta})
		}

		if r.IsOpenShift() {
			route := &routev1.Route{ObjectMeta: defaultMeta}
			err = r.DeleteResource(route)
			if err != nil {
				reqLogger.Error(err, "Failed to clean up non-Knative resource Route")
				return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
			}
		}

		if isKnativeSupported {
			ksvc := &servingv1alpha1.Service{ObjectMeta: defaultMeta}
			err = r.CreateOrUpdate(ksvc, instance, func() error {
				appstacksutils.CustomizeKnativeService(ksvc, instance)
				appstacksutils.CustomizeServiceBinding(resolvedBindingSecret, &ksvc.Spec.Template.Spec.PodSpec, instance)
				return nil
			})

			if err != nil {
				reqLogger.Error(err, "Failed to reconcile Knative Service")
				return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
			}
			return r.ManageSuccess(common.StatusConditionTypeReconciled, instance)
		}
		return r.ManageError(errors.New("failed to reconcile Knative service as operator could not find Knative CRDs"), common.StatusConditionTypeReconciled, instance)
	}

	if isKnativeSupported {
		ksvc := &servingv1alpha1.Service{ObjectMeta: defaultMeta}
		err = r.DeleteResource(ksvc)
		if err != nil {
			reqLogger.Error(err, "Failed to delete Knative Service")
			r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
	}

	svc := &corev1.Service{ObjectMeta: defaultMeta}
	err = r.CreateOrUpdate(svc, instance, func() error {
		appstacksutils.CustomizeService(svc, ba)
		svc.Annotations = appstacksutils.MergeMaps(svc.Annotations, instance.Spec.Service.Annotations)
		monitoringEnabledLabelName := getMonitoringEnabledLabelName(ba)
		if instance.Spec.Monitoring != nil {
			svc.Labels[monitoringEnabledLabelName] = "true"
		} else {
			if _, ok := svc.Labels[monitoringEnabledLabelName]; ok {
				delete(svc.Labels, monitoringEnabledLabelName)
			}
		}
		return nil
	})
	if err != nil {
		reqLogger.Error(err, "Failed to reconcile Service")
		return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	}

	if instance.Spec.Storage != nil {
		// Delete Deployment if exists
		deploy := &appsv1.Deployment{ObjectMeta: defaultMeta}
		err = r.DeleteResource(deploy)

		if err != nil {
			reqLogger.Error(err, "Failed to delete Deployment")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
		svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: instance.Name + "-headless", Namespace: instance.Namespace}}
		err = r.CreateOrUpdate(svc, instance, func() error {
			appstacksutils.CustomizeService(svc, instance)
			svc.Spec.ClusterIP = corev1.ClusterIPNone
			svc.Spec.Type = corev1.ServiceTypeClusterIP
			return nil
		})
		if err != nil {
			reqLogger.Error(err, "Failed to reconcile headless Service")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}

		statefulSet := &appsv1.StatefulSet{ObjectMeta: defaultMeta}
		err = r.CreateOrUpdate(statefulSet, instance, func() error {
			appstacksutils.CustomizeStatefulSet(statefulSet, instance)
			appstacksutils.CustomizePodSpec(&statefulSet.Spec.Template, instance)
			appstacksutils.CustomizePersistence(statefulSet, instance)
			appstacksutils.CustomizeServiceBinding(resolvedBindingSecret, &statefulSet.Spec.Template.Spec, instance)
			return nil
		})
		if err != nil {
			reqLogger.Error(err, "Failed to reconcile StatefulSet")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}

	} else {
		// Delete StatefulSet if exists
		statefulSet := &appsv1.StatefulSet{ObjectMeta: defaultMeta}
		err = r.DeleteResource(statefulSet)
		if err != nil {
			reqLogger.Error(err, "Failed to delete Statefulset")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}

		// Delete StatefulSet if exists
		headlesssvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: instance.Name + "-headless", Namespace: instance.Namespace}}
		err = r.DeleteResource(headlesssvc)

		if err != nil {
			reqLogger.Error(err, "Failed to delete headless Service")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
		deploy := &appsv1.Deployment{ObjectMeta: defaultMeta}
		err = r.CreateOrUpdate(deploy, instance, func() error {
			appstacksutils.CustomizeDeployment(deploy, instance)
			appstacksutils.CustomizePodSpec(&deploy.Spec.Template, instance)
			appstacksutils.CustomizeServiceBinding(resolvedBindingSecret, &deploy.Spec.Template.Spec, instance)
			return nil
		})
		if err != nil {
			reqLogger.Error(err, "Failed to reconcile Deployment")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}

	}

	if instance.Spec.Autoscaling != nil {
		hpa := &autoscalingv1.HorizontalPodAutoscaler{ObjectMeta: defaultMeta}
		err = r.CreateOrUpdate(hpa, instance, func() error {
			appstacksutils.CustomizeHPA(hpa, instance)
			return nil
		})

		if err != nil {
			reqLogger.Error(err, "Failed to reconcile HorizontalPodAutoscaler")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
	} else {
		hpa := &autoscalingv1.HorizontalPodAutoscaler{ObjectMeta: defaultMeta}
		err = r.DeleteResource(hpa)
		if err != nil {
			reqLogger.Error(err, "Failed to delete HorizontalPodAutoscaler")
			return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		}
	}

	if ok, err := r.IsGroupVersionSupported(routev1.SchemeGroupVersion.String(), "Route"); err != nil {
		reqLogger.Error(err, fmt.Sprintf("Failed to check if %s is supported", routev1.SchemeGroupVersion.String()))
		r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	} else if ok {
		if instance.Spec.Expose != nil && *instance.Spec.Expose {
			route := &routev1.Route{ObjectMeta: defaultMeta}
			err = r.CreateOrUpdate(route, instance, func() error {
				key, cert, caCert, destCACert, err := r.GetRouteTLSValues(ba)
				if err != nil {
					return err
				}
				appstacksutils.CustomizeRoute(route, ba, key, cert, caCert, destCACert)

				return nil
			})
			if err != nil {
				reqLogger.Error(err, "Failed to reconcile Route")
				return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
			}
		} else {
			route := &routev1.Route{ObjectMeta: defaultMeta}
			err = r.DeleteResource(route)
			if err != nil {
				reqLogger.Error(err, "Failed to delete Route")
				return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
			}
		}
	} else {

		if ok, err := r.IsGroupVersionSupported(networkingv1beta1.SchemeGroupVersion.String(), "Ingress"); err != nil {
			reqLogger.Error(err, fmt.Sprintf("Failed to check if %s is supported", networkingv1beta1.SchemeGroupVersion.String()))
			r.ManageError(err, common.StatusConditionTypeReconciled, instance)
		} else if ok {
			if instance.Spec.Expose != nil && *instance.Spec.Expose {
				ing := &networkingv1beta1.Ingress{ObjectMeta: defaultMeta}
				err = r.CreateOrUpdate(ing, instance, func() error {
					appstacksutils.CustomizeIngress(ing, instance)
					return nil
				})
				if err != nil {
					reqLogger.Error(err, "Failed to reconcile Ingress")
					return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
				}
			} else {
				ing := &networkingv1beta1.Ingress{ObjectMeta: defaultMeta}
				err = r.DeleteResource(ing)
				if err != nil {
					reqLogger.Error(err, "Failed to delete Ingress")
					return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
				}
			}
		}
	}

	if ok, err := r.IsGroupVersionSupported(prometheusv1.SchemeGroupVersion.String(), "ServiceMonitor"); err != nil {
		reqLogger.Error(err, fmt.Sprintf("Failed to check if %s is supported", prometheusv1.SchemeGroupVersion.String()))
		r.ManageError(err, common.StatusConditionTypeReconciled, instance)
	} else if ok {
		if instance.Spec.Monitoring != nil && (instance.Spec.CreateKnativeService == nil || !*instance.Spec.CreateKnativeService) {
			sm := &prometheusv1.ServiceMonitor{ObjectMeta: defaultMeta}
			err = r.CreateOrUpdate(sm, instance, func() error {
				appstacksutils.CustomizeServiceMonitor(sm, instance)
				return nil
			})
			if err != nil {
				reqLogger.Error(err, "Failed to reconcile ServiceMonitor")
				return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
			}
		} else {
			sm := &prometheusv1.ServiceMonitor{ObjectMeta: defaultMeta}
			err = r.DeleteResource(sm)
			if err != nil {
				reqLogger.Error(err, "Failed to delete ServiceMonitor")
				return r.ManageError(err, common.StatusConditionTypeReconciled, instance)
			}
		}

	} else {
		reqLogger.V(1).Info(fmt.Sprintf("%s is not supported", prometheusv1.SchemeGroupVersion.String()))
	}

	return r.ManageSuccess(common.StatusConditionTypeReconciled, instance)
}

// SetupWithManager initializes reconciler
func (r *RuntimeComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {

	mgr.GetFieldIndexer().IndexField(context.Background(), &appstacksv1beta1.RuntimeComponent{}, indexFieldImageStreamName, func(obj client.Object) []string {
		instance := obj.(*appstacksv1beta1.RuntimeComponent)
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
	mgr.GetFieldIndexer().IndexField(context.Background(), &appstacksv1beta1.RuntimeComponent{}, indexFieldBindingsResourceRef, func(obj client.Object) []string {
		instance := obj.(*appstacksv1beta1.RuntimeComponent)

		if instance.Spec.Bindings != nil && instance.Spec.Bindings.ResourceRef != "" {
			return []string{instance.Spec.Bindings.ResourceRef}
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

	var b *builder.Builder
	b = ctrl.NewControllerManagedBy(mgr).For(&appstacksv1beta1.RuntimeComponent{}, builder.WithPredicates(pred)).
		Owns(&corev1.Service{}, builder.WithPredicates(predSubResource)).
		Owns(&corev1.Secret{}, builder.WithPredicates(predSubResource)).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(predSubResWithGenCheck)).
		Owns(&appsv1.StatefulSet{}, builder.WithPredicates(predSubResWithGenCheck)).
		Owns(&autoscalingv1.HorizontalPodAutoscaler{}, builder.WithPredicates(predSubResource)).
		Watches(&source.Kind{Type: &corev1.Secret{}}, &EnqueueRequestsForCustomIndexField{
			Matcher: &BindingSecretMatcher{
				klient: mgr.GetClient(),
			},
		}).
		Watches(&source.Kind{Type: &corev1.Secret{}}, &utils.EnqueueRequestsForServiceBinding{
			Client:          mgr.GetClient(),
			GroupName:       "app.stacks",
			WatchNamespaces: watchNamespaces,
		})

	ok, _ := r.IsGroupVersionSupported(routev1.SchemeGroupVersion.String(), "Route")
	if ok {
		b = b.Owns(&routev1.Route{}, builder.WithPredicates(predSubResource))
	}
	ok, _ = r.IsGroupVersionSupported(networkingv1beta1.SchemeGroupVersion.String(), "Ingress")
	if ok {
		b = b.Owns(&networkingv1beta1.Ingress{}, builder.WithPredicates(predSubResource))
	}
	ok, _ = r.IsGroupVersionSupported(servingv1alpha1.SchemeGroupVersion.String(), "Service")
	if ok {
		b = b.Owns(&servingv1alpha1.Service{}, builder.WithPredicates(predSubResource))
	}
	ok, _ = r.IsGroupVersionSupported(certmngrv1alpha2.SchemeGroupVersion.String(), "Certificate")
	if ok {
		b = b.Owns(&certmngrv1alpha2.Certificate{}, builder.WithPredicates(predSubResource))
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

func getMonitoringEnabledLabelName(ba common.BaseComponent) string {
	return "monitor." + ba.GetGroupName() + "/enabled"
}
