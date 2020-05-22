package e2e

import (
	goctx "context"
	"fmt"
	"errors"
	"testing"
	"time"

	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	"github.com/application-stacks/runtime-component-operator/test/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	routev1 "github.com/openshift/api/route/v1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	certmngrv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	// "k8s.io/apimachinery/pkg/runtime"
)

// RuntimeCertManagerTest : ...
func RuntimeCertManagerTest(t *testing.T) {
	ctx, err := util.InitializeContext(t, cleanupTimeout, retryInterval)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Cleanup()

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("Couldn't get namespace: %v", err)
	}

	t.Logf("Namespace: %s", namespace)

	f := framework.Global

	if !util.IsCertManagerInstalled(t, f, ctx) {
		t.Log("cert manager not installed, skipping...")
		return
	}

	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "runtime-component-operator", 1, retryInterval, timeout)
	if err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = runtimePodCertTest(t, f, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = runtimeRouteCertTest(t, f, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = runtimeCustomIssuerTest(t, f, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}

	if err = runtimeExistingCertTest(t, f, ctx); err != nil {
		util.FailureCleanup(t, f, namespace, err)
	}
}

func runtimePodCertTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	const name = "example-runtime-pod-cert"

	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	runtime := util.MakeBasicRuntimeComponent(t, f, name, namespace, 1)
	runtime.Spec.Service.Certificate = &appstacksv1beta1.Certificate{}

	certName := fmt.Sprintf("%s-svc-crt", name)
	err = deployAndWaitForCertificate("Creating cert-manager pod test",
			t, f, ctx, runtime, name, namespace, certName)
	if err != nil {
		return err
	}

	return nil
}

func runtimeRouteCertTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	const name = "example-runtime-route-cert"

	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace %v", err)
	}

	runtime := util.MakeBasicRuntimeComponent(t, f, name, namespace, 1)
	terminationPolicy := routev1.TLSTerminationReencrypt
	expose := true
	runtime.Spec.Expose = &expose
	runtime.Spec.Route = &appstacksv1beta1.RuntimeComponentRoute{
		Host:        "myapp.mycompany.com",
		Termination: &terminationPolicy,
		Certificate: &appstacksv1beta1.Certificate{},
	}

	certName := fmt.Sprintf("%s-route-crt", name)
	err = deployAndWaitForCertificate("Creating cert-manager route test",
			t, f, ctx, runtime, name, namespace, certName)
	if err != nil {
		return err
	}

	target := types.NamespacedName{Name: name, Namespace: namespace}
	err = util.UpdateApplication(f, target, func(r *appstacksv1beta1.RuntimeComponent) {
		r.Spec.Route = nil
	})
	if err != nil {
		return err
	}

	// wait for the change to take effect and verify the state
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, name, 1, retryInterval, operatorTimeout)
	if err != nil {
		return err
	}

	certExists, certErr := certificateExists(f, fmt.Sprintf("%s-route-crt", name), namespace) 
	if certErr != nil {
		return certErr
	}
	if certExists {
		return errors.New("certificate persists when runtime.Spec.Route is nil")
	}

	return nil
}

func runtimeCustomIssuerTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	const name = "example-custom-issuer-cert"
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace %v", err)
	}

	err = util.CreateCertificateIssuer(t, f, ctx, "custom-issuer")
	if err != nil {
		return err
	}

	runtime := util.MakeBasicRuntimeComponent(t, f, name, namespace, 1)
	terminationPolicy := routev1.TLSTerminationReencrypt
	expose := true
	var durationTime time.Duration = 10 * time.Minute
	duration := metav1.Duration{
		Duration: durationTime,
	}
	runtime.Spec.Expose = &expose
	runtime.Spec.Route = &appstacksv1beta1.RuntimeComponentRoute{
		Host:        "myapp.mycompany.com",
		Termination: &terminationPolicy,
		Certificate: &appstacksv1beta1.Certificate{
			Duration:     &duration,
			Organization: []string{"My Company"},
			IssuerRef: cmmeta.ObjectReference{
				Name: "custom-issuer",
				Kind: "ClusterIssuer",
			},
		},
	}

	certName := fmt.Sprintf("%s-route-crt", name)
	err = deployAndWaitForCertificate("Creating cert-manager custom issuer test",
			t, f, ctx, runtime, name, namespace, certName)
	if err != nil {
		return err
	}

	return nil
}


func runtimeExistingCertTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	const name = "example-existing-cert"
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace %v", err)
	}
	secretRefName := "my-app-rt-tls"
	secret := makeCertSecret(secretRefName, namespace)
	err = f.Client.Create(goctx.TODO(), secret,
		&framework.CleanupOptions{TestContext: ctx, Timeout: time.Second, RetryInterval: time.Second})
	if err != nil {
		return err
	}

	runtime := util.MakeBasicRuntimeComponent(t, f, name, namespace, 1)
	terminationPolicy := routev1.TLSTerminationReencrypt
	expose := true
	
	runtime.Spec.Expose = &expose
	runtime.Spec.Route = &appstacksv1beta1.RuntimeComponentRoute{
		Host:        "myapp.mycompany.com",
		Termination: &terminationPolicy,
		CertificateSecretRef: &secretRefName,
	}

	timestamp := time.Now().UTC()
	t.Logf("%s - Creating cert-manager existing certificate test...", timestamp)

	err = f.Client.Create(goctx.TODO(), runtime,
		&framework.CleanupOptions{TestContext: ctx, Timeout: time.Second, RetryInterval: time.Second})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, name, 1, retryInterval, timeout)
	if err != nil {
		return err
	}
	routev1.AddToScheme(f.Scheme)
	route := &routev1.Route{}
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, route)
	if err != nil {
		return err
	}
	if route.Spec.TLS.Certificate != "faketlscrt" ||
		route.Spec.TLS.CACertificate != "fakecacrt" ||
		route.Spec.TLS.Key != "faketlskey" ||
		route.Spec.TLS.DestinationCACertificate != "fakedestca" {
		return errors.New("route.Spec.TLS fields are not set correctly")
	}

	return nil
}

/* Helper Functions Below */
func deployAndWaitForCertificate (msg string, t *testing.T, f *framework.Framework, ctx *framework.TestCtx, 
		runtime *appstacksv1beta1.RuntimeComponent, n string, ns string, certName string) error {
	timestamp := time.Now().UTC()
	t.Logf("%s - %s...", timestamp, msg)
	err := f.Client.Create(goctx.TODO(), runtime,
		&framework.CleanupOptions{TestContext: ctx, Timeout: time.Second, RetryInterval: time.Second})
	if err != nil {
		return err
	}

	err = e2eutil.WaitForDeployment(t, f.KubeClient, ns, n, 1, retryInterval, timeout)
	if err != nil {
		return err
	}

	err = util.WaitForCertificate(t, f, ns, certName, retryInterval, timeout)
	if err != nil {
		return err
	}

	return nil
}

func makeCertSecret(n string, ns string) *corev1.Secret {
	data := map[string][]byte{
		"ca.crt": []byte("fakecacrt"),
		"tls.crt": []byte("faketlscrt"),
		"tls.key": []byte("faketlskey"),
		"destCA.crt": []byte("fakedestca"),
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: n,
			Namespace: ns,
		},
		Type: "kubernetes.io/tls",
		Data: data,
	}
	return &secret
}

func certificateExists(f *framework.Framework, n string, ns string) (bool, error) {
	cert := &certmngrv1alpha2.Certificate{}
	certErr := f.Client.Get(goctx.TODO(), types.NamespacedName{Name:n, Namespace: ns}, cert)
	if certErr != nil {
		if apierrors.IsNotFound(certErr) {
			return false, nil
		}
		return false, certErr
	}
	return true, nil
}