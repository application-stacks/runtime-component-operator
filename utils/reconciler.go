package utils

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	networkingv1 "k8s.io/api/networking/v1"

	"github.com/application-stacks/runtime-component-operator/common"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReconcilerBase base reconciler with some common behaviour
type ReconcilerBase struct {
	apiReader  client.Reader
	client     client.Client
	scheme     *runtime.Scheme
	recorder   record.EventRecorder
	restConfig *rest.Config
	discovery  discovery.DiscoveryInterface
	controller controller.Controller
}

// NewReconcilerBase creates a new ReconcilerBase
func NewReconcilerBase(apiReader client.Reader, client client.Client, scheme *runtime.Scheme, restConfig *rest.Config, recorder record.EventRecorder) ReconcilerBase {
	return ReconcilerBase{
		apiReader:  apiReader,
		client:     client,
		scheme:     scheme,
		recorder:   recorder,
		restConfig: restConfig,
	}
}

// GetController returns controller
func (r *ReconcilerBase) GetController() controller.Controller {
	return r.controller
}

// SetController sets controller
func (r *ReconcilerBase) SetController(c controller.Controller) {
	r.controller = c
}

// GetClient returns client
func (r *ReconcilerBase) GetClient() client.Client {
	return r.client
}

// GetAPIReader returns a client.Reader. Use client.Reader only if a
// particular resource does not implement the 'watch' verb such as
// ImageStreamTag. This is because the operator-sdk Client
// automatically performs a Watch on all the objects that are obtained
// with Get, but some resources such as the ImageStreamTag kind does not
// implement the Watch verb, which caused errors.
// Here is an example of how the error would look like:
//
//	`Failed to watch *v1.ImageStreamTag: the server does not allow this method on the requested resource (get imagestreamtags.image.openshift.io)`
func (r *ReconcilerBase) GetAPIReader() client.Reader {
	return r.apiReader
}

// GetRecorder returns the underlying recorder
func (r *ReconcilerBase) GetRecorder() record.EventRecorder {
	return r.recorder
}

// GetDiscoveryClient ...
func (r *ReconcilerBase) GetDiscoveryClient() (discovery.DiscoveryInterface, error) {
	if r.discovery == nil {
		var err error
		r.discovery, err = discovery.NewDiscoveryClientForConfig(r.restConfig)
		return r.discovery, err
	}

	return r.discovery, nil
}

// SetDiscoveryClient ...
func (r *ReconcilerBase) SetDiscoveryClient(discovery discovery.DiscoveryInterface) {
	r.discovery = discovery
}

var log = logf.Log.WithName("utils")
var logD1 = log.V(common.LogLevelDebug)

// CreateOrUpdate ...
func (r *ReconcilerBase) CreateOrUpdate(obj client.Object, owner metav1.Object, reconcile func() error) error {

	if owner != nil {
		controllerutil.SetControllerReference(owner, obj, r.scheme)
	}

	result, err := controllerutil.CreateOrUpdate(context.TODO(), r.GetClient(), obj, reconcile)
	if err != nil {
		return err
	}

	var gvk schema.GroupVersionKind
	gvk, err = apiutil.GVKForObject(obj, r.scheme)
	if err == nil {
		logD1.Info("Reconciled", "Kind", gvk.Kind, "Namespace", obj.GetNamespace(), "Name", obj.GetName(), "Status", result)
	}

	return err
}

// DeleteResource deletes kubernetes resource
func (r *ReconcilerBase) DeleteResource(obj client.Object) error {
	err := r.client.Delete(context.TODO(), obj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Unable to delete object ", "object", obj)
			return err
		}
		return nil
	}

	metaObj, ok := obj.(metav1.Object)
	if !ok {
		err := fmt.Errorf("%T is not a metav1.Object", obj)
		log.Error(err, "Failed to convert into metav1.Object")
		return err
	}

	var gvk schema.GroupVersionKind
	gvk, err = apiutil.GVKForObject(obj, r.scheme)
	if err == nil {
		log.Info("Reconciled", "Kind", gvk.Kind, "Name", metaObj.GetName(), "Status", "deleted")
	}
	return nil
}

// DeleteResources ...
func (r *ReconcilerBase) DeleteResources(resources []client.Object) error {
	for i := range resources {
		err := r.DeleteResource(resources[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// GetOpConfigMap ...
func (r *ReconcilerBase) GetOpConfigMap(name string, ns string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	err := r.GetAPIReader().Get(context.TODO(), types.NamespacedName{Name: name, Namespace: ns}, configMap)
	if err != nil {
		return nil, err
	}
	return configMap, nil
}

func addStatusWarnings(ba common.BaseComponent) {

	s := ba.GetStatus()

	mtls := ba.GetManageTLS()
	svc := ba.GetService()
	if (mtls == nil || *mtls == true) && svc != nil && svc.GetPort() == 9080 {
		status := corev1.ConditionTrue
		msg := "ManageTLS is true but port is set to 9080"
		statusCondition := s.NewCondition(common.StatusConditionTypeWarning)
		statusCondition.SetReason("")
		statusCondition.SetMessage(msg)
		statusCondition.SetStatus(status)
		s.SetCondition(statusCondition)
	} else {
		// The warning condition may previously have been set, but is now not needed.
		// Removing the warning is clearer than have a warning condition set to 'false'
		statusCondition := s.NewCondition(common.StatusConditionTypeWarning)
		s.UnsetCondition(statusCondition)
	}
}

func getMaxReconcileInterval(success bool) int {
	var maxInterval int
	if success {
		maxInterval, _ = strconv.Atoi(common.LoadFromConfig(common.Config, common.OpConfigReconcileIntervalSuccessMaximum))
	} else {
		maxInterval, _ = strconv.Atoi(common.LoadFromConfig(common.Config, common.OpConfigReconcileIntervalFailureMaximum))
	}

	return maxInterval
}

func resetReconcileInterval(s common.BaseComponentStatus) time.Duration {
	minIntervalInt, _ := strconv.Atoi(common.LoadFromConfig(common.Config, common.OpConfigReconcileIntervalMinimum))
	minInterval := int32(minIntervalInt)

	// If user wants to hide reconcileInterval field, unset the value
	showReconcileInterval := common.LoadFromConfig(common.Config, common.OpConfigShowReconcileInterval)

	if showReconcileInterval == "true" {
		s.SetReconcileInterval(&minInterval)
	} else {
		s.UnsetReconcileInterval()
	}

	return time.Duration(minInterval) * time.Second
}

// Precondition: Operator config values for common.OpConfigReconcileIntervalMinimum and common.OpConfigReconcileIntervalPercentage must be integers
func updateReconcileInterval(maxSeconds int, s common.BaseComponentStatus, ba common.BaseComponent) time.Duration {
	lastestTransitionTime := s.GetLatestTransitionTime()
	if lastestTransitionTime == nil {
		return resetReconcileInterval(s)
	}
	currentTime := metav1.Time{Time: time.Now()}
	realReconcileInterval := math.Floor(currentTime.Time.Sub(lastestTransitionTime.Time).Seconds())

	minInterval, _ := strconv.ParseFloat(common.LoadFromConfig(common.Config, common.OpConfigReconcileIntervalMinimum), 64)
	intervalIncreasePercentage, _ := strconv.ParseFloat(common.LoadFromConfig(common.Config, common.OpConfigReconcileIntervalPercentage), 64)
	increase := 1 + (intervalIncreasePercentage / 100)

	newInterval := int32(minInterval)

	// If last transition time was more than maxSeconds ago,
	// set reconcile interval to maxSeconds
	if realReconcileInterval >= float64(maxSeconds) {
		newInterval = int32(maxSeconds)
	} else if realReconcileInterval >= minInterval {
		// If last transition time was in between minInterval and maxSeconds ago,
		// increase the reconcile interval
		increasedInterval := realReconcileInterval

		// Calculate exponent
		exponent := 0
		for increasedInterval > minInterval {
			increasedInterval = math.Floor(increasedInterval / increase)
			exponent += 1
		}
		newIncrease := math.Pow(1+(intervalIncreasePercentage/100), float64(exponent))
		newInterval = int32(minInterval * newIncrease)

		// If the increased interval exceeds maxSeconds,
		// set reconcile interval to maxSeconds
		if newInterval >= int32(maxSeconds) {
			newInterval = int32(maxSeconds)
		}
	}

	// If user wants to hide reconcileInterval field, unset the value
	showReconcileInterval := common.LoadFromConfig(common.Config, common.OpConfigShowReconcileInterval)

	if showReconcileInterval == "true" {
		s.SetReconcileInterval(&newInterval)
	} else {
		s.UnsetReconcileInterval()
	}

	return time.Duration(newInterval) * time.Second
}

// ManageError ...
func (r *ReconcilerBase) ManageError(issue error, conditionType common.StatusConditionType, ba common.BaseComponent) (reconcile.Result, error) {
	s := ba.GetStatus()
	obj := ba.(client.Object)
	logger := log.WithValues("ba.Namespace", obj.GetNamespace(), "ba.Name", obj.GetName())
	logger.Error(issue, "ManageError", "Condition", conditionType, "ba", ba)
	r.GetRecorder().Event(obj, "Warning", "ProcessingError", issue.Error())

	oldCondition := s.GetCondition(conditionType)
	newCondition := s.NewCondition(conditionType)
	newCondition.SetReason(string(apierrors.ReasonForError(issue)))
	newCondition.SetMessage(issue.Error())
	newCondition.SetStatus(corev1.ConditionFalse)
	r.setCondition(ba, oldCondition, newCondition)

	addStatusWarnings(ba)

	if conditionType != common.StatusConditionTypeResourcesReady {
		//Check Application status (reconciliation & resource status & endpoint status)
		r.CheckApplicationStatus(ba)
	} else {
		// If the new condition type is ResourcesReady (false), make sure the
		// conditions for Ready and Reconciled are set to false
		readyNewCondition := s.NewCondition(common.StatusConditionTypeReady)
		readyNewCondition.SetStatus(corev1.ConditionFalse)
		readyNewCondition.SetMessage("Resources are not ready.")
		readyNewCondition.SetReason("")
		s.SetCondition(readyNewCondition)
		reconciledNewCondition := s.NewCondition(common.StatusConditionTypeReconciled)
		reconciledNewCondition.SetStatus(corev1.ConditionFalse)
		reconciledNewCondition.SetMessage("Resources are not ready.")
		reconciledNewCondition.SetReason("")
		s.SetCondition(reconciledNewCondition)
	}

	var retryInterval time.Duration
	// If the application was reconciled and now it is not or encountered a different error
	// Use the default reconcile interval
	if oldCondition == nil || s.GetReconcileInterval() == nil || oldCondition.GetStatus() != newCondition.GetStatus() || oldCondition.GetMessage() != newCondition.GetMessage() || oldCondition.GetReason() != newCondition.GetReason() {
		retryInterval = resetReconcileInterval(s)
	} else {
		// If the application fails to reconcile again and the error message has not changed
		// Increase the retry interval upto maxSeconds
		maxSeconds := getMaxReconcileInterval(false)
		retryInterval = updateReconcileInterval(maxSeconds, s, ba)
	}

	err := r.UpdateStatus(obj)
	if err != nil {

		if apierrors.IsConflict(issue) {
			return reconcile.Result{Requeue: true}, nil
		}
		logger.Error(err, "Unable to update status")
		return reconcile.Result{
			RequeueAfter: time.Second,
			Requeue:      true,
		}, nil
	}

	return reconcile.Result{
		//RequeueAfter: time.Duration(math.Min(float64(retryInterval.Nanoseconds()*2), float64(time.Hour.Nanoseconds()*6))),
		RequeueAfter: retryInterval,
		Requeue:      true,
	}, nil
}

// ManageSuccess ...
func (r *ReconcilerBase) ManageSuccess(conditionType common.StatusConditionType, ba common.BaseComponent) (reconcile.Result, error) {
	s := ba.GetStatus()
	oldRecCondition := s.GetCondition(conditionType)

	newRecCondition := s.NewCondition(conditionType)
	newRecCondition.SetReason("")
	newRecCondition.SetMessage("")
	newRecCondition.SetStatus(corev1.ConditionTrue)
	s.SetCondition(newRecCondition)

	addStatusWarnings(ba)

	// Check application status (reconciliation & resource condition & endpoint condition)
	// CheckApplicationStatus returns overall Application condition if ready
	// If not ready, it returns Resources condition
	oldCondition, newCondition := r.CheckApplicationStatus(ba)

	var retryInterval time.Duration

	// If the application or resources are not ready
	if oldCondition == nil || oldCondition.GetStatus() != newCondition.GetStatus() || oldCondition.GetMessage() != newCondition.GetMessage() || oldRecCondition == nil || oldRecCondition.GetStatus() != newRecCondition.GetStatus() || oldRecCondition.GetMessage() != newRecCondition.GetMessage() || oldCondition.GetReason() != newCondition.GetReason() {
		// If the application or the resources were not reconciled before
		// Or if the application (resources) were not ready before
		// Use the default reconcile interval
		retryInterval = resetReconcileInterval(s)
	} else {
		// If the application and resources stay ready and there are no changes
		// Increase the retry interval upto maxSeconds
		if newCondition.GetStatus() != corev1.ConditionTrue {
			maxSeconds := getMaxReconcileInterval(false)
			retryInterval = updateReconcileInterval(maxSeconds, s, ba)
		} else {
			maxSeconds := getMaxReconcileInterval(true)
			retryInterval = updateReconcileInterval(maxSeconds, s, ba)
		}
	}

	err := r.UpdateStatus(ba.(client.Object))
	if err != nil {
		log.Error(err, "Unable to update status")
		return reconcile.Result{
			RequeueAfter: time.Second,
			Requeue:      true,
		}, nil
	}

	return reconcile.Result{RequeueAfter: retryInterval}, nil
}

// IsGroupVersionSupported ...
func (r *ReconcilerBase) IsGroupVersionSupported(groupVersion string, kind string) (bool, error) {
	cli, err := r.GetDiscoveryClient()
	if err != nil {
		log.Error(err, "Failed to return a discovery client for the current reconciler")
		return false, err
	}

	res, err := cli.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	for _, v := range res.APIResources {
		if v.Kind == kind {
			return true, nil
		}
	}

	return false, nil
}

// UpdateStatus updates the fields corresponding to the status subresource for the object
func (r *ReconcilerBase) UpdateStatus(obj client.Object) error {
	return r.GetClient().Status().Update(context.Background(), obj)
}

// IsOpenShift returns true if the operator is running on an OpenShift platform
func (r *ReconcilerBase) IsOpenShift() bool {
	isOpenShift, err := r.IsGroupVersionSupported(routev1.SchemeGroupVersion.String(), "Route")
	if err != nil {
		return false
	}
	return isOpenShift
}

// GetRouteTLSValues returns certificate an key values to be used in the route
func (r *ReconcilerBase) GetRouteTLSValues(ba common.BaseComponent) (key string, cert string, ca string, destCa string, err error) {
	key, cert, ca, destCa = "", "", "", ""
	mObj := ba.(metav1.Object)
	if ba.GetManageTLS() == nil || *ba.GetManageTLS() || ba.GetService() != nil && ba.GetService().GetCertificateSecretRef() != nil {
		tlsSecret := &corev1.Secret{}
		secretName := ba.GetStatus().GetReferences()[common.StatusReferenceCertSecretName]
		err := r.GetClient().Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: mObj.GetNamespace()}, tlsSecret)
		if err != nil {
			r.ManageError(err, common.StatusConditionTypeReconciled, ba)
			return "", "", "", "", err
		}
		caCrt, ok := tlsSecret.Data["ca.crt"]
		if ok {
			destCa = string(caCrt)
		}
	}
	if ba.GetRoute() != nil && ba.GetRoute().GetCertificateSecretRef() != nil {
		tlsSecret := &corev1.Secret{}
		secretName := mObj.GetName() + "-route-tls"
		if ba.GetRoute().GetCertificateSecretRef() != nil {
			secretName = *ba.GetRoute().GetCertificateSecretRef()
		}
		err := r.GetClient().Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: mObj.GetNamespace()}, tlsSecret)
		if err != nil {
			r.ManageError(err, common.StatusConditionTypeReconciled, ba)
			return "", "", "", "", err
		}

		var extraCerts string
		v, ok := tlsSecret.Data["tls.crt"]
		if ok {
			cert = string(v)
			certBlock, nextCerts := pem.Decode(v)
			if certBlock == nil {
				return "", "", "", "", errors.New("failed to load route certificate")
			}
			cert = string(pem.EncodeToMemory(certBlock))
			if len(nextCerts) > 0 {
				//multiple certificates (chain)
				extraCerts = string(nextCerts)
			}
		}
		v, ok = tlsSecret.Data["ca.crt"]
		if ok || extraCerts != "" {
			ca = string(v)
			if extraCerts != "" {
				if ca != "" {
					ca = ca + "\n" + extraCerts
				} else {
					ca = extraCerts
				}
			}
		}
		v, ok = tlsSecret.Data["tls.key"]
		if ok {
			key = string(v)
		}
		v, ok = tlsSecret.Data["destCA.crt"]
		if ok {
			destCa = string(v)
		}
	}
	return key, cert, ca, destCa, nil
}

func (r *ReconcilerBase) GenerateCMIssuer(namespace string, prefix string, CACommonName string, operatorName string) error {
	if ok, err := r.IsGroupVersionSupported(certmanagerv1.SchemeGroupVersion.String(), "Issuer"); err != nil {
		return err
	} else if !ok {
		return APIVersionNotFoundError
	}

	issuer := &certmanagerv1.Issuer{ObjectMeta: metav1.ObjectMeta{
		Name:      prefix + "-self-signed",
		Namespace: namespace,
	}}
	err := r.CreateOrUpdate(issuer, nil, func() error {
		issuer.Spec.SelfSigned = &certmanagerv1.SelfSignedIssuer{}
		issuer.Labels = MergeMaps(issuer.Labels, map[string]string{"app.kubernetes.io/managed-by": operatorName})
		return nil
	})
	if err != nil {
		return err
	}
	caCert := &certmanagerv1.Certificate{ObjectMeta: metav1.ObjectMeta{
		Name:      prefix + "-ca-cert",
		Namespace: namespace,
	}}

	err = r.CreateOrUpdate(caCert, nil, func() error {
		caCert.Labels = MergeMaps(caCert.Labels, map[string]string{"app.kubernetes.io/managed-by": operatorName})
		caCert.Spec.CommonName = CACommonName
		caCert.Spec.IsCA = true
		caCert.Spec.SecretName = prefix + "-ca-tls"
		caCert.Spec.IssuerRef = certmanagermetav1.ObjectReference{
			Name: prefix + "-self-signed",
		}

		duration, err := time.ParseDuration(common.LoadFromConfig(common.Config, common.OpConfigCMCADuration))
		if err != nil {
			return err
		}

		caCert.Spec.Duration = &metav1.Duration{Duration: duration}
		return nil
	})

	if err != nil {
		return err
	}
	CustomCACert := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      prefix + "-custom-ca-tls",
		Namespace: namespace,
	}}
	customCACertFound := false
	err = r.GetClient().Get(context.Background(), types.NamespacedName{Name: CustomCACert.GetName(),
		Namespace: CustomCACert.GetNamespace()}, CustomCACert)
	if err == nil {
		customCACertFound = true
	}

	issuer = &certmanagerv1.Issuer{ObjectMeta: metav1.ObjectMeta{
		Name:      prefix + "-ca-issuer",
		Namespace: namespace,
	}}
	err = r.CreateOrUpdate(issuer, nil, func() error {
		issuer.Labels = MergeMaps(issuer.Labels, map[string]string{"app.kubernetes.io/managed-by": operatorName})
		issuer.Spec.CA = &certmanagerv1.CAIssuer{}
		issuer.Spec.CA.SecretName = prefix + "-ca-tls"
		if issuer.Annotations == nil {
			issuer.Annotations = map[string]string{}
		}
		if customCACertFound {
			issuer.Spec.CA.SecretName = CustomCACert.Name

		}
		return nil
	})
	if err != nil {
		return err
	}

	for i := range issuer.Status.Conditions {
		if issuer.Status.Conditions[i].Type == certmanagerv1.IssuerConditionReady && issuer.Status.Conditions[i].Status == certmanagermetav1.ConditionFalse {
			return errors.New("Certificate Issuer is not ready")
		}
		if issuer.Status.Conditions[i].Type == certmanagerv1.IssuerConditionReady && issuer.Status.Conditions[i].ObservedGeneration != issuer.ObjectMeta.Generation {
			return errors.New("Certificate Issuer is not ready")
		}
	}
	return nil
}

func (r *ReconcilerBase) GenerateSvcCertSecret(ba common.BaseComponent, prefix string, CACommonName string, operatorName string) (bool, error) {

	delete(ba.GetStatus().GetReferences(), common.StatusReferenceCertSecretName)
	cleanup := func() {
		if ok, err := r.IsGroupVersionSupported(certmanagerv1.SchemeGroupVersion.String(), "Certificate"); err != nil {
			return
		} else if ok {
			obj := ba.(metav1.Object)
			svcCert := &certmanagerv1.Certificate{}
			svcCert.Name = obj.GetName() + "-svc-tls-cm"
			svcCert.Namespace = obj.GetNamespace()
			r.client.Delete(context.Background(), svcCert)
		}
	}

	if ba.GetCreateKnativeService() != nil && *ba.GetCreateKnativeService() {
		cleanup()
		return false, nil
	}
	if ba.GetService() != nil && ba.GetService().GetCertificateSecretRef() != nil {
		cleanup()
		return false, nil
	}
	if ba.GetManageTLS() != nil && !*ba.GetManageTLS() {
		cleanup()
		return false, nil
	}
	if ba.GetService() != nil && ba.GetService().GetAnnotations() != nil {
		if _, ok := ba.GetService().GetAnnotations()["service.beta.openshift.io/serving-cert-secret-name"]; ok {
			cleanup()
			return false, nil
		}
		if _, ok := ba.GetService().GetAnnotations()["service.alpha.openshift.io/serving-cert-secret-name"]; ok {
			cleanup()
			return false, nil
		}
	}
	if ok, err := r.IsGroupVersionSupported(certmanagerv1.SchemeGroupVersion.String(), "Certificate"); err != nil {
		return false, err
	} else if ok {
		bao := ba.(metav1.Object)

		err = r.GenerateCMIssuer(bao.GetNamespace(), prefix, CACommonName, operatorName)
		if err != nil {
			if errors.Is(err, APIVersionNotFoundError) {
				return false, nil
			}
			return true, err
		}
		svcCertSecretName := bao.GetName() + "-svc-tls-cm"

		svcCert := &certmanagerv1.Certificate{ObjectMeta: metav1.ObjectMeta{
			Name:      svcCertSecretName,
			Namespace: bao.GetNamespace(),
		}}

		customIssuer := &certmanagerv1.Issuer{ObjectMeta: metav1.ObjectMeta{
			Name:      prefix + "-custom-issuer",
			Namespace: bao.GetNamespace(),
		}}

		customIssuerFound := false
		err = r.GetClient().Get(context.Background(), types.NamespacedName{Name: customIssuer.Name,
			Namespace: customIssuer.Namespace}, customIssuer)
		if err == nil {
			customIssuerFound = true
		}

		shouldRefreshCertSecret := false
		err = r.CreateOrUpdate(svcCert, bao, func() error {
			svcCert.Labels = ba.GetLabels()
			svcCert.Annotations = MergeMaps(svcCert.Annotations, ba.GetAnnotations())
			if ba.GetService() != nil {
				if ba.GetService().GetCertificate() != nil {
					if ba.GetService().GetCertificate().GetAnnotations() != nil {
						svcCert.Annotations = MergeMaps(svcCert.Annotations, ba.GetService().GetCertificate().GetAnnotations())
					}
				}
			}

			svcCert.Spec.CommonName = trimCommonName(bao.GetName(), bao.GetNamespace())
			svcCert.Spec.DNSNames = make([]string, 4)
			svcCert.Spec.DNSNames[0] = bao.GetName() + "." + bao.GetNamespace() + ".svc"
			svcCert.Spec.DNSNames[1] = bao.GetName() + "." + bao.GetNamespace() + ".svc.cluster.local"
			svcCert.Spec.DNSNames[2] = bao.GetName() + "." + bao.GetNamespace()
			svcCert.Spec.DNSNames[3] = bao.GetName()
			if ba.GetStatefulSet() != nil {
				svcCert.Spec.DNSNames = append(svcCert.Spec.DNSNames, bao.GetName()+"-headless."+bao.GetNamespace()+".svc")
				svcCert.Spec.DNSNames = append(svcCert.Spec.DNSNames, bao.GetName()+"-headless."+bao.GetNamespace()+".svc.cluster.local")
				svcCert.Spec.DNSNames = append(svcCert.Spec.DNSNames, bao.GetName()+"-headless."+bao.GetNamespace())
				svcCert.Spec.DNSNames = append(svcCert.Spec.DNSNames, bao.GetName()+"-headless")
				// Wildcard entries for the pods
				svcCert.Spec.DNSNames = append(svcCert.Spec.DNSNames, "*."+bao.GetName()+"-headless."+bao.GetNamespace()+".svc")
				svcCert.Spec.DNSNames = append(svcCert.Spec.DNSNames, "*."+bao.GetName()+"-headless."+bao.GetNamespace()+".svc.cluster.local")
				svcCert.Spec.DNSNames = append(svcCert.Spec.DNSNames, "*."+bao.GetName()+"-headless."+bao.GetNamespace())
				svcCert.Spec.DNSNames = append(svcCert.Spec.DNSNames, "*."+bao.GetName()+"-headless")
			}
			svcCert.Spec.IsCA = false
			svcCert.Spec.IssuerRef = certmanagermetav1.ObjectReference{
				Name: prefix + "-ca-issuer",
			}
			if customIssuerFound {
				svcCert.Spec.IssuerRef.Name = customIssuer.Name
			}

			rVersion, _ := GetIssuerResourceVersion(r.client, svcCert)
			if svcCert.Spec.SecretTemplate == nil {
				svcCert.Spec.SecretTemplate = &certmanagerv1.CertificateSecretTemplate{
					Annotations: map[string]string{},
				}
			}

			if svcCert.Spec.SecretTemplate.Annotations[ba.GetGroupName()+"/cm-issuer-version"] != rVersion {
				if svcCert.Spec.SecretTemplate.Annotations == nil {
					svcCert.Spec.SecretTemplate.Annotations = map[string]string{}
				}
				svcCert.Spec.SecretTemplate.Annotations[ba.GetGroupName()+"/cm-issuer-version"] = rVersion
				shouldRefreshCertSecret = true
			}

			svcCert.Spec.SecretName = svcCertSecretName

			duration, err := time.ParseDuration(common.LoadFromConfig(common.Config, common.OpConfigCMCertDuration))
			if err != nil {
				return err
			}
			svcCert.Spec.Duration = &metav1.Duration{Duration: duration}

			return nil
		})
		if err != nil {
			return true, err
		}
		if shouldRefreshCertSecret {
			r.DeleteResource(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: svcCertSecretName, Namespace: svcCert.Namespace}})
		}
		ba.GetStatus().SetReference(common.StatusReferenceCertSecretName, svcCertSecretName)
	} else {
		return false, nil
	}
	return true, nil
}

func (r *ReconcilerBase) GetIngressInfo(ba common.BaseComponent) (host string, path string, protocol string) {
	mObj := ba.(metav1.Object)
	protocol = "http"
	if ok, err := r.IsGroupVersionSupported(routev1.SchemeGroupVersion.String(), "Route"); err != nil {
		r.ManageError(err, common.StatusConditionTypeReconciled, ba)
	} else if ok {
		route := &routev1.Route{}
		r.GetClient().Get(context.Background(), types.NamespacedName{Name: mObj.GetName(), Namespace: mObj.GetNamespace()}, route)
		host = route.Spec.Host
		path = route.Spec.Path
		if route.Spec.TLS != nil {
			protocol = "https"
		}
		return host, path, protocol
	} else {
		if ok, err := r.IsGroupVersionSupported(networkingv1.SchemeGroupVersion.String(), "Ingress"); err != nil {
			r.ManageError(err, common.StatusConditionTypeReconciled, ba)
		} else if ok {
			ingress := &networkingv1.Ingress{}
			r.GetClient().Get(context.Background(), types.NamespacedName{Name: mObj.GetName(), Namespace: mObj.GetNamespace()}, ingress)
			if len(ingress.Spec.Rules) > 0 && ingress.Spec.Rules[0].Host != "" {
				host = ingress.Spec.Rules[0].Host
				if len(ingress.Spec.TLS) > 0 && len(ingress.Spec.TLS[0].Hosts) > 0 && ingress.Spec.TLS[0].Hosts[0] != "" {
					protocol = "https"
				}
				if ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths != nil && len(ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths) != 0 {
					path = ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path
				}
				return host, path, protocol
			}
		}
	}
	return host, path, protocol
}

// Create a common name for a certificate that is no longer
// that 64 bytes
func trimCommonName(compName string, ns string) (cn string) {

	commonName := compName + "." + ns + ".svc"
	if len(commonName) > 64 {
		// Try removing '.svc'
		commonName = compName + "." + ns
	}
	if len(commonName) > 64 {
		// Try removing the namespace
		commonName = compName
	}
	if len(commonName) > 64 {
		// Just have to truncate
		commonName = commonName[:64]
	}

	return commonName
}
