package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/application-stacks/runtime-component-operator/common"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// String constants
const (
	APIVersion                        = "apiVersion"
	Kind                              = "kind"
	Metadata                          = "metadata"
	Spec                              = "spec"
	ExposeBindingOverrideSecretSuffix = "-expose-binding-override"
	ExposeBindingSecretSuffix         = "-expose-binding"
)

// ReconcileBindings goes through the reconcile logic for service binding
func (r *ReconcilerBase) ReconcileBindings(ba common.BaseComponent) (reconcile.Result, error) {
	if res, err := r.reconcileExpose(ba); isRequeue(res, err) {
		return res, err
	}
	if ba.GetBindings() != nil && ba.GetBindings().GetEmbedded() != nil {
		if res, err := r.reconcileEmbedded(ba); isRequeue(res, err) {
			return res, err
		}
		return r.done(ba)
	}
	if err := r.cleanUpEmbeddedBindings(ba); err != nil {
		return r.requeueError(ba, err)
	}
	if res, err := r.reconcileExternals(ba); isRequeue(res, err) {
		return res, err
	}
	return r.done(ba)
}

func (r *ReconcilerBase) reconcileExpose(ba common.BaseComponent) (reconcile.Result, error) {
	mObj := ba.(metav1.Object)
	bindingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getExposeBindingSecretName(ba),
			Namespace: mObj.GetNamespace(),
		},
	}

	if ba.GetBindings() != nil && ba.GetBindings().GetExpose() != nil &&
		ba.GetBindings().GetExpose().GetEnabled() != nil && *ba.GetBindings().GetExpose().GetEnabled() {
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
			return r.requeueError(ba, err)
		}

		// Update binding status
		r.updateBindingStatus(bindingSecret.Name, ba)
		return r.done(ba)
	}

	// Update status
	r.updateBindingStatus("", ba)
	// Remove binding secret
	if err := r.DeleteResource(bindingSecret); client.IgnoreNotFound(err) != nil {
		return r.requeueError(ba, err)
	}
	return r.done(ba)
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
		protocol = []byte("http")
		secretData["protocol"] = protocol
	}
	if basePath, found = secretData["basePath"]; !found {
		basePath = []byte("/")
		secretData["basePath"] = basePath
	}
	if port, found = secretData["port"]; !found {
		if ba.GetCreateKnativeService() == nil || *(ba.GetCreateKnativeService()) == false {
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

	secret.Data = secretData
}

func (r *ReconcilerBase) updateBindingStatus(bindingSecretName string, ba common.BaseComponent) {
	var bindingStatus *corev1.LocalObjectReference
	if bindingSecretName != "" {
		bindingStatus = &corev1.LocalObjectReference{Name: bindingSecretName}
	}
	ba.GetStatus().SetBinding(bindingStatus)
}

func (r *ReconcilerBase) reconcileExternals(ba common.BaseComponent) (retRes reconcile.Result, retErr error) {
	mObj := ba.(metav1.Object)
	var resolvedBindings []string

	if ba.GetBindings() != nil && ba.GetBindings().GetResourceRef() != "" {
		bindingName := ba.GetBindings().GetResourceRef()
		key := types.NamespacedName{Name: bindingName, Namespace: mObj.GetNamespace()}
		bindingSecret := &corev1.Secret{}
		err := r.GetClient().Get(context.TODO(), key, bindingSecret)
		if err == nil {
			resolvedBindings = append(resolvedBindings, bindingName)
		} else {
			err = errors.Wrapf(err, "service binding dependency not satisfied: unable to find service binding secret for external binding %q in namespace %q", bindingName, mObj.GetNamespace())
			return r.requeueError(ba, err)
		}
	} else if ba.GetBindings() == nil || ba.GetBindings().GetAutoDetect() == nil || *ba.GetBindings().GetAutoDetect() {
		bindingName := getDefaultServiceBindingName(ba)
		key := types.NamespacedName{Name: bindingName, Namespace: mObj.GetNamespace()}

		for _, gvk := range r.getServiceBindingGVK() {
			// Using a unstructured object to find ServiceBinding CR since GVK might change
			bindingObj := &unstructured.Unstructured{}
			bindingObj.SetGroupVersionKind(gvk)
			err := r.client.Get(context.Background(), key, bindingObj)
			if err != nil {
				if !kerrors.IsNotFound(err) {
					log.Error(errors.Wrapf(err, "failed to find a service binding resource during auto-detect for GVK %q", gvk), "failed to get Service Binding CR")
				}
				continue
			}

			bindingSecret := &corev1.Secret{}
			err = r.GetClient().Get(context.TODO(), key, bindingSecret)
			if err == nil {
				resolvedBindings = append(resolvedBindings, bindingName)
				break
			} else {
				err = errors.Wrapf(err, "service binding dependency not satisfied: unable to find service binding secret for external binding %q in namespace %q", bindingName, mObj.GetNamespace())
				return r.requeueError(ba, err)
			}
		}
	}

	retRes, retErr = r.done(ba)
	defer func() {
		if res, err := r.updateResolvedBindingStatus(resolvedBindings, ba); isRequeue(res, err) {
			retRes, retErr = res, err
		}
	}()
	return
}

//GetResolvedBindingSecret returns the secret referenced in .status.resolvedBindings
func (r *ReconcilerBase) GetResolvedBindingSecret(ba common.BaseComponent) (*corev1.Secret, error) {
	if len(ba.GetStatus().GetResolvedBindings()) == 0 {
		return nil, nil
	}
	mObj := ba.(metav1.Object)
	secret := &corev1.Secret{}
	key := types.NamespacedName{Name: ba.GetStatus().GetResolvedBindings()[0], Namespace: mObj.GetNamespace()}
	err := r.client.Get(context.TODO(), key, secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

// done when no error happens
func (r *ReconcilerBase) done(ba common.BaseComponent) (reconcile.Result, error) {
	return r.ManageSuccess(common.StatusConditionTypeDependenciesSatisfied, ba)
}

// requeueError simply calls ManageError when dependency is not fulfilled
func (r *ReconcilerBase) requeueError(ba common.BaseComponent, err error) (reconcile.Result, error) {
	r.ManageError(err, common.StatusConditionTypeDependenciesSatisfied, ba)
	return r.ManageError(errors.New("dependency not satisfied"), common.StatusConditionTypeReconciled, ba)
}

func isRequeue(res reconcile.Result, err error) bool {
	return err != nil || res.Requeue
}

func getCopiedToNamespacesAnnotationKey(ba common.BaseComponent) string {
	return "service." + ba.GetGroupName() + "/copied-to-namespaces"
}

func getConsumedByAnnotationKey(ba common.BaseComponent) string {
	return "service." + ba.GetGroupName() + "/consumed-by"
}

func equals(sl1, sl2 []string) bool {
	if len(sl1) != len(sl2) {
		return false
	}
	for i, v := range sl1 {
		if v != sl2[i] {
			return false
		}
	}
	return true
}

func getOpConfigServiceBindingGVKs() []schema.GroupVersionKind {
	gvkStringList := strings.Split(common.Config[common.OpConfigSvcBindingGVKs], ",")
	for i := range gvkStringList {
		gvkStringList[i] = strings.TrimSpace(gvkStringList[i])
	}

	gvkList := []schema.GroupVersionKind{}
	for i := range gvkStringList {
		gvk, _ := schema.ParseKindArg(gvkStringList[i])
		if gvk == nil {
			log.Error(errors.Errorf("failed to parse %q to a valid GroupVersionKind", gvkStringList[i]), "Invalid GroupVersionKind from operator ConfigMap")
			continue
		}
		gvkList = append(gvkList, *gvk)
	}
	return gvkList
}

func (r *ReconcilerBase) getServiceBindingGVK() (gvkList []schema.GroupVersionKind) {
	for _, gvk := range getOpConfigServiceBindingGVKs() {
		if ok, _ := r.IsGroupVersionSupported(gvk.GroupVersion().String(), gvk.Kind); ok {
			gvkList = append(gvkList, gvk)
		}
	}
	return gvkList
}

// IsServiceBindingSupported returns true if at least one GVK in the operator ConfigMap's serviceBinding.groupVersionKinds is installed
func (r *ReconcilerBase) IsServiceBindingSupported() bool {
	return len(r.getServiceBindingGVK()) > 0
}

// cleanUpEmbeddedBindings deletes Service Binding resources owned by current CR based having the same name as CR
func (r *ReconcilerBase) cleanUpEmbeddedBindings(ba common.BaseComponent) error {
	mObj := ba.(metav1.Object)
	for _, gvk := range r.getServiceBindingGVK() {
		bindingObj := &unstructured.Unstructured{}
		bindingObj.SetGroupVersionKind(gvk)
		key := types.NamespacedName{Name: getDefaultServiceBindingName(ba), Namespace: mObj.GetNamespace()}
		err := r.client.Get(context.Background(), key, bindingObj)
		if err == nil && metav1.IsControlledBy(bindingObj, mObj) {
			err = r.client.Delete(context.Background(), bindingObj)
			if client.IgnoreNotFound(err) != nil {
				return err
			}
		} else if client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

// reconcileEmbedded reconciles embedded blob in bindings.embedded to create or update Service Binding resource
func (r *ReconcilerBase) reconcileEmbedded(ba common.BaseComponent) (retRes reconcile.Result, retErr error) {
	var resolvedBindings []string

	object, err := r.toJSONFromRaw(ba.GetBindings().GetEmbedded())
	if err != nil {
		err = errors.Wrapf(err, "failed: unable marshalling to JSON")
		return r.requeueError(ba, err)
	}

	embedded := &unstructured.Unstructured{}
	embedded.SetUnstructuredContent(object)
	err = r.updateEmbeddedObject(object, embedded, ba)
	if err != nil {
		err = errors.Wrapf(err, "failed: cannot add missing information to the embedded Service Binding")
		return r.requeueError(ba, err)
	}

	apiVersion, kind := embedded.GetAPIVersion(), embedded.GetKind()
	ok, err := r.IsGroupVersionSupported(apiVersion, kind)
	if !ok {
		err = errors.Wrapf(err, "failed: embedded Service Binding CRD with GroupVersion %q and Kind %q is not supported on the cluster", apiVersion, kind)
		return r.requeueError(ba, err)
	}

	err = r.createOrUpdateEmbedded(embedded, ba)
	if err != nil {
		return r.requeueError(ba, errors.Wrapf(err, "failed: cannot create or update embedded Service Binding resource %q in namespace %q", embedded.GetName(), embedded.GetNamespace()))
	}

	// Get binding secret and add it to status field. If binding hasn't been successful, secret won't be created and it keeps trying
	key := types.NamespacedName{Name: embedded.GetName(), Namespace: embedded.GetNamespace()}
	bindingSecret := &corev1.Secret{}
	err = r.GetClient().Get(context.TODO(), key, bindingSecret)
	if err == nil {
		resolvedBindings = append(resolvedBindings, embedded.GetName())
	} else {
		err = errors.Wrapf(err, "service binding dependency not satisfied: unable to find service binding secret for embedded binding %q in namespace %q", embedded.GetName(), embedded.GetNamespace())
		return r.requeueError(ba, err)
	}

	retRes, retErr = r.done(ba)
	defer func() {
		if res, err := r.updateResolvedBindingStatus(resolvedBindings, ba); isRequeue(res, err) {
			retRes, retErr = res, err
		}
	}()
	return
}

func (r *ReconcilerBase) updateResolvedBindingStatus(bindings []string, ba common.BaseComponent) (reconcile.Result, error) {
	if !equals(bindings, ba.GetStatus().GetResolvedBindings()) {
		sort.Strings(bindings)
		ba.GetStatus().SetResolvedBindings(bindings)
		if err := r.UpdateStatus(ba.(client.Object)); err != nil {
			return r.requeueError(ba, errors.Wrapf(err, "unable to update status with resolved service binding information"))
		}
	}
	return r.done(ba)
}

func (r *ReconcilerBase) createOrUpdateEmbedded(embedded *unstructured.Unstructured, ba common.BaseComponent) error {
	result := controllerutil.OperationResultNone
	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion(embedded.GetAPIVersion())
	existing.SetKind(embedded.GetKind())
	key := types.NamespacedName{Name: embedded.GetName(), Namespace: embedded.GetNamespace()}
	err := r.client.Get(context.TODO(), key, existing)
	if err != nil {
		if kerrors.IsNotFound(err) {
			err = r.client.Create(context.TODO(), embedded)
			if err != nil {
				return err
			}
			result = controllerutil.OperationResultCreated
			// add watcher
			err = r.controller.Watch(&source.Kind{Type: existing}, &handler.EnqueueRequestForOwner{
				IsController: true,
				OwnerType:    ba.(runtime.Object),
			})
			if err != nil {
				return errors.Wrap(err, "Cannot add watcher")
			}
		} else {
			return err
		}
	} else {
		// Update the found object and write the result back if there are any changes
		if !reflect.DeepEqual(embedded.Object[Spec], existing.Object[Spec]) {
			existing.Object[Spec] = embedded.Object[Spec]
			err = r.client.Update(context.TODO(), existing)
			if err != nil {
				return err
			}
			result = controllerutil.OperationResultUpdated
		}
	}

	log.Info("Reconciled", "Kind", embedded.GetKind(), "Namespace", embedded.GetNamespace(), "Name", embedded.GetName(), "Status", result)
	return nil
}

func (r *ReconcilerBase) toJSONFromRaw(content *runtime.RawExtension) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(content.Raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *ReconcilerBase) updateEmbeddedObject(object map[string]interface{}, embedded *unstructured.Unstructured, ba common.BaseComponent) error {
	mObj := ba.(metav1.Object)

	if _, ok := object[Spec]; !ok {
		return errors.New("failed: embedded Service Binding is missing a 'spec' section")
	}

	if _, ok := object[Metadata]; ok {
		return errors.New("failed: embedded Service Binding must not have a 'metadata' section")
	}
	embedded.SetName(getDefaultServiceBindingName(ba))
	embedded.SetNamespace(mObj.GetNamespace())
	embedded.SetLabels(mObj.GetLabels())
	embedded.SetAnnotations(mObj.GetAnnotations())

	if err := controllerutil.SetControllerReference(mObj, embedded, r.scheme); err != nil {
		return errors.Wrap(err, "SetControllerReference returned error")
	}

	apiVersion, okAPIVersion := object[APIVersion]
	kind, okKind := object[Kind]

	// If either API Version or Kind is not set, try getting it from the Operator ConfigMap
	var defaultGVK schema.GroupVersionKind
	if !okAPIVersion || !okKind {
		cmGVK := getOpConfigServiceBindingGVKs()
		if len(cmGVK) == 0 {
			return errors.New("failed: embedded Service Binding does not specify 'apiVersion' or 'kind' and there is no default GVK defined in the operator ConfigMap")
		}
		defaultGVK = cmGVK[0]
	}
	if !okAPIVersion {
		apiVersion = defaultGVK.GroupVersion().String()
	}
	if !okKind {
		kind = defaultGVK.Kind
	}

	embedded.SetKind(kind.(string))
	embedded.SetAPIVersion(apiVersion.(string))
	return nil
}

func getDefaultServiceBindingName(ba common.BaseComponent) string {
	return (ba.(metav1.Object)).GetName() + "-binding"
}

func getOverrideExposeBindingSecretName(ba common.BaseComponent) string {
	return (ba.(metav1.Object)).GetName() + ExposeBindingOverrideSecretSuffix
}

func getExposeBindingSecretName(ba common.BaseComponent) string {
	return (ba.(metav1.Object)).GetName() + ExposeBindingSecretSuffix
}
