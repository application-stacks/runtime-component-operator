package utils

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/application-stacks/runtime-component-operator/common"
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
	"k8s.io/apimachinery/pkg/types"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateConfigMap creates or updates ConfigMap resource
func (r *ReconcilerBase) UpdateConfigMap(OperatorName string, ns string) {
	configMap, err := r.GetOpConfigMap(OperatorName, ns)
	if err != nil {
		log.Info("Failed to get " + OperatorName + " config map, error: " + err.Error())
		CreateConfigMap(OperatorName)
	} else {
		common.Config.LoadFromConfigMap(configMap)
	}
}

// UpdateImageStreamTag updates image reference
func (r *ReconcilerBase) UpdateImageReference(ba common.BaseComponent) error {
	status, metaObj := ba.GetStatus(), ba.(metav1.Object)

	status.SetImageReference(ba.GetApplicationImage())
	if r.IsOpenShift() {
		image, err := imageutil.ParseDockerImageReference(ba.GetApplicationImage())
		if err == nil {
			isTag := &imagev1.ImageStreamTag{}
			isTagName := imageutil.JoinImageStreamTag(image.Name, image.Tag)
			isTagNamespace := image.Namespace
			if isTagNamespace == "" {
				isTagNamespace = metaObj.GetNamespace()
			}
			key := types.NamespacedName{Name: isTagName, Namespace: isTagNamespace}
			err = r.GetAPIReader().Get(context.Background(), key, isTag)
			// Call ManageError only if the error type is not found or is not forbidden. Forbidden could happen
			// when the operator tries to call GET for ImageStreamTags on a namespace that doesn't exists (e.g.
			// cannot get imagestreamtags.image.openshift.io in the namespace "navidsh": no RBAC policy matched)
			if err == nil {
				image := isTag.Image
				if image.DockerImageReference != "" {
					status.SetImageReference(image.DockerImageReference)
				}
			} else if err != nil && !kerrors.IsNotFound(err) && !kerrors.IsForbidden(err) && !strings.Contains(isTagName, "/") {
				return err
			}
		}
	}

	return nil
}

// UpdateServiceAccount creates or updates Services Account resource
func (r *ReconcilerBase) UpdateServiceAccount(ba common.BaseComponent, defaultMeta metav1.ObjectMeta) error {
	metaObj := ba.(metav1.Object)

	if GetServiceAccountName(ba) == "" {
		serviceAccount := &corev1.ServiceAccount{ObjectMeta: defaultMeta}
		err := r.CreateOrUpdate(serviceAccount, metaObj, func() error {
			return CustomizeServiceAccount(serviceAccount, ba, r.GetClient())
		})
		if err != nil {
			log.Error(err, "Failed to reconcile ServiceAccount")
			return err
		}
	} else {
		// delete our SA, as one has been specified
		serviceAccount := &corev1.ServiceAccount{ObjectMeta: defaultMeta}
		if err := r.DeleteResource(serviceAccount); err != nil {
			log.Error(err, "Failed to delete ServiceAccount")
			return err
		}
	}
	// Check if the ServiceAccount has a valid pull secret before creating the deployment/statefulset
	// or setting up knative. Otherwise the pods can go into an ImagePullBackOff loop
	if saErr := ServiceAccountPullSecretExists(ba, r.GetClient()); saErr != nil {
		return saErr
	}

	return nil
}

// UpdateKnativeService creates or updates Knative Service resource
func (r *ReconcilerBase) UpdateKnativeService(ba common.BaseComponent, defaultMeta metav1.ObjectMeta, isKnativeSupported bool, createKnativeService bool) error {
	metaObj := ba.(metav1.Object)

	// When Knative is supported and Knative serving is being used
	if createKnativeService {

		// Clean up non-Knative resources
		resources := []client.Object{
			&corev1.Service{ObjectMeta: defaultMeta},
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: metaObj.GetName() + "-headless", Namespace: metaObj.GetNamespace()}},
			&appsv1.Deployment{ObjectMeta: defaultMeta},
			&appsv1.StatefulSet{ObjectMeta: defaultMeta},
			&autoscalingv1.HorizontalPodAutoscaler{ObjectMeta: defaultMeta},
			&networkingv1.NetworkPolicy{ObjectMeta: defaultMeta},
		}
		if err := r.DeleteResources(resources); err != nil {
			log.Error(err, "Failed to clean up non-Knative resources")
			return err
		}

		if ok, _ := r.IsGroupVersionSupported(networkingv1.SchemeGroupVersion.String(), "Ingress"); ok {
			r.DeleteResource(&networkingv1.Ingress{ObjectMeta: defaultMeta})
		}

		if r.IsOpenShift() {
			route := &routev1.Route{ObjectMeta: defaultMeta}
			if err := r.DeleteResource(route); err != nil {
				log.Error(err, "Failed to clean up non-Knative resource Route")
				return err
			}
		}

		if isKnativeSupported {
			log.Info("Knative is supported and Knative Service is enabled")
			ksvc := &servingv1.Service{ObjectMeta: defaultMeta}
			err := r.CreateOrUpdate(ksvc, metaObj, func() error {
				CustomizeKnativeService(ksvc, ba)
				return nil
			})
			if err != nil {
				log.Error(err, "Failed to reconcile Knative Service")
				return err
			}
			return nil
		}

		return errors.New("failed to reconcile Knative service as operator could not find Knative CRDs")
	}

	if isKnativeSupported {
		ksvc := &servingv1.Service{ObjectMeta: defaultMeta}
		if err := r.DeleteResource(ksvc); err != nil {
			log.Error(err, "Failed to delete Knative Service")
			r.ManageError(err, common.StatusConditionTypeReconciled, ba)
		}
	}

	return nil
}

// UpdateSvcCertSecret creates or updates Service Cert secret
func (r *ReconcilerBase) UpdateSvcCertSecret(ba common.BaseComponent, prefix string, CACommonName string, operatorName string) (bool, error) {
	useCertmanager, err := r.GenerateSvcCertSecret(ba, prefix, CACommonName, operatorName)
	if err != nil {
		log.Error(err, "Failed to reconcile CertManager Certificate")
		return useCertmanager, err
	}

	if ba.GetService().GetCertificateSecretRef() != nil {
		ba.GetStatus().SetReference(common.StatusReferenceCertSecretName, *ba.GetService().GetCertificateSecretRef())
	}

	return useCertmanager, nil
}

// UpdateService creates or updates Service resource
func (r *ReconcilerBase) UpdateService(ba common.BaseComponent, defaultMeta metav1.ObjectMeta, useCertmanager bool) error {
	metaObj := ba.(metav1.Object)

	svc := &corev1.Service{ObjectMeta: defaultMeta}
	err := r.CreateOrUpdate(svc, metaObj, func() error {
		CustomizeService(svc, ba)
		svc.Annotations = MergeMaps(svc.Annotations, ba.GetAnnotations())
		if !useCertmanager && r.IsOpenShift() {
			AddOCPCertAnnotation(ba, svc)
		}
		monitoringEnabledLabelName := getMonitoringEnabledLabelName(ba)
		if ba.GetMonitoring() != nil {
			svc.Labels[monitoringEnabledLabelName] = "true"
		} else {
			delete(svc.Labels, monitoringEnabledLabelName)
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to reconcile Service")
		return err
	}

	return nil
}

// UpdateTLSReference creates or updates TLS reference in status field
func (r *ReconcilerBase) UpdateTLSReference(ba common.BaseComponent) error {
	if (ba.GetManageTLS() == nil || *ba.GetManageTLS()) &&
		ba.GetStatus().GetReferences()[common.StatusReferenceCertSecretName] == "" {
		return errors.New("failed to generate TLS certificate; ensure cert-manager is installed and running")
	}
	return nil
}

// UpdateNetworkPolicy creates or updates Network Policy resource
func (r *ReconcilerBase) UpdateNetworkPolicy(ba common.BaseComponent, defaultMeta metav1.ObjectMeta) error {
	metaObj := ba.(metav1.Object)

	networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: defaultMeta}
	if np := ba.GetNetworkPolicy(); np == nil || np != nil && !np.IsDisabled() {
		err := r.CreateOrUpdate(networkPolicy, metaObj, func() error {
			CustomizeNetworkPolicy(networkPolicy, r.IsOpenShift(), ba)
			return nil
		})
		if err != nil {
			log.Error(err, "Failed to reconcile network policy")
			return err
		}
	} else {
		if err := r.DeleteResource(networkPolicy); err != nil {
			log.Error(err, "Failed to delete network policy")
			return err
		}
	}

	return nil
}

// UpdateStatefulSet creates or updates StatefulSet resource
func (r *ReconcilerBase) UpdateStatefulSet(ba common.BaseComponent, defaultMeta metav1.ObjectMeta) error {
	metaObj := ba.(metav1.Object)

	// Delete Deployment if exists
	deploy := &appsv1.Deployment{ObjectMeta: defaultMeta}
	if err := r.DeleteResource(deploy); err != nil {
		log.Error(err, "Failed to delete Deployment")
		return err
	}

	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: metaObj.GetName() + "-headless", Namespace: metaObj.GetNamespace()}}
	err := r.CreateOrUpdate(svc, metaObj, func() error {
		CustomizeService(svc, ba)
		svc.Spec.ClusterIP = corev1.ClusterIPNone
		svc.Spec.Type = corev1.ServiceTypeClusterIP
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to reconcile headless Service")
		return err
	}

	statefulSet := &appsv1.StatefulSet{ObjectMeta: defaultMeta}
	err = r.CreateOrUpdate(statefulSet, metaObj, func() error {
		CustomizeStatefulSet(statefulSet, ba)
		CustomizePodSpec(&statefulSet.Spec.Template, ba)
		if err := CustomizePodWithSVCCertificate(&statefulSet.Spec.Template, ba, r.GetClient()); err != nil {
			return err
		}
		CustomizePersistence(statefulSet, ba)
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to reconcile StatefulSet")
		return err
	}

	return nil
}

// UpdateDeploymentReq creates or update Deployment resource
func (r *ReconcilerBase) UpdateDeployment(ba common.BaseComponent, defaultMeta metav1.ObjectMeta) error {
	metaObj := ba.(metav1.Object)

	// Delete StatefulSet if exists
	statefulSet := &appsv1.StatefulSet{ObjectMeta: defaultMeta}
	if err := r.DeleteResource(statefulSet); err != nil {
		log.Error(err, "Failed to delete Statefulset")
		return err
	}

	// Delete Headless Service if exists
	headlesssvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: metaObj.GetName() + "-headless", Namespace: metaObj.GetNamespace()}}
	if err := r.DeleteResource(headlesssvc); err != nil {
		log.Error(err, "Failed to delete headless Service")
		return err
	}

	deploy := &appsv1.Deployment{ObjectMeta: defaultMeta}
	err := r.CreateOrUpdate(deploy, metaObj, func() error {
		CustomizeDeployment(deploy, ba)
		CustomizePodSpec(&deploy.Spec.Template, ba)
		if err := CustomizePodWithSVCCertificate(&deploy.Spec.Template, ba, r.GetClient()); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Failed to reconcile Deployment")
		return err
	}

	return nil
}

// UpdateAutoscaling creates or updates HPA resource
func (r *ReconcilerBase) UpdateAutoscaling(ba common.BaseComponent, defaultMeta metav1.ObjectMeta) error {
	metaObj := ba.(metav1.Object)

	if ba.GetAutoscaling() != nil {
		hpa := &autoscalingv1.HorizontalPodAutoscaler{ObjectMeta: defaultMeta}
		err := r.CreateOrUpdate(hpa, metaObj, func() error {
			CustomizeHPA(hpa, ba)
			return nil
		})

		if err != nil {
			log.Error(err, "Failed to reconcile HorizontalPodAutoscaler")
			return err
		}
	} else {
		hpa := &autoscalingv1.HorizontalPodAutoscaler{ObjectMeta: defaultMeta}
		if err := r.DeleteResource(hpa); err != nil {
			log.Error(err, "Failed to delete HorizontalPodAutoscaler")
			return err
		}
	}

	return nil
}

// UpdateRouteOrIngress creates or updates Route resource if supported, otherwise Ingress resource
func (r *ReconcilerBase) UpdateRouteOrIngress(ba common.BaseComponent, defaultMeta metav1.ObjectMeta) error {
	// Check if Route is supported
	if ok, err := r.IsGroupVersionSupported(routev1.SchemeGroupVersion.String(), "Route"); err != nil {
		log.Error(err, fmt.Sprintf("Failed to check if %s is supported", routev1.SchemeGroupVersion.String()))
		r.ManageError(err, common.StatusConditionTypeReconciled, ba)
	} else if ok {
		if err = r.UpdateRoute(ba, defaultMeta); err != nil {
			return err
		}
	} else {
		// If Route is not supported, check if Ingress is supported
		if ok, err := r.IsGroupVersionSupported(networkingv1.SchemeGroupVersion.String(), "Ingress"); err != nil {
			log.Error(err, fmt.Sprintf("Failed to check if %s is supported", networkingv1.SchemeGroupVersion.String()))
			r.ManageError(err, common.StatusConditionTypeReconciled, ba)
		} else if ok {
			if err = r.UpdateIngress(ba, defaultMeta); err != nil {
				return err
			}
		} else {
			log.V(1).Info(fmt.Sprintf("%s is not supported", networkingv1.SchemeGroupVersion.String()))
		}
	}

	return nil
}

// UpdateRoute creates or updates Route resource
func (r *ReconcilerBase) UpdateRoute(ba common.BaseComponent, defaultMeta metav1.ObjectMeta) error {
	metaObj := ba.(metav1.Object)

	if ba.GetExpose() != nil && *ba.GetExpose() {
		route := &routev1.Route{ObjectMeta: defaultMeta}
		err := r.CreateOrUpdate(route, metaObj, func() error {
			key, cert, caCert, destCACert, err := r.GetRouteTLSValues(ba)
			if err != nil {
				return err
			}
			CustomizeRoute(route, ba, key, cert, caCert, destCACert)
			return nil
		})
		if err != nil {
			log.Error(err, "Failed to reconcile Route")
			return err
		}
	} else {
		route := &routev1.Route{ObjectMeta: defaultMeta}
		if err := r.DeleteResource(route); err != nil {
			log.Error(err, "Failed to delete Route")
			return err
		}
	}

	return nil
}

// UpdateIngress creates or updates Ingress resource
func (r *ReconcilerBase) UpdateIngress(ba common.BaseComponent, defaultMeta metav1.ObjectMeta) error {
	metaObj := ba.(metav1.Object)

	if ba.GetExpose() != nil && *ba.GetExpose() {
		ing := &networkingv1.Ingress{ObjectMeta: defaultMeta}
		err := r.CreateOrUpdate(ing, metaObj, func() error {
			CustomizeIngress(ing, ba)
			return nil
		})
		if err != nil {
			log.Error(err, "Failed to reconcile Ingress")
			return err
		}
	} else {
		ing := &networkingv1.Ingress{ObjectMeta: defaultMeta}
		if err := r.DeleteResource(ing); err != nil {
			log.Error(err, "Failed to delete Ingress")
			return err
		}
	}

	return nil
}

// UpdateIngress creates or updates Service Monitor resource
func (r *ReconcilerBase) UpdateServiceMonitor(ba common.BaseComponent, defaultMeta metav1.ObjectMeta) error {
	if ok, err := r.IsGroupVersionSupported(prometheusv1.SchemeGroupVersion.String(), "ServiceMonitor"); err != nil {
		log.Error(err, fmt.Sprintf("Failed to check if %s is supported", prometheusv1.SchemeGroupVersion.String()))
		r.ManageError(err, common.StatusConditionTypeReconciled, ba)
	} else if ok {
		metaObj := ba.(metav1.Object)

		if ba.GetMonitoring() != nil && (ba.GetCreateKnativeService() == nil || !*ba.GetCreateKnativeService()) {
			// Validate the monitoring endpoints' configuration before creating/updating the ServiceMonitor
			if err := ValidatePrometheusMonitoringEndpoints(ba, r.GetClient(), metaObj.GetNamespace()); err != nil {
				return err
			}

			sm := &prometheusv1.ServiceMonitor{ObjectMeta: defaultMeta}
			err = r.CreateOrUpdate(sm, metaObj, func() error {
				CustomizeServiceMonitor(sm, ba)
				return nil
			})
			if err != nil {
				log.Error(err, "Failed to reconcile ServiceMonitor")
				return err
			}
		} else {
			sm := &prometheusv1.ServiceMonitor{ObjectMeta: defaultMeta}
			if err = r.DeleteResource(sm); err != nil {
				log.Error(err, "Failed to delete ServiceMonitor")
				return err
			}
		}
	} else {
		log.V(1).Info(fmt.Sprintf("%s is not supported", prometheusv1.SchemeGroupVersion.String()))
	}

	return nil
}

func getMonitoringEnabledLabelName(ba common.BaseComponent) string {
	return "monitor." + ba.GetGroupName() + "/enabled"
}
