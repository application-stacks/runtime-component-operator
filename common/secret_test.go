package common

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestSecretScopeClear(t *testing.T) {
	value := []byte("secret-value")
	buffer := []byte("buffer-value")
	scope := &SecretScope{}
	secret := scope.Track(&corev1.Secret{})
	// data assigned after Track must still be cleared
	secret.Data = map[string][]byte{"password": value}
	scope.TrackBytes(buffer)
	scope.Clear()

	if len(secret.Data) != 0 {
		t.Fatal("expected Secret data to be removed")
	}
	for _, b := range append(value, buffer...) {
		if b != 0 {
			t.Fatal("expected tracked bytes to be zeroed")
		}
	}
	ClearSecret(nil)
}

func TestGenerateRandomAlphanumeric(t *testing.T) {
	a, err := GenerateRandomAlphanumeric(15)
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateRandomAlphanumeric(15)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 15 || string(a) == string(b) {
		t.Fatalf("expected two distinct 15 byte values, got %q and %q", a, b)
	}
	for _, c := range a {
		if !strings.ContainsRune(alphanumeric, rune(c)) {
			t.Fatalf("unexpected character %q", c)
		}
	}
}

func TestSecretArg(t *testing.T) {
	if got := SecretArg("--password=", []byte("pw")); got != "--password=pw" {
		t.Fatalf("unexpected argument %q", got)
	}
}
