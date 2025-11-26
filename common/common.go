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
				Port:   intstr.FromInt(ba.GetManagedPort()),
				Scheme: ba.GetManagedScheme(),
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
				Port:   intstr.FromInt(ba.GetManagedPort()),
				Scheme: ba.GetManagedScheme(),
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
				Port:   intstr.FromInt(ba.GetManagedPort()),
				Scheme: ba.GetManagedScheme(),
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

func CustomizeProbeDefaults(config *corev1.Probe, defaultProbe *corev1.Probe) *corev1.Probe {
	probe := defaultProbe
	if config.ProbeHandler.Exec != nil {
		probe.ProbeHandler.Exec = config.ProbeHandler.Exec
	}
	if config.ProbeHandler.GRPC != nil {
		probe.ProbeHandler.GRPC = config.ProbeHandler.GRPC
	}
	if config.ProbeHandler.HTTPGet != nil {
		if probe.ProbeHandler.HTTPGet == nil {
			probe.ProbeHandler.HTTPGet = &corev1.HTTPGetAction{}
		}
		if config.ProbeHandler.HTTPGet.Port.Type != 0 {
			probe.ProbeHandler.HTTPGet.Port = config.ProbeHandler.HTTPGet.Port
		}
		if config.ProbeHandler.HTTPGet.Host != "" {
			probe.ProbeHandler.HTTPGet.Host = config.ProbeHandler.HTTPGet.Host
		}
		if config.ProbeHandler.HTTPGet.Path != "" {
			probe.ProbeHandler.HTTPGet.Path = config.ProbeHandler.HTTPGet.Path
		}
		if config.ProbeHandler.HTTPGet.Scheme != "" {
			probe.ProbeHandler.HTTPGet.Scheme = config.ProbeHandler.HTTPGet.Scheme
		}
		if len(config.ProbeHandler.HTTPGet.HTTPHeaders) > 0 {
			probe.ProbeHandler.HTTPGet.HTTPHeaders = config.ProbeHandler.HTTPGet.HTTPHeaders
		}
	}
	if config.ProbeHandler.TCPSocket != nil {
		probe.ProbeHandler.TCPSocket = config.ProbeHandler.TCPSocket
	}
	if config.InitialDelaySeconds != 0 {
		probe.InitialDelaySeconds = config.InitialDelaySeconds
	}
	if config.TimeoutSeconds != 0 {
		probe.TimeoutSeconds = config.TimeoutSeconds
	}
	if config.PeriodSeconds != 0 {
		probe.PeriodSeconds = config.PeriodSeconds
	}
	if config.SuccessThreshold != 0 {
		probe.SuccessThreshold = config.SuccessThreshold
	}
	if config.FailureThreshold != 0 {
		probe.FailureThreshold = config.FailureThreshold
	}
	if config.TerminationGracePeriodSeconds != nil {
		probe.TerminationGracePeriodSeconds = config.TerminationGracePeriodSeconds
	}
	return probe
}
