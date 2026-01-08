package common

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretResource struct {
	secret *corev1.Secret
	once   sync.Once
}

type WaitableSecretResource struct {
	*SecretResource
	wg sync.WaitGroup
}

// Creates a Secret that clears it's data when recCtx is canceled
func NewSecret(recCtx context.Context, name, namespace string) *corev1.Secret {
	secretResource := NewSecretResource(name, namespace)
	go func() {
		<-recCtx.Done()
		secretResource.Clear(nil)
	}()
	return secretResource.GetSecret()
}

func NewWaitableSecret(recCtx context.Context, name, namespace string) (*corev1.Secret, *sync.WaitGroup) {
	waitableSecretResource := NewWaitableSecretResource(name, namespace)
	go func() {
		<-recCtx.Done()
		waitableSecretResource.Clear(waitableSecretResource.GetWaitGroup())
	}()
	return waitableSecretResource.GetSecret(), waitableSecretResource.GetWaitGroup()
}

func NewSecretResource(name, namespace string) *SecretResource {
	resource := &SecretResource{
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
	}
	return resource
}

func NewWaitableSecretResource(name, namespace string) *WaitableSecretResource {
	resource := &WaitableSecretResource{
		SecretResource: NewSecretResource(name, namespace),
		wg:             sync.WaitGroup{},
	}
	return resource
}

func (wsr *WaitableSecretResource) GetWaitGroup() *sync.WaitGroup {
	return &wsr.wg
}

func (r *SecretResource) GetSecret() *corev1.Secret {
	return r.secret
}

func (r *SecretResource) Clear(wg *sync.WaitGroup) {
	if r.secret == nil {
		return
	}

	r.once.Do(func() {
		if wg != nil {
			wg.Wait()
		}
		for secretKey, secretValue := range r.secret.Data {
			clear(secretValue)
			delete(r.secret.Data, secretKey)
		}
	})
}
