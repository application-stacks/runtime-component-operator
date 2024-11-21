package common

import (
	uberzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

	// OpConfigLogLevel the level of logs to be written
	OpConfigLogLevel = "operatorLogLevel"

	// The allowed values for OpConfigLogLevel
	logLevelWarning  = "warning"
	logLevelInfo     = "info"
	logLevelDebug    = "fine"
	logLevelDebug2   = "finer"
	logLevelDebugMax = "finest"

	// Constants to use when fetching a debug level logger
	LogLevelDebug  = 1
	LogLevelDebug2 = 2

	// zap logging level constants
	zLevelWarn   zapcore.Level = 1
	zLevelInfo   zapcore.Level = 0
	zLevelDebug  zapcore.Level = -1
	zLevelDebug2 zapcore.Level = -2
	// zapcore.Level is defined as int8, so this logs everything
	zLevelDebugMax zapcore.Level = -127

	// OpConfigReconcileIntervalSeconds default reconciliation interval in seconds
	OpConfigReconcileIntervalSeconds = "reconcileIntervalSeconds"

	// OpConfigReconcileIntervalPercentage default reconciliation interval increase, represented as a percentage (100 equaling to 100%)
	// When the reconciliation interval needs to increase, it will increase by the given percentage
	OpConfigReconcileIntervalPercentage = "reconcileIntervalIncreasePercentage"
)

// Config stores operator configuration
var Config = OpConfig{}

var LevelFunc = uberzap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
	return lvl >= Config.GetZapLogLevel()
})

var StackLevelFunc = uberzap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
	configuredLevel := Config.GetZapLogLevel()
	if configuredLevel > zapcore.DebugLevel {
		// No stack traces unless fine/finer/finest has been requested
		// Zap's debug is mapped to fine
		return false
	}
	// Stack traces for error or worse (fatal/panic)
	if lvl >= zapcore.ErrorLevel {
		return true
	}
	// Logging is set to fine/finer/finest but msg is info or less. No stack trace
	return false
})

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

// Returns the zap log level corresponding to the value of the
// 'logLevel' key in the config map. Returns 'info' if they key
// is missing or contains an invalid value.
func (oc OpConfig) GetZapLogLevel() zapcore.Level {
	level, ok := oc[OpConfigLogLevel]
	if !ok {
		return zLevelInfo
	}
	switch level {
	case logLevelWarning:
		return zLevelWarn
	case logLevelInfo:
		return zLevelInfo
	case logLevelDebug:
		return zLevelDebug
	case logLevelDebug2:
		return zLevelDebug2
	case logLevelDebugMax:
		return zLevelDebugMax
	default:
		// config value is invalid.
		return zLevelInfo
	}
}

// DefaultOpConfig returns default configuration
func DefaultOpConfig() OpConfig {
	cfg := OpConfig{}
	cfg[OpConfigDefaultHostname] = ""
	cfg[OpConfigCMCADuration] = "8766h"
	cfg[OpConfigCMCertDuration] = "2160h"
	cfg[OpConfigLogLevel] = logLevelInfo
	cfg[OpConfigReconcileIntervalSeconds] = "15"
	cfg[OpConfigReconcileIntervalPercentage] = "100"
	return cfg
}
