package common

import (
	"context"
	"fmt"

	"github.com/awnumar/memguard"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LockedBufferSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	LockedData        SecretMap `json:"lockedData,omitempty"`
}

type SecretMap map[string]*memguard.LockedBuffer

func (sm SecretMap) Destroy() {
	for _, buf := range sm {
		buf.Destroy()
	}
}

func (sm SecretMap) Get(key string) ([]byte, bool) {
	if buf, found := sm[key]; found {
		return buf.Bytes(), true
	}
	return []byte{}, false
}

func (lockedBufferSecret LockedBufferSecret) Destroy() {
	if lockedBufferSecret.LockedData != nil {
		lockedBufferSecret.LockedData.Destroy()
	}
}

// Gets a Secret from the k8s client loaded as a LockedBufferSecret
func GetSecret(client client.Client, name string, ns string) (*LockedBufferSecret, error) {
	if client == nil {
		return nil, fmt.Errorf("the reconciler client could not be found")
	}
	secret := &corev1.Secret{}
	secret.Name = name
	secret.Namespace = ns
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: ns}, secret)
	if err != nil {
		return nil, err
	}

	lockedBufferSecret := &LockedBufferSecret{}
	lockedBufferSecret.TypeMeta = secret.TypeMeta
	lockedBufferSecret.ObjectMeta = secret.ObjectMeta
	for secretKey, secretValue := range secret.Data {
		lockedBufferSecret.LockedData[secretKey] = memguard.NewBufferFromBytes(secretValue)
	}
	return lockedBufferSecret, nil
}

// Copies a Locked Buffer Secret into a core Secret with a corresponding cleanup func
func CopySecret(in *LockedBufferSecret, out *corev1.Secret) func() {
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta
	for key, buf := range in.LockedData {
		out.Data[key] = buf.Bytes()
	}
	return func() {
		for key := range out.Data {
			delete(out.Data, key)
		}
		out.Data = nil
	}
}

// Returns the client.Get status of Secret name in namespace ns after clearing sensitive byte array content
func CheckSecret(client client.Client, name string, ns string) error {
	if client == nil {
		return fmt.Errorf("the reconciler client could not be found")
	}

	secret := &corev1.Secret{}
	secret.Name = name
	secret.Namespace = ns
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: ns}, secret)
	for key, value := range secret.Data {
		clear(value)
		delete(secret.Data, key)
	}
	return err
}
