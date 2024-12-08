package common

import (
	"errors"
	"strconv"
	"sync"

	uberzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	corev1 "k8s.io/api/core/v1"
)

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
var Config *sync.Map

func init() {
	Config = &sync.Map{}
}

var LevelFunc = uberzap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
	return lvl >= GetZapLogLevel(Config)
})

var StackLevelFunc = uberzap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
	configuredLevel := GetZapLogLevel(Config)
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

// Loads a string value stored at key in the sync.Map oc or "" if it does not exist
func LoadFromConfig(oc *sync.Map, key string) string {
	value, ok := oc.Load(key)
	if !ok {
		return ""
	}
	return value.(string)
}

func CheckValidValue(oc *sync.Map, key string, OperatorName string) error {
	value := LoadFromConfig(oc, key)

	intValue, err := strconv.Atoi(value)
	if err != nil {
		SetConfigMapDefaultValue(oc, key)
		return errors.New(key + " in ConfigMap: " + OperatorName + " has an invalid syntax, error: " + err.Error())
	} else if key == OpConfigReconcileIntervalSeconds && intValue <= 0 {
		SetConfigMapDefaultValue(oc, key)
		return errors.New(key + " in ConfigMap: " + OperatorName + " is set to " + value + ". It must be greater than 0.")
	} else if key == OpConfigReconcileIntervalPercentage && intValue < 0 {
		SetConfigMapDefaultValue(oc, key)
		return errors.New(key + " in ConfigMap: " + OperatorName + " is set to " + value + ". It must be greater than or equal to 0.")
	}

	return nil
}

// SetConfigMapDefaultValue sets default value for specified key
func SetConfigMapDefaultValue(oc *sync.Map, key string) {
	cm := DefaultOpConfig()
	defaultValue, ok := cm.Load(key)
	if ok {
		oc.Store(key, defaultValue)
	}
}

// Returns the zap log level corresponding to the value of the
// 'logLevel' key in the config map. Returns 'info' if they key
// is missing or contains an invalid value.
func GetZapLogLevel(oc *sync.Map) zapcore.Level {
	level := LoadFromConfig(oc, OpConfigLogLevel)
	if level == "" {
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
func DefaultOpConfig() *sync.Map {
	cfg := &sync.Map{}
	cfg.Store(OpConfigDefaultHostname, "")
	cfg.Store(OpConfigCMCADuration, "8766h")
	cfg.Store(OpConfigCMCertDuration, "2160h")
	cfg.Store(OpConfigLogLevel, logLevelInfo)
	cfg.Store(OpConfigReconcileIntervalSeconds, "5")
	cfg.Store(OpConfigReconcileIntervalPercentage, "100")
	return cfg
}
