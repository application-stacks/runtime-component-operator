package utils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/application-stacks/runtime-component-operator/common"
	corev1 "k8s.io/api/core/v1"
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
func (r *ReconcilerBase) ReconcileBindings(recCtx context.Context, ba common.BaseComponent) error {
	if err := r.reconcileExpose(recCtx, ba); err != nil {
		return err
	}
	return nil
}

func (r *ReconcilerBase) reconcileExpose(recCtx context.Context, ba common.BaseComponent) error {
	mObj := ba.(metav1.Object)
	bindingSecret := common.NewSecret(recCtx, getExposeBindingSecretName(ba), mObj.GetNamespace())

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
			r.applyDefaultValuesToExpose(recCtx, bindingSecret, ba)
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

func (r *ReconcilerBase) applyDefaultValuesToExpose(recCtx context.Context, secret *corev1.Secret, ba common.BaseComponent) {
	mObj := ba.(metav1.Object)
	secret.Labels = ba.GetLabels()
	secret.Annotations = MergeMaps(secret.Annotations, ba.GetAnnotations())

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secretData := secret.Data
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
		certSecret := common.NewSecret(recCtx, ba.GetStatus().GetReferences()[common.StatusReferenceCertSecretName], mObj.GetNamespace())
		err := r.GetClient().Get(context.TODO(), types.NamespacedName{Name: certSecret.Name, Namespace: certSecret.Namespace}, certSecret)
		if err == nil {
			caCert := certSecret.Data["ca.crt"]
			tlsCrt := certSecret.Data["tls.crt"]
			chainedCerts := make([]byte, len(caCert)+len(tlsCrt))
			nCount := copy(chainedCerts, tlsCrt)
			nCount += copy(chainedCerts[len(tlsCrt):], caCert)
			if nCount > 0 {
				secretData["certificates"] = chainedCerts
			}
		}
	}

	if _, found = secretData["ingress-uri"]; !found && ba.GetExpose() != nil && *ba.GetExpose() {
		host, path, protocol := r.GetIngressInfo(ba)
		secretData["ingress-uri"] = []byte(fmt.Sprintf("%s://%s%s%s", protocol, host, path, string(basePath)))
	}
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
