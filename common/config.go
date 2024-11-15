package common

import (
	"errors"
	"strconv"

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

	// OpConfigReconcileIntervalSeconds default reconciliation interval in seconds
	OpConfigReconcileIntervalSeconds = "reconcileIntervalSeconds"

	// OpConfigReconcileIntervalPercentage default reconciliation interval increase, represented as a percentage (100 equaling to 100%)
	// When the reconciliation interval needs to increase, it will increase by the given percentage
	OpConfigReconcileIntervalPercentage = "reconcileIntervalIncreasePercentage"
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
func (oc OpConfig) CheckValidValue(key string, OperatorName string) error {
	value := oc[key]

	intValue, err := strconv.Atoi(value)
	if err != nil {
		oc.SetConfigMapDefaultValue(key)
		return errors.New(key + " in ConfigMap: " + OperatorName + " has an invalid syntax, error: " + err.Error())
	} else if key == OpConfigReconcileIntervalSeconds && intValue <= 0 {
		oc.SetConfigMapDefaultValue(key)
		return errors.New(key + " in ConfigMap: " + OperatorName + " is set to " + value + ". It must be greater than 0.")
	} else if key == OpConfigReconcileIntervalPercentage && intValue < 0 {
		oc.SetConfigMapDefaultValue(key)
		return errors.New(key + " in ConfigMap: " + OperatorName + " is set to " + value + ". It must be greater than or equal to 0.")
	}

	return nil
}

// SetConfigMapDefaultValue sets default value for specified key
func (oc OpConfig) SetConfigMapDefaultValue(key string) {
	cm := DefaultOpConfig()
	oc[key] = cm[key]
}

// DefaultOpConfig returns default configuration
func DefaultOpConfig() OpConfig {
	cfg := OpConfig{}
	cfg[OpConfigDefaultHostname] = ""
	cfg[OpConfigCMCADuration] = "8766h"
	cfg[OpConfigCMCertDuration] = "2160h"
	cfg[OpConfigReconcileIntervalSeconds] = "15"
	cfg[OpConfigReconcileIntervalPercentage] = "100"
	return cfg
}
