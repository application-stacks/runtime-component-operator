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

// Creates a Secret that clears it's data when recCtx is canceled
func NewSecret(recCtx context.Context, name, namespace string) *corev1.Secret {
	secretResource := NewSecretResource(name, namespace)
	go func() {
		<-recCtx.Done()
		secretResource.Clear()
	}()
	return secretResource.GetSecret()
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

func (r *SecretResource) GetSecret() *corev1.Secret {
	return r.secret
}

func (r *SecretResource) Clear() {
	if r.secret == nil {
		return
	}

	r.once.Do(func() {
		for secretKey, secretValue := range r.secret.Data {
			clear(secretValue)
			delete(r.secret.Data, secretKey)
		}
	})
}
