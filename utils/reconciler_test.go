package utils

import (
	"context"
	"fmt"
	appstacksv1 "github.com/application-stacks/runtime-component-operator/api/v1"
	"reflect"
	"strings"
	"testing"

	"github.com/application-stacks/runtime-component-operator/common"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	coretesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	servingv1alpha1 "knative.dev/serving/pkg/apis/serving/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	defaultMeta = metav1.ObjectMeta{
		Name:      "app",
		Namespace: "runtimecomponent",
	}
	spec = appstacksv1.RuntimeComponentSpec{}
)

const (
	routeCrtExpected = `-----BEGIN CERTIFICATE-----
MIIDJDCCAgygAwIBAgIQYjfKtSv5Ky2e9eT6lJGmSDANBgkqhkiG9w0BAQsFADA8
MQwwCgYDVQQKEwNJQk0xEjAQBgNVBAsTCVdlYlNwaGVyZTEYMBYGA1UEAxMPV0xP
IFRlc3QgSW50IENBMCAXDTIzMDQxMTIwMjcxOFoYDzIwNTMwMzMwMDYyNzE4WjAb
MRkwFwYDVQQDExBUZXN0IGNlcnRpZmljYXRlMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAruaFXcJ5/VYJKxGQbLbBcVv7sfmpU3XkXtIOSyUKENp02gnF
CDvwQKN6CFJHwvabflSSkMGPoshqHMlX9X2QUJRI9MqoI9iSTz0NY6t/2yedrK+0
vHzQrayD9UeEHWAyNKw794TB/9haUQQ0Ehp5jGFqk/p/U2g8CTyKM/41e3w2OnMA
HAfj4j1YMxDZ6jnxA3L6hGuAJwq+bg48I0xHx9cGEtQ9s4seKqaWeSGrlxwNW6up
4e19IcK1wHw+Kr3Nz4Wp2xBMLxae7632jrwkzWwsHankVYbo/ldqT7bXtKXB3vsZ
Fekqv/rPH8zdIN3absSlX+79VCokR95JrxjkFwIDAQABo0EwPzAOBgNVHQ8BAf8E
BAMCBaAwDAYDVR0TAQH/BAIwADAfBgNVHSMEGDAWgBQwrZjlaIwdBnegir6ZX4rf
wg8YQzANBgkqhkiG9w0BAQsFAAOCAQEASccwVEJ5L18vjkmkUGcZOuGZfchM4mqY
pHSQpJEmRalKE6Ci9mhUY9ijHVx19h8JYCUycK7shK2a024Jxj80tgUH7lvt3CUU
3fka3H8rqenGfYvKcQGu4/sp5G6C7Urt73y05n4itqojqX/EH5ie5lVCmnLaTD5O
rGbu5/wxlCL7U5pOE6AHOK8rHryRNIcy5WUmEtg834s68GOzU3lURDveITSRxH9U
FXuigxlTSyvbs4Kb/KIuZVo71IKvmg19NW086lja1NlI/Cvhiz6G7lzWZHYkWFAT
ytPQmBKGWBDeEph/kBi52auhlh1cpBguzXSufe0vB159nk6I+O43aQ==
-----END CERTIFICATE-----`
	tlsCrt = `-----BEGIN CERTIFICATE-----
MIIDJDCCAgygAwIBAgIQYjfKtSv5Ky2e9eT6lJGmSDANBgkqhkiG9w0BAQsFADA8
MQwwCgYDVQQKEwNJQk0xEjAQBgNVBAsTCVdlYlNwaGVyZTEYMBYGA1UEAxMPV0xP
IFRlc3QgSW50IENBMCAXDTIzMDQxMTIwMjcxOFoYDzIwNTMwMzMwMDYyNzE4WjAb
MRkwFwYDVQQDExBUZXN0IGNlcnRpZmljYXRlMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAruaFXcJ5/VYJKxGQbLbBcVv7sfmpU3XkXtIOSyUKENp02gnF
CDvwQKN6CFJHwvabflSSkMGPoshqHMlX9X2QUJRI9MqoI9iSTz0NY6t/2yedrK+0
vHzQrayD9UeEHWAyNKw794TB/9haUQQ0Ehp5jGFqk/p/U2g8CTyKM/41e3w2OnMA
HAfj4j1YMxDZ6jnxA3L6hGuAJwq+bg48I0xHx9cGEtQ9s4seKqaWeSGrlxwNW6up
4e19IcK1wHw+Kr3Nz4Wp2xBMLxae7632jrwkzWwsHankVYbo/ldqT7bXtKXB3vsZ
Fekqv/rPH8zdIN3absSlX+79VCokR95JrxjkFwIDAQABo0EwPzAOBgNVHQ8BAf8E
BAMCBaAwDAYDVR0TAQH/BAIwADAfBgNVHSMEGDAWgBQwrZjlaIwdBnegir6ZX4rf
wg8YQzANBgkqhkiG9w0BAQsFAAOCAQEASccwVEJ5L18vjkmkUGcZOuGZfchM4mqY
pHSQpJEmRalKE6Ci9mhUY9ijHVx19h8JYCUycK7shK2a024Jxj80tgUH7lvt3CUU
3fka3H8rqenGfYvKcQGu4/sp5G6C7Urt73y05n4itqojqX/EH5ie5lVCmnLaTD5O
rGbu5/wxlCL7U5pOE6AHOK8rHryRNIcy5WUmEtg834s68GOzU3lURDveITSRxH9U
FXuigxlTSyvbs4Kb/KIuZVo71IKvmg19NW086lja1NlI/Cvhiz6G7lzWZHYkWFAT
ytPQmBKGWBDeEph/kBi52auhlh1cpBguzXSufe0vB159nk6I+O43aQ==
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIDYzCCAkugAwIBAgIQHUhe9rTL8xqZSBWoWMAKlzANBgkqhkiG9w0BAQsFADA4
MQwwCgYDVQQKEwNJQk0xEjAQBgNVBAsTCVdlYlNwaGVyZTEUMBIGA1UEAxMLV0xP
IFRlc3QgQ0EwIBcNMjMwNDExMTkxOTE5WhgPMjA1MzA0MDMwOTE5MTlaMDwxDDAK
BgNVBAoTA0lCTTESMBAGA1UECxMJV2ViU3BoZXJlMRgwFgYDVQQDEw9XTE8gVGVz
dCBJbnQgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC22iIaO0Or
cIjre8vYw97GpLLG9Hk8ini94Ac5ndBRqP6Ft/VYTxRSq79K9NdVY81FaPgpXAG1
dkXMRB6AV5ksb7rW3Pd05LxYIPwh+scuMQshLO6PC+5grJnVjGDqsVbZutKQrUXs
jB6ZabwdY+FzL9l7CKYr96aDnqw24XzAWO7oLCfO6UlT7E2RuqPsDgCmVi6fUjli
yqegJk7XO1wNTTGF0PwxtnVvTxXfj2yxWi+LYpdj3vS/Utooo6VuadXHZLQROAdi
0C+yGRlAW3rSDHD5RC9I1QxpdZ6OI5fVUow0LtGpu+xT93yCQg/NBb+BEAuPfl6B
Xilc23lQRsjNAgMBAAGjYzBhMA4GA1UdDwEB/wQEAwICpDAPBgNVHRMBAf8EBTAD
AQH/MB0GA1UdDgQWBBQwrZjlaIwdBnegir6ZX4rfwg8YQzAfBgNVHSMEGDAWgBRO
O94eOb0mqo2C1+FsGP8fPHqsnTANBgkqhkiG9w0BAQsFAAOCAQEAlnb2K4bCWmVf
SWIhd2n4uXkqZZ0jv4sdDyB9E7bEZVcP9LOrd2y9AzEOoj60PH34CeiAASiAdnsA
hkMhfeuuhXqmiScRZOnpGv+7Zn2QtEzuyQR4jWypbazu7f/o3/PsCT/QWHF5wjby
iRQI8vOB8plJMHlEo5+VZWwQgwVliiLH+BoOsSUgAxwfJckTfHvJ+w2G0heLly93
tRanvHec4tOuWG+W/ndFgjUuN4ruGlwOQp1cDMqhyFLlUCxcvOPVJfQETuq09G3w
xShDUu0sZVfPbLCPBclFHqBo1hwX4O5QxF8ZhuQf2HQDzENnUSRLsZ2KQ0xsiB3K
O3reDUrwAw==
-----END CERTIFICATE-----`
	tlsKey = `fakekey`
	caCrt  = `-----BEGIN CERTIFICATE-----
MIIDPjCCAiagAwIBAgIQP2PJh7eh2XwB44qQWNrQQjANBgkqhkiG9w0BAQsFADA4
MQwwCgYDVQQKEwNJQk0xEjAQBgNVBAsTCVdlYlNwaGVyZTEUMBIGA1UEAxMLV0xP
IFRlc3QgQ0EwIBcNMjMwNDExMTkxNjIzWhgPMjA1MzA0MDMxOTE2MjNaMDgxDDAK
BgNVBAoTA0lCTTESMBAGA1UECxMJV2ViU3BoZXJlMRQwEgYDVQQDEwtXTE8gVGVz
dCBDQTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALJqE2F7Zxz99QX3
TsZeZXFjuUCkpmgzxak4bx3LrrIHuMLANkLiRwPfx/eYAHUKoyFVPSomYbsO4Ism
W9oMft371f8gTYw4Fhtd1CBHqUjvS2eG1cQvyp0BpyLffutXMYszt3xlvDOvJPi/
Pi5cviPs1mOah6vWw0U/4/9bxatmBHm9fOBfwAfkobNPapgEO6dq9PgazjsnKFEX
u4qCYB5tCOoAigs9JbJKkftz7lkIUNV/j57eoKXWhZ7jl0yIAobF88UvufsmvHl1
6TOG+NI9x2Lio4ktrMR84CCUtVfmlmwozZc94lAZzbgz+oFZ9Sqe2Lw5zhxsc8gy
f85pla0CAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8w
HQYDVR0OBBYEFE473h45vSaqjYLX4WwY/x88eqydMA0GCSqGSIb3DQEBCwUAA4IB
AQCK8UUbsylNruQ7BiUvwS3juuDnwOetQUl8nfiYfjGxI16DnTyxm1U8gqJfe56y
1cHx7mCEp/S4Nzt0mHjmaXowef6m+oz/rWgYYUSCs1SV7acCtuWMw16g8Hw9Uf1o
HQ5/wVdxLQSdyk9aMmxc/J/3je6luwGQOQH2Atpy0sR+YnJrGLXELElyBdz5UVpG
EO/bg7zXzFr+MEyEjnUOV+iDlZ2fny3jDFDZDCbSxaS8ZjlU9yqrBQuHncYJtAhD
dSfynBwa4E2wECsYlpC9Kn39FhGYGNrgPEExtCsyRgTuexCag7hNCFMB3LqBjQow
hQRsBqxwjnK5AFLH7xQyhoWc
-----END CERTIFICATE-----`
	caCertExpected = `-----BEGIN CERTIFICATE-----
MIIDPjCCAiagAwIBAgIQP2PJh7eh2XwB44qQWNrQQjANBgkqhkiG9w0BAQsFADA4
MQwwCgYDVQQKEwNJQk0xEjAQBgNVBAsTCVdlYlNwaGVyZTEUMBIGA1UEAxMLV0xP
IFRlc3QgQ0EwIBcNMjMwNDExMTkxNjIzWhgPMjA1MzA0MDMxOTE2MjNaMDgxDDAK
BgNVBAoTA0lCTTESMBAGA1UECxMJV2ViU3BoZXJlMRQwEgYDVQQDEwtXTE8gVGVz
dCBDQTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALJqE2F7Zxz99QX3
TsZeZXFjuUCkpmgzxak4bx3LrrIHuMLANkLiRwPfx/eYAHUKoyFVPSomYbsO4Ism
W9oMft371f8gTYw4Fhtd1CBHqUjvS2eG1cQvyp0BpyLffutXMYszt3xlvDOvJPi/
Pi5cviPs1mOah6vWw0U/4/9bxatmBHm9fOBfwAfkobNPapgEO6dq9PgazjsnKFEX
u4qCYB5tCOoAigs9JbJKkftz7lkIUNV/j57eoKXWhZ7jl0yIAobF88UvufsmvHl1
6TOG+NI9x2Lio4ktrMR84CCUtVfmlmwozZc94lAZzbgz+oFZ9Sqe2Lw5zhxsc8gy
f85pla0CAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8w
HQYDVR0OBBYEFE473h45vSaqjYLX4WwY/x88eqydMA0GCSqGSIb3DQEBCwUAA4IB
AQCK8UUbsylNruQ7BiUvwS3juuDnwOetQUl8nfiYfjGxI16DnTyxm1U8gqJfe56y
1cHx7mCEp/S4Nzt0mHjmaXowef6m+oz/rWgYYUSCs1SV7acCtuWMw16g8Hw9Uf1o
HQ5/wVdxLQSdyk9aMmxc/J/3je6luwGQOQH2Atpy0sR+YnJrGLXELElyBdz5UVpG
EO/bg7zXzFr+MEyEjnUOV+iDlZ2fny3jDFDZDCbSxaS8ZjlU9yqrBQuHncYJtAhD
dSfynBwa4E2wECsYlpC9Kn39FhGYGNrgPEExtCsyRgTuexCag7hNCFMB3LqBjQow
hQRsBqxwjnK5AFLH7xQyhoWc
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIDYzCCAkugAwIBAgIQHUhe9rTL8xqZSBWoWMAKlzANBgkqhkiG9w0BAQsFADA4
MQwwCgYDVQQKEwNJQk0xEjAQBgNVBAsTCVdlYlNwaGVyZTEUMBIGA1UEAxMLV0xP
IFRlc3QgQ0EwIBcNMjMwNDExMTkxOTE5WhgPMjA1MzA0MDMwOTE5MTlaMDwxDDAK
BgNVBAoTA0lCTTESMBAGA1UECxMJV2ViU3BoZXJlMRgwFgYDVQQDEw9XTE8gVGVz
dCBJbnQgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC22iIaO0Or
cIjre8vYw97GpLLG9Hk8ini94Ac5ndBRqP6Ft/VYTxRSq79K9NdVY81FaPgpXAG1
dkXMRB6AV5ksb7rW3Pd05LxYIPwh+scuMQshLO6PC+5grJnVjGDqsVbZutKQrUXs
jB6ZabwdY+FzL9l7CKYr96aDnqw24XzAWO7oLCfO6UlT7E2RuqPsDgCmVi6fUjli
yqegJk7XO1wNTTGF0PwxtnVvTxXfj2yxWi+LYpdj3vS/Utooo6VuadXHZLQROAdi
0C+yGRlAW3rSDHD5RC9I1QxpdZ6OI5fVUow0LtGpu+xT93yCQg/NBb+BEAuPfl6B
Xilc23lQRsjNAgMBAAGjYzBhMA4GA1UdDwEB/wQEAwICpDAPBgNVHRMBAf8EBTAD
AQH/MB0GA1UdDgQWBBQwrZjlaIwdBnegir6ZX4rfwg8YQzAfBgNVHSMEGDAWgBRO
O94eOb0mqo2C1+FsGP8fPHqsnTANBgkqhkiG9w0BAQsFAAOCAQEAlnb2K4bCWmVf
SWIhd2n4uXkqZZ0jv4sdDyB9E7bEZVcP9LOrd2y9AzEOoj60PH34CeiAASiAdnsA
hkMhfeuuhXqmiScRZOnpGv+7Zn2QtEzuyQR4jWypbazu7f/o3/PsCT/QWHF5wjby
iRQI8vOB8plJMHlEo5+VZWwQgwVliiLH+BoOsSUgAxwfJckTfHvJ+w2G0heLly93
tRanvHec4tOuWG+W/ndFgjUuN4ruGlwOQp1cDMqhyFLlUCxcvOPVJfQETuq09G3w
xShDUu0sZVfPbLCPBclFHqBo1hwX4O5QxF8ZhuQf2HQDzENnUSRLsZ2KQ0xsiB3K
O3reDUrwAw==
-----END CERTIFICATE-----`
	destCACrt = "fakedestcacrt"
)

func TestGetDiscoveryClient(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	newDC, err := r.GetDiscoveryClient()

	if newDC == nil {
		t.Fatalf("GetDiscoverClient did not create a new discovery client. newDC: (%v) err: (%v)", newDC, err)
	}
}

func TestCreateOrUpdate(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: defaultMeta}

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	err := r.CreateOrUpdate(serviceAccount, runtimecomponent, func() error {
		CustomizeServiceAccount(serviceAccount, runtimecomponent, r.GetClient())
		return nil
	})

	testCOU := []Test{{"CreateOrUpdate error is nil", nil, err}}
	verifyTests(testCOU, t)
}

func TestDeleteResources(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)
	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	r.SetDiscoveryClient(createFakeDiscoveryClient())
	nsn := types.NamespacedName{Name: "app", Namespace: "runtimecomponent"}

	sa := &corev1.ServiceAccount{ObjectMeta: defaultMeta}
	ss := &appsv1.StatefulSet{ObjectMeta: defaultMeta}
	roList := []client.Object{sa, ss}

	if err := r.GetClient().Create(context.TODO(), sa); err != nil {
		t.Fatalf("Create ServiceAccount: (%v)", err)
	}

	if err := r.GetClient().Get(context.TODO(), nsn, sa); err != nil {
		t.Fatalf("Get ServiceAccount (%v)", err)
	}

	if err := r.GetClient().Create(context.TODO(), ss); err != nil {
		t.Fatalf("Create StatefulSet: (%v)", err)
	}

	if err := r.GetClient().Get(context.TODO(), nsn, ss); err != nil {
		t.Fatalf("Get StatefulSet (%v)", err)
	}

	// Delete Resources
	r.DeleteResources(roList)

	if err := r.GetClient().Get(context.TODO(), nsn, sa); err == nil {
		t.Fatalf("ServiceAccount was not deleted")
	}

	if err := r.GetClient().Get(context.TODO(), nsn, ss); err == nil {
		t.Fatalf("StatefulSet was not deleted")
	}
}

func TestGetOpConfigMap(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data: map[string]string{
			stack: `{"expose":true, "service":{"port": 3000,"type": "ClusterIP"}}`,
		},
	}

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := cl

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	if err := r.GetClient().Create(context.TODO(), configMap); err != nil {
		t.Fatalf("Create configMap: (%v)", err)
	}

	cm, err := r.GetOpConfigMap(name, namespace)

	testGAOCM := []Test{
		{"GetOpConfigMap error is nil", nil, err},
		{"GetOpConfigMap ConfigMap is correct", true, reflect.DeepEqual(cm.Data, configMap.Data)},
	}
	verifyTests(testGAOCM, t)
}

func TestManageError(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)
	err := fmt.Errorf("test-error")

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	rec, err := r.ManageError(err, common.StatusConditionTypeReconciled, runtimecomponent)

	testME := []Test{
		{"ManageError Requeue", true, rec.Requeue},
		{"ManageError New Condition Status", corev1.ConditionFalse, runtimecomponent.Status.Conditions[0].Status},
	}
	verifyTests(testME, t)
}

func TestManageSuccess(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)
	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	r.ManageSuccess(common.StatusConditionTypeReconciled, runtimecomponent)

	testMS := []Test{
		{"ManageSuccess New Condition Status", corev1.ConditionTrue, runtimecomponent.Status.Conditions[0].Status},
	}
	verifyTests(testMS, t)
}

func TestIsGroupVersionSupported(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)

	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))
	fakeDiscoveryClient := &fakediscovery.FakeDiscovery{
		Fake: &coretesting.Fake{Resources: []*metav1.APIResourceList{
			{
				GroupVersion: "abc/v1",
				APIResources: []metav1.APIResource{
					{Kind: "Test", Name: "tests", Group: "abc", Version: "v1"},
				}}}},
	}
	r.SetDiscoveryClient(fakeDiscoveryClient)

	ok, err := r.IsGroupVersionSupported("abc/v1", "Test")
	if err != nil && ok {
		t.Fatalf("Group version should be supported: (%v)", err)
	}

	ok, err = r.IsGroupVersionSupported("abc/v1", "Abc")
	if err == nil && ok {
		t.Fatalf("Group version should not be supported")
	}

	ok, err = r.IsGroupVersionSupported("abc/v2", "Test")
	if err == nil && ok {
		t.Fatalf("Group version should not be supported")
	}
}

// testGetSvcTLSValues test part of the function GetRouteTLSValues in reconciler.go.
func testGetSvcTLSValues(t *testing.T) {
	// Configure the runtime component

	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	expose := true
	runtimecomponent.Spec.Expose = &expose
	runtimecomponent.Spec.Service = &appstacksv1.RuntimeComponentService{
		Port: 3000,
	}

	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)

	// Deploy the expected secret
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)
	secret := makeCertSecret("my-app-svc-tls", namespace)
	runtimecomponent.Spec.Service.CertificateSecretRef = &secret.Name
	if err := cl.Create(context.TODO(), secret); err != nil {
		t.Fatal(err)
	}

	runtimecomponent.Status.SetReference(common.StatusReferenceCertSecretName, secret.Name)

	// Use the reconciler to retrieve the secret
	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))
	key, cert, ca, destCa, err := r.GetRouteTLSValues(runtimecomponent)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the results where only destCa should be retrieved.
	testSTV := []Test{
		{"Svc TLS Value - Key", "", key},
		{"Svc TLS Value - Cert", "", cert},
		{"Svc TLS Value - CA Cert", "", ca},
		{"Svc TLS Value - Dest CA Cert", caCrt, destCa},
	}
	verifyTests(testSTV, t)
}

// testGetRouteTLSValues test the function GetRouteTLSValues in reconciler.go.
func testGetRouteTLSValues(t *testing.T) {
	// Configure the rumtime component
	runtimecomponent := createRuntimeComponent(name, namespace, spec)
	terminationPolicy := routev1.TLSTerminationReencrypt
	secretRefName := "my-app-route-tls"
	runtimecomponent.Spec.Expose = &expose
	manageTLS := false
	runtimecomponent.Spec.ManageTLS = &manageTLS
	runtimecomponent.Spec.Route = &appstacksv1.RuntimeComponentRoute{
		Host:                 "myapp.mycompany.com",
		Termination:          &terminationPolicy,
		CertificateSecretRef: &secretRefName,
	}
	objs, s := []runtime.Object{runtimecomponent}, scheme.Scheme
	s.AddKnownTypes(appstacksv1.GroupVersion, runtimecomponent)

	// Create a fake client and a reconciler
	cl := fakeclient.NewFakeClient(objs...)
	rcl := fakeclient.NewFakeClient(objs...)
	r := NewReconcilerBase(rcl, cl, s, &rest.Config{}, record.NewFakeRecorder(10))

	// Make and deploy the secret for later retrieval
	secret := makeCertSecret(secretRefName, namespace)
	if err := cl.Create(context.TODO(), secret); err != nil {
		t.Fatal(err)
	}

	// Use the reconciler to retrieve the secret
	key, cert, ca, destCa, err := r.GetRouteTLSValues(runtimecomponent)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the result
	testRTV := []Test{
		{"Route TLS Value - Key", key, tlsKey},
		{"Route TLS Value - Cert", strings.Trim(cert, "\n"), routeCrtExpected},
		{"Route TLS Value - CA Cert", ca, caCertExpected},
		{"Route TLS Value - Dest CA Cert", destCa, destCACrt},
	}
	verifyTests(testRTV, t)
}

func TestGetRouteTLSValues(t *testing.T) {
	logger := zap.New()
	logf.SetLogger(logger)
	// Test two scenarios: retrieving secret from service and retrieving secret from route
	testGetSvcTLSValues(t)
	testGetRouteTLSValues(t)
}

func createFakeDiscoveryClient() discovery.DiscoveryInterface {
	fakeDiscoveryClient := &fakediscovery.FakeDiscovery{Fake: &coretesting.Fake{}}
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: routev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "routes", Namespaced: true, Kind: "Route"},
			},
		},
		{
			GroupVersion: servingv1alpha1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "services", Namespaced: true, Kind: "Service", SingularName: "service"},
			},
		},
	}

	return fakeDiscoveryClient
}

// makeCertSecret returns a pointer to a simple Secret object with fake values inside.
func makeCertSecret(n string, ns string) *corev1.Secret {
	data := map[string][]byte{
		"ca.crt":     []byte(caCrt),
		"tls.crt":    []byte(tlsCrt),
		"tls.key":    []byte(tlsKey),
		"destCA.crt": []byte(destCACrt),
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n,
			Namespace: ns,
		},
		Type: "kubernetes.io/tls",
		Data: data,
	}
	return &secret
}
