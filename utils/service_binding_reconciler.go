package utils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/application-stacks/runtime-component-operator/common"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// String constants
const (
	ExposeBindingOverrideSecretSuffix = "-expose-binding-override"
	ExposeBindingSecretSuffix         = "-expose-binding"
)

// ReconcileBindings goes through the reconcile logic for service binding
func (r *ReconcilerBase) ReconcileBindings(ba common.BaseComponent) error {
	if err := r.reconcileExpose(ba); err != nil {
		return err
	}
	return nil
}

func (r *ReconcilerBase) reconcileExpose(ba common.BaseComponent) error {
	mObj := ba.(metav1.Object)
	bindingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getExposeBindingSecretName(ba),
			Namespace: mObj.GetNamespace(),
		},
	}

	if ba.GetService() != nil && ba.GetService().GetBindable() != nil && *ba.GetService().GetBindable() {
		err := r.CreateOrUpdate(bindingSecret, mObj, func() error {
			customSecret := &corev1.Secret{}
			// Check if custom values are provided in a secret, and apply the custom values
			if err := r.getCustomValuesToExpose(customSecret, ba); err != nil {
				return err
			}
			// Use content of the 'override' secret as the base secret content
			bindingSecret.Data = customSecret.Data
			// Apply default values to the override secret if certain values are not set
			r.applyDefaultValuesToExpose(bindingSecret, ba)
			return nil
		})
		if err != nil {
			return err
		}

		// Update binding status
		r.updateBindingStatus(bindingSecret.Name, ba)
		return nil
	}

	// Update status
	r.updateBindingStatus("", ba)
	// Remove binding secret
	if err := r.DeleteResource(bindingSecret); client.IgnoreNotFound(err) != nil {
		return err
	}
	return nil
}

func (r *ReconcilerBase) getCustomValuesToExpose(secret *corev1.Secret, ba common.BaseComponent) error {
	mObj := ba.(metav1.Object)
	key := types.NamespacedName{Name: getOverrideExposeBindingSecretName(ba), Namespace: mObj.GetNamespace()}
	err := r.GetClient().Get(context.TODO(), key, secret)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	return nil
}

func (r *ReconcilerBase) applyDefaultValuesToExpose(secret *corev1.Secret, ba common.BaseComponent) {
	mObj := ba.(metav1.Object)
	secret.Labels = ba.GetLabels()
	secret.Annotations = MergeMaps(secret.Annotations, ba.GetAnnotations())

	secretData := secret.Data
	if secretData == nil {
		secretData = map[string][]byte{}
	}
	var host, protocol, basePath, port []byte
	var found bool
	if host, found = secretData["host"]; !found {
		host = []byte(fmt.Sprintf("%s.%s.svc.cluster.local", mObj.GetName(), mObj.GetNamespace()))
		secretData["host"] = host
	}
	if protocol, found = secretData["protocol"]; !found {
		if ba.GetManageTLS() == nil || *ba.GetManageTLS() {
			protocol = []byte("https")

		} else {
			protocol = []byte("http")
		}
		secretData["protocol"] = protocol
	}
	if basePath, found = secretData["basePath"]; !found {
		basePath = []byte("/")
		secretData["basePath"] = basePath
	}
	if port, found = secretData["port"]; !found {
		if ba.GetCreateKnativeService() == nil || !*ba.GetCreateKnativeService() {
			port = []byte(strconv.Itoa(int(ba.GetService().GetPort())))
		}
		secretData["port"] = port
	}
	if _, found = secretData["uri"]; !found {
		uri := []byte(fmt.Sprintf("%s://%s", protocol, host))
		portStr := string(port)
		if portStr != "" {
			uri = []byte(fmt.Sprintf("%s:%s", uri, portStr))
		}
		basePathStr := string(basePath)
		if basePathStr != "" {
			basePathStr = strings.TrimPrefix(basePathStr, "/")
			uri = []byte(fmt.Sprintf("%s/%s", uri, basePathStr))
		}
		secretData["uri"] = uri
	}

	if _, found = secretData["certificates"]; !found && ba.GetStatus().GetReferences()[common.StatusReferenceCertSecretName] != "" {

		certSecret := &corev1.Secret{}
		err := r.GetClient().Get(context.TODO(), types.NamespacedName{Name: ba.GetStatus().GetReferences()[common.StatusReferenceCertSecretName], Namespace: mObj.GetNamespace()}, certSecret)
		if err == nil {
			caCert := certSecret.Data["ca.crt"]
			tlsCrt := certSecret.Data["tls.crt"]
			chain := string(tlsCrt) + string(caCert)
			if chain != "" {
				secretData["certificates"] = []byte(chain)
			}
		}
	}

	if _, found = secretData["ingress-uri"]; !found && ba.GetExpose() != nil && *ba.GetExpose() {
		if ok, err := r.IsGroupVersionSupported(routev1.SchemeGroupVersion.String(), "Route"); err != nil {
			r.ManageError(err, common.StatusConditionTypeReconciled, ba)
		} else if ok {
			route := &routev1.Route{}
			r.GetClient().Get(context.Background(), types.NamespacedName{Name: mObj.GetName(), Namespace: mObj.GetNamespace()}, route)
			routeHost := route.Spec.Host
			routePath := route.Spec.Path
			if route.Spec.TLS != nil {
				secretData["ingress-uri"] = []byte(fmt.Sprintf("%s://%s%s%s", "https", routeHost, routePath, string(basePath)))

			} else {
				secretData["ingress-uri"] = []byte(fmt.Sprintf("%s://%s%s%s", "http", routeHost, routePath, string(basePath)))
			}
		} else {
			if ok, err := r.IsGroupVersionSupported(networkingv1.SchemeGroupVersion.String(), "Ingress"); err != nil {
				r.ManageError(err, common.StatusConditionTypeReconciled, ba)
			} else if ok {
				ingress := &networkingv1.Ingress{}
				r.GetClient().Get(context.Background(), types.NamespacedName{Name: mObj.GetName(), Namespace: mObj.GetNamespace()}, ingress)
				if len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].Host != "" {
					host := ingress.Spec.Rules[0].Host
					if len(ingress.Spec.TLS) > 0 && len(ingress.Spec.TLS[0].Hosts) > 0 && ingress.Spec.TLS[0].Hosts[0] != "" {
						secretData["ingress-uri"] = []byte(fmt.Sprintf("%s://%s%s", "https", host, string(basePath)))

					} else {
						secretData["ingress-uri"] = []byte(fmt.Sprintf("%s://%s%s", "http", host, string(basePath)))

					}
				}
			}
		}
	}
	secret.Data = secretData
}

func (r *ReconcilerBase) updateBindingStatus(bindingSecretName string, ba common.BaseComponent) {
	var bindingStatus *corev1.LocalObjectReference
	if bindingSecretName != "" {
		bindingStatus = &corev1.LocalObjectReference{Name: bindingSecretName}
	}
	ba.GetStatus().SetBinding(bindingStatus)
}

func getOverrideExposeBindingSecretName(ba common.BaseComponent) string {
	return (ba.(metav1.Object)).GetName() + ExposeBindingOverrideSecretSuffix
}

func getExposeBindingSecretName(ba common.BaseComponent) string {
	return (ba.(metav1.Object)).GetName() + ExposeBindingSecretSuffix
}
