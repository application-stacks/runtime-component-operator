package util

import (
	goctx "context"
	"os/exec"
	"testing"
	"time"

	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	certmngrv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	applicationsv1beta1 "sigs.k8s.io/application/pkg/apis/app/v1beta1"
	dynclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// MakeBasicRuntimeComponent : Create a simple RuntimeComponent with provided number of replicas.
func MakeBasicRuntimeComponent(t *testing.T, f *framework.Framework, n string, ns string, replicas int32) *appstacksv1beta1.RuntimeComponent {
	probe := corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/",
			Port: intstr.FromInt(3000),
		},
	}
	expose := false
	serviceType := corev1.ServiceTypeClusterIP
	return &appstacksv1beta1.RuntimeComponent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RuntimeComponent",
			APIVersion: "app.stacks/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      n,
			Namespace: ns,
		},
		Spec: appstacksv1beta1.RuntimeComponentSpec{
			ApplicationImage: "navidsh/demo-day",
			Replicas:         &replicas,
			Expose:           &expose,
			Service: &appstacksv1beta1.RuntimeComponentService{
				Port: 3000,
				Type: &serviceType,
			},
			ReadinessProbe: &corev1.Probe{
				Handler:             probe,
				InitialDelaySeconds: 1,
				TimeoutSeconds:      1,
				PeriodSeconds:       5,
				SuccessThreshold:    1,
				FailureThreshold:    16,
			},
			LivenessProbe: &corev1.Probe{
				Handler:             probe,
				InitialDelaySeconds: 4,
				TimeoutSeconds:      1,
				PeriodSeconds:       5,
				SuccessThreshold:    1,
				FailureThreshold:    6,
			},
		},
	}
}

// WaitForStatefulSet : Identical to WaitForDeployment but for StatefulSets.
func WaitForStatefulSet(t *testing.T, kc kubernetes.Interface, ns, n string, replicas int, retryInterval, timeout time.Duration) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		statefulset, err := kc.AppsV1().StatefulSets(ns).Get(n, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s statefulset\n", n)
				return false, nil
			}
			return false, err
		}

		if int(statefulset.Status.ReadyReplicas) == replicas {
			return true, nil
		}
		t.Logf("Waiting for full availability of %s statefulset (%d/%d)\n", n, statefulset.Status.CurrentReplicas, replicas)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("StatefulSet available (%d/%d)\n", replicas, replicas)
	return nil
}

// InitializeContext : Sets up initial context
func InitializeContext(t *testing.T, clean, retryInterval time.Duration) (*framework.TestCtx, error) {
	ctx := framework.NewTestCtx(t)
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{
		TestContext:   ctx,
		Timeout:       clean,
		RetryInterval: retryInterval,
	})
	if err != nil {
		if ctx != nil {
			ctx.Cleanup()
		}
		return nil, err
	}

	t.Log("Cluster context initialized.")
	return ctx, nil
}

// FailureCleanup : Log current state of the namespace and exit with fatal
func FailureCleanup(t *testing.T, f *framework.Framework, ns string, failure error) {
	t.Log("***** FAILURE")
	t.Logf("ERROR: %v", failure)
	t.Log("*****")
	options := &dynclient.ListOptions{
		Namespace: ns,
	}
	podlist := &corev1.PodList{}
	err := f.Client.List(goctx.TODO(), podlist, options)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("***** Logging pods in namespace: %s", ns)
	for _, p := range podlist.Items {
		t.Log("------------------------------------------------------------")
		t.Log(p)
	}

	crlist := &appstacksv1beta1.RuntimeComponentList{}
	err = f.Client.List(goctx.TODO(), crlist, options)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("***** Logging Runtime Components in namespace: %s", ns)
	for _, application := range crlist.Items {
		t.Log("-----------------------------------------------------------")
		t.Log(application)
	}

	t.FailNow()
}

// WaitForKnativeDeployment : Poll for ksvc creation when createKnativeService is set to true
func WaitForKnativeDeployment(t *testing.T, f *framework.Framework, ns, n string, retryInterval, timeout time.Duration) error {
	// add to scheme to framework can find the resource
	err := servingv1alpha1.AddToScheme(f.Scheme)
	if err != nil {
		return err
	}

	err = wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		ksvc := &servingv1alpha1.ServiceList{}
		lerr := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: n, Namespace: ns}, ksvc)
		if lerr != nil {
			if apierrors.IsNotFound(lerr) {
				t.Logf("waiting for knative service %s...", n)
				return false, nil
			}
			// issue retrieving ksvc
			return false, lerr
		}

		t.Logf("found knative service %s", n)
		return true, nil
	})
	return err
}

// IsKnativeServiceDeployed : Check if the Knative service is deployed.
func IsKnativeServiceDeployed(t *testing.T, f *framework.Framework, ns, n string) (bool, error) {
	err := servingv1alpha1.AddToScheme(f.Scheme)
	if err != nil {
		return false, err
	}

	ksvc := &servingv1alpha1.ServiceList{}
	lerr := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: n, Namespace: ns}, ksvc)
	if lerr != nil {
		if apierrors.IsNotFound(lerr) {
			return false, nil
		}
		return false, lerr
	}

	return true, nil
}

func IsCertManagerInstalled(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) bool {
	certmngrv1alpha2.AddToScheme(f.Scheme)

	ns, _ := ctx.GetNamespace()

	certIssuer := &certmngrv1alpha2.Issuer{}

	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "name", Namespace: ns}, certIssuer)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			t.Log(err)
			return false
		}
	}

	return true
}

func CreateCertificateIssuer(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, n string) error {
	err := certmngrv1alpha2.AddToScheme(f.Scheme)
	if err != nil {
		return err
	}

	ns, _ := ctx.GetNamespace()

	certIssuer := &certmngrv1alpha2.ClusterIssuer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterIssuer",
			APIVersion: "cert-manager.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      n,
			Namespace: ns,
		},
		Spec: certmngrv1alpha2.IssuerSpec{
			IssuerConfig: certmngrv1alpha2.IssuerConfig{
				SelfSigned: &certmngrv1alpha2.SelfSignedIssuer{},
			},
		},
	}
	err = f.Client.Create(goctx.TODO(), certIssuer, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second, RetryInterval: time.Second})
	if err != nil {
		t.Log("failed to create cert issuer!")
		return err
	}

	t.Log("cert issuer successfully created!")
	return nil
}

// WaitForCertificate : Poll for generated certificates from our cert manager functionality
func WaitForCertificate(t *testing.T, f *framework.Framework, ns, n string, retryInterval, timeout time.Duration) error {
	err := certmngrv1alpha2.AddToScheme(f.Scheme)
	if err != nil {
		return err
	}

	err = wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		cert := &certmngrv1alpha2.Certificate{}
		certErr := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: n, Namespace: ns}, cert)
		if certErr != nil {
			if apierrors.IsNotFound(certErr) {
				t.Logf("waiting for certificate %s...", n)
				return false, nil
			}
			t.Log("new error encountered")
			return true, certErr
		}

		t.Logf("found certificate %s", n)
		return true, nil
	})

	return err
}

// UpdateApplication updates target app using provided function, retrying in the case that status has changed
func UpdateApplication(f *framework.Framework, target types.NamespacedName, update func(r *appstacksv1beta1.RuntimeComponent)) error {
	retryInterval := time.Second * 5
	timeout := time.Second * 30
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		temp := &appstacksv1beta1.RuntimeComponent{}
		err = f.Client.Get(goctx.TODO(), target, temp)
		if err != nil {
			return true, err
		}

		update(temp)

		err = f.Client.Update(goctx.TODO(), temp)
		if err != nil {
			return false, err
		}

		return true, nil
	})

	return err
}

// WaitForApplicationDelete wait for kappnav to delete the generated application
func WaitForApplicationDelete(t *testing.T, f *framework.Framework, target types.NamespacedName) error {
	retryInterval := time.Second * 5
	timeout := time.Second * 30
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		application := &applicationsv1beta1.Application{}

		if err := f.Client.Get(goctx.TODO(), target, application); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return true, err
		}

		t.Logf("application '%s' not deleted, waiting...", target.Name)
		return false, nil
	})

	return err
}

// WaitForApplicationCreated wait for kappnav to create the generated application
func WaitForApplicationCreated(t *testing.T, f *framework.Framework, target types.NamespacedName) error {
	retryInterval := time.Second * 5
	timeout := time.Second * 30
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		application := &applicationsv1beta1.Application{}

		if err := f.Client.Get(goctx.TODO(), target, application); err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("application '%s' not found, waiting...", target.Name)
				return false, nil
			}
			return true, err
		}

		return true, nil
	})

	return err
}

func CreateApplicationTarget(f *framework.Framework, ctx *framework.TestCtx, target types.NamespacedName, l map[string]string) error {
	ns, err := ctx.GetNamespace()
	if err != nil {
		return err
	}

	application := &applicationsv1beta1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      target.Name,
			Namespace: target.Namespace,
			Annotations: map[string]string{
				"kappnav.component.namespaces": ns,
			},
		},
		Spec: applicationsv1beta1.ApplicationSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: l,
			},
		},
	}
	timeout := time.Second * 30
	retryInterval := time.Second * 1

	err = f.Client.Create(goctx.TODO(), application, &framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		return err
	}

	return nil
}

// CommandError : Reports back an error if a command fails to execute
func CommandError(t *testing.T, err error, out []byte) error {
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			t.Log(exiterr.Error())
			return exiterr
		}
		t.Log("unknown error occurred, see below")
		t.Log(err.Error())
		return err
	}
	return nil
}

func IsKnativeInstalled(t *testing.T, f *framework.Framework) error {
	ksvc := servingv1alpha1.Service{}
	err := servingv1alpha1.AddToScheme(f.Scheme)
	if err != nil {
		return err
	}
	
	err = f.Client.List(goctx.TODO(), &ksvc)
	if err != nil {
		t.Log(err)
		return err
	}

	return nil
}
