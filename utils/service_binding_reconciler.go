package utils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/application-stacks/runtime-component-operator/common"
	"github.com/awnumar/memguard"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	bindingSecret, err := common.GetSecret(r.GetClient(), getExposeBindingSecretName(ba), mObj.GetNamespace())
	defer bindingSecret.Destroy()
	if err != nil {
		return err
	}

	if ba.GetService() != nil && ba.GetService().GetBindable() != nil && *ba.GetService().GetBindable() {
		customSecret := &common.LockedBufferSecret{}
		defer customSecret.Destroy()
		// Check if custom values are provided in a secret, and apply the custom values
		err := r.getCustomValuesToExpose(customSecret, ba)
		if err != nil {
			return err
		}
		// Use content of the 'override' secret as the base secret content
		bindingSecret.LockedData = customSecret.LockedData
		customSecret.LockedData = nil
		// Apply default values to the override secret if certain values are not set
		r.applyDefaultValuesToExpose(recCtx, bindingSecret, ba)

		if err := r.CreateOrUpdateSecret(bindingSecret, mObj); err != nil {
			return err
		}

		// Update binding status
		r.updateBindingStatus(bindingSecret.Name, ba)
		return nil
	}

	// Update status
	r.updateBindingStatus("", ba)
	// Remove binding secret
	if err := r.DeleteSecretResource(bindingSecret); client.IgnoreNotFound(err) != nil {
		return err
	}
	return nil
}

func (r *ReconcilerBase) getCustomValuesToExpose(secret *common.LockedBufferSecret, ba common.BaseComponent) error {
	mObj := ba.(metav1.Object)
	overrideExposeBindingSecret, err := common.GetSecret(r.GetClient(), getOverrideExposeBindingSecretName(ba), mObj.GetNamespace())
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if overrideExposeBindingSecret != nil {
		secret = overrideExposeBindingSecret
	}
	return nil
}

func (r *ReconcilerBase) applyDefaultValuesToExpose(recCtx context.Context, secret *common.LockedBufferSecret, ba common.BaseComponent) {
	mObj := ba.(metav1.Object)
	secret.Labels = ba.GetLabels()
	secret.Annotations = MergeMaps(secret.Annotations, ba.GetAnnotations())

	if secret.LockedData == nil {
		secret.LockedData = common.SecretMap{}
	}
	secretData := secret.LockedData
	var host, protocol, basePath, port []byte
	var found bool
	if host, found := secretData.Get("host"); !found {
		host = []byte(fmt.Sprintf("%s.%s.svc.cluster.local", mObj.GetName(), mObj.GetNamespace()))
		secretData["host"] = memguard.NewBufferFromBytes(host)
	}
	if protocol, found = secretData.Get("protocol"); !found {
		if ba.GetManageTLS() == nil || *ba.GetManageTLS() {
			protocol = []byte("https")

		} else {
			protocol = []byte("http")
		}
		secretData["protocol"] = memguard.NewBufferFromBytes(protocol)
	}
	if basePath, found = secretData.Get("basePath"); !found {
		basePath = []byte("/")
		secretData["basePath"] = memguard.NewBufferFromBytes(basePath)
	}
	if port, found = secretData.Get("port"); !found {
		if ba.GetCreateKnativeService() == nil || !*ba.GetCreateKnativeService() {
			port = []byte(strconv.Itoa(int(ba.GetService().GetPort())))
		}
		secretData["port"] = memguard.NewBufferFromBytes(port)
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
		secretData["uri"] = memguard.NewBufferFromBytes(uri)
	}

	if _, found = secretData["certificates"]; !found && ba.GetStatus().GetReferences()[common.StatusReferenceCertSecretName] != "" {
		certSecret, err := common.GetSecret(r.GetClient(), ba.GetStatus().GetReferences()[common.StatusReferenceCertSecretName], mObj.GetNamespace())
		defer certSecret.LockedData.Destroy()
		if err == nil {
			caCert, _ := certSecret.LockedData.Get("ca.crt")
			tlsCrt, _ := certSecret.LockedData.Get("tls.crt")

			chainedCerts := make([]byte, len(caCert)+len(tlsCrt))
			nCount := copy(chainedCerts, tlsCrt)
			nCount += copy(chainedCerts[len(tlsCrt):], caCert)
			if nCount > 0 {
				secretData["certificates"] = memguard.NewBufferFromBytes(chainedCerts)
			}
		}
	}

	if _, found = secretData["ingress-uri"]; !found && ba.GetExpose() != nil && *ba.GetExpose() {
		host, path, protocol := r.GetIngressInfo(ba)
		secretData["ingress-uri"] = memguard.NewBufferFromBytes([]byte(fmt.Sprintf("%s://%s%s%s", protocol, host, path, string(basePath))))
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
