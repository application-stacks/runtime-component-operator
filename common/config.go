package common

import (
	corev1 "k8s.io/api/core/v1"
)

// OpConfig stored operator configuration
type OpConfig map[string]string

const (

	// OpConfigDefaultHostname a DNS name to be used for hostname generation.
	OpConfigDefaultHostname = "defaultHostname"

	// OpConfigCMCADuration default duration for cert-manager issued CA
	OpConfigCMCADuration = "certManagerCACertDuration"

	// OpConfigCMCADuration default duration for cert-manager issued service certificate
	OpConfigCMCertDuration = "certManagerCertDuration"

	// OpConfigReconcileInterval default reconciliation interval in seconds
	OpConfigReconcileInterval = "reconcileInterval"

	// OpConfigReconcileIntervalIncrease default reconciliation interval increase, represented as a percentage (1 equaling to 100%)
	// When the reconciliation needs to increase, it will increase by the given percentage
	OpConfigReconcileIntervalIncrease = "reconcileIntervalIncrease"
)

// Config stores operator configuration
var Config = OpConfig{}

// LoadFromConfigMap creates a config out of kubernetes config map
func (oc OpConfig) LoadFromConfigMap(cm *corev1.ConfigMap) {
	for k, v := range DefaultOpConfig() {
		oc[k] = v
	}

	for k, v := range cm.Data {
		oc[k] = v
	}
}

// DefaultOpConfig returns default configuration
func DefaultOpConfig() OpConfig {
	cfg := OpConfig{}
	cfg[OpConfigDefaultHostname] = ""
	cfg[OpConfigCMCADuration] = "8766h"
	cfg[OpConfigCMCertDuration] = "2160h"
	cfg[OpConfigReconcileInterval] = "15"
	cfg[OpConfigReconcileIntervalIncrease] = "1"
	return cfg
}
