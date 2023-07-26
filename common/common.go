package common

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GetDefaultMicroProfileStartupProbe returns the default values for MicroProfile Health-based startup probe.
func GetDefaultMicroProfileStartupProbe(ba BaseComponent) *BaseComponentProbe {
	port := intstr.FromInt(int(ba.GetService().GetPort()))
	periodSeconds := int32(10)
	timeoutSeconds := int32(2)
	failureThreshold := int32(20)
	return &BaseComponentProbe{
		BaseComponentProbeHandler: BaseComponentProbeHandler{
			HTTPGet: &OptionalHTTPGetAction{
				Path:   "/health/started",
				Port:   &port,
				Scheme: "HTTPS",
			},
		},
		PeriodSeconds:    &periodSeconds,
		TimeoutSeconds:   &timeoutSeconds,
		FailureThreshold: &failureThreshold,
	}
}

// GetDefaultMicroProfileReadinessProbe returns the default values for MicroProfile Health-based readiness probe.
func GetDefaultMicroProfileReadinessProbe(ba BaseComponent) *BaseComponentProbe {
	port := intstr.FromInt(int(ba.GetService().GetPort()))
	initialDelaySeconds := int32(10)
	periodSeconds := int32(10)
	timeoutSeconds := int32(2)
	failureThreshold := int32(20)
	return &BaseComponentProbe{
		BaseComponentProbeHandler: BaseComponentProbeHandler{
			HTTPGet: &OptionalHTTPGetAction{
				Path:   "/health/ready",
				Port:   &port,
				Scheme: "HTTPS",
			},
		},
		InitialDelaySeconds: &initialDelaySeconds,
		PeriodSeconds:       &periodSeconds,
		TimeoutSeconds:      &timeoutSeconds,
		FailureThreshold:    &failureThreshold,
	}
}

// GetDefaultMicroProfileLivenessProbe returns the default values for MicroProfile Health-based liveness probe.
func GetDefaultMicroProfileLivenessProbe(ba BaseComponent) *BaseComponentProbe {
	port := intstr.FromInt(int(ba.GetService().GetPort()))
	initialDelaySeconds := int32(60)
	periodSeconds := int32(10)
	timeoutSeconds := int32(2)
	failureThreshold := int32(3)
	return &BaseComponentProbe{
		BaseComponentProbeHandler: BaseComponentProbeHandler{
			HTTPGet: &OptionalHTTPGetAction{
				Path:   "/health/live",
				Port:   &port,
				Scheme: "HTTPS",
			},
		},
		InitialDelaySeconds: &initialDelaySeconds,
		PeriodSeconds:       &periodSeconds,
		TimeoutSeconds:      &timeoutSeconds,
		FailureThreshold:    &failureThreshold,
	}
}

// GetComponentNameLabel returns the component's name label.
func GetComponentNameLabel(ba BaseComponent) string {
	return ba.GetGroupName() + "/name"
}
