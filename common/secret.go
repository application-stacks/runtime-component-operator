package common

import (
	"context"
	"crypto/rand"
	"math/big"
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
		ClearSecret(r.secret)
	})
}

// ClearSecret zeroes and removes all data held in the Secret.
func ClearSecret(secret *corev1.Secret) {
	if secret == nil {
		return
	}
	for secretKey, secretValue := range secret.Data {
		clear(secretValue)
		delete(secret.Data, secretKey)
	}
}

// SecretScope collects Secrets and byte slices so one deferred Clear zeroes them all.
type SecretScope struct {
	secrets []*corev1.Secret
	buffers [][]byte
}

// Track registers the Secret to be cleared and returns it.
func (s *SecretScope) Track(secret *corev1.Secret) *corev1.Secret {
	s.secrets = append(s.secrets, secret)
	return secret
}

// TrackBytes registers the byte slice to be zeroed and returns it.
func (s *SecretScope) TrackBytes(buffer []byte) []byte {
	s.buffers = append(s.buffers, buffer)
	return buffer
}

// Clear zeroes everything registered with the scope.
func (s *SecretScope) Clear() {
	for _, secret := range s.secrets {
		ClearSecret(secret)
	}
	for _, buffer := range s.buffers {
		clear(buffer)
	}
	s.secrets, s.buffers = nil, nil
}

const alphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

// GenerateRandomAlphanumeric returns a cryptographically secure random alphanumeric string as bytes. The caller must clear it.
func GenerateRandomAlphanumeric(length int) ([]byte, error) {
	b := make([]byte, length)
	max := big.NewInt(int64(len(alphanumeric)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			clear(b)
			return nil, err
		}
		b[i] = alphanumeric[n.Int64()]
	}
	return b, nil
}

// SecretArg returns prefix+secret as a string command line argument, clearing the intermediate buffer.
func SecretArg(prefix string, secret []byte) string {
	buf := append([]byte(prefix), secret...)
	defer clear(buf)
	return string(buf)
}
