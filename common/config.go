package common

import (
	"go.uber.org/zap/zapcore"
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
	return cfg
}
