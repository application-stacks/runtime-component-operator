package common

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GetDefaultMicroProfileStartupProbe returns the default values for MicroProfile Health-based startup probe.
func GetDefaultMicroProfileStartupProbe(ba BaseComponent) *BaseComponentProbe {
	port := intstr.FromInt(int(ba.GetService().GetPort()))
	return &BaseComponentProbe{
		BaseComponentProbeHandler: BaseComponentProbeHandler{
			HTTPGet: &OptionalHTTPGetAction{
				Path:   "/health/started",
				Port:   &port,
				Scheme: "HTTPS",
			},
		},
		PeriodSeconds:    10,
		TimeoutSeconds:   2,
		FailureThreshold: 20,
	}
}

// GetDefaultMicroProfileReadinessProbe returns the default values for MicroProfile Health-based readiness probe.
func GetDefaultMicroProfileReadinessProbe(ba BaseComponent) *BaseComponentProbe {
	port := intstr.FromInt(int(ba.GetService().GetPort()))
	return &BaseComponentProbe{
		BaseComponentProbeHandler: BaseComponentProbeHandler{
			HTTPGet: &OptionalHTTPGetAction{
				Path:   "/health/ready",
				Port:   &port,
				Scheme: "HTTPS",
			},
		},
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		TimeoutSeconds:      2,
		FailureThreshold:    10,
	}
}

// GetDefaultMicroProfileLivenessProbe returns the default values for MicroProfile Health-based liveness probe.
func GetDefaultMicroProfileLivenessProbe(ba BaseComponent) *BaseComponentProbe {
	port := intstr.FromInt(int(ba.GetService().GetPort()))
	return &BaseComponentProbe{
		BaseComponentProbeHandler: BaseComponentProbeHandler{
			HTTPGet: &OptionalHTTPGetAction{
				Path:   "/health/live",
				Port:   &port,
				Scheme: "HTTPS",
			},
		},
		InitialDelaySeconds: 60,
		PeriodSeconds:       10,
		TimeoutSeconds:      2,
		FailureThreshold:    3,
	}
}

// GetComponentNameLabel returns the component's name label.
func GetComponentNameLabel(ba BaseComponent) string {
	return ba.GetGroupName() + "/name"
}
