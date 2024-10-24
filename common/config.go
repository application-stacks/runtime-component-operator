package common

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
)

const (

	// OpConfigDefaultHostname a DNS name to be used for hostname generation.
	OpConfigDefaultHostname = "defaultHostname"

	// OpConfigCMCADuration default duration for cert-manager issued CA
	OpConfigCMCADuration = "certManagerCACertDuration"

	// OpConfigCMCADuration default duration for cert-manager issued service certificate
	OpConfigCMCertDuration = "certManagerCertDuration"
)

// Config stores operator configuration
var Config *sync.Map

func init() {
	Config = &sync.Map{}
}

// LoadFromConfigMap creates a config out of kubernetes config map
func LoadFromConfigMap(oc *sync.Map, cm *corev1.ConfigMap) {
	cfg := DefaultOpConfig()
	cfg.Range(func(key, value interface{}) bool {
		oc.Store(key, value)
		return true
	})
	for k, v := range cm.Data {
		oc.Store(k, v)
	}
}

// DefaultOpConfig returns default configuration
func DefaultOpConfig() *sync.Map {
	cfg := &sync.Map{}
	cfg.Store(OpConfigDefaultHostname, "")
	cfg.Store(OpConfigCMCADuration, "8766h")
	cfg.Store(OpConfigCMCertDuration, "2160h")
	return cfg
}
