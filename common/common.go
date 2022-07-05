package common

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GetDefaultMicroProfileStartupProbe returns the default values for MicroProfile Health-based startup probe.
func GetDefaultMicroProfileStartupProbe(ba BaseComponent) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/started",
				Port:   intstr.FromInt(int(ba.GetService().GetPort())),
				Scheme: "HTTPS",
			},
		},
		PeriodSeconds:    10,
		TimeoutSeconds:   2,
		FailureThreshold: 20,
	}
}

// GetDefaultMicroProfileReadinessProbe returns the default values for MicroProfile Health-based readiness probe.
func GetDefaultMicroProfileReadinessProbe(ba BaseComponent) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/ready",
				Port:   intstr.FromInt(int(ba.GetService().GetPort())),
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
func GetDefaultMicroProfileLivenessProbe(ba BaseComponent) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health/live",
				Port:   intstr.FromInt(int(ba.GetService().GetPort())),
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
