package common

import (
	corev1 "k8s.io/api/core/v1"
)

// OpConfig stored operator configuration
type OpConfig map[string]string

const (
	// OpConfigPropDefaultIssuer issuer to use for cert-manager certificate
	OpConfigPropDefaultIssuer = "defaultIssuer"

	// OpConfigPropUseClusterIssuer whether to use ClusterIssuer for cert-manager certificate
	OpConfigPropUseClusterIssuer = "useClusterIssuer"
)

// Config stores operator configuration
var Config = OpConfig{}

// LoadFromConfigMap creates a config out of kubernetes config map
func (oc OpConfig) LoadFromConfigMap(cm *corev1.ConfigMap) {
	for k := range oc {
		delete(oc, k)
	}
	for k, v := range cm.Data {
		oc[k] = v
	}
}

// DefaultOpConfig returns default configuration
func DefaultOpConfig() OpConfig {
	cfg := OpConfig{}
	cfg[OpConfigPropDefaultIssuer] = "self-signed"
	cfg[OpConfigPropUseClusterIssuer] = "true"
	return cfg
}
