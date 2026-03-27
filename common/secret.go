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
	for _, buffer := range sm {
		buffer.Destroy()
	}
}

func (sm SecretMap) Get(key string) ([]byte, bool) {
	if buffer, found := sm[key]; found {
		return buffer.Bytes(), true
	}
	return []byte{}, false
}

func (sm SecretMap) Set(key string, value []byte) {
	if buffer, found := sm[key]; found {
		buffer.Destroy()
	}
	sm[key] = memguard.NewBufferFromBytes(value)
}

func (lockedSecret LockedBufferSecret) Destroy() {
	if lockedSecret.LockedData != nil {
		lockedSecret.LockedData.Destroy()
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

	lockedSecret := &LockedBufferSecret{}
	lockedSecret.TypeMeta = secret.TypeMeta
	lockedSecret.ObjectMeta = secret.ObjectMeta

	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: ns}, secret)
	if err != nil {
		return lockedSecret, err
	}

	if lockedSecret.LockedData == nil {
		lockedSecret.LockedData = SecretMap{}
	}
	if secret.Data != nil {
		for secretKey, secretValue := range secret.Data {
			lockedSecret.LockedData[secretKey] = memguard.NewBufferFromBytes(secretValue)
		}
	}
	return lockedSecret, nil
}

// Copies a Locked Buffer Secret into a core Secret with a corresponding cleanup func
func CopySecret(in *LockedBufferSecret, out *corev1.Secret) func() {
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta
	if out.Data == nil {
		out.Data = map[string][]byte{}
	}
	for key, buffer := range in.LockedData {
		out.Data[key] = buffer.Bytes()
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
