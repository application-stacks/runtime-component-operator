package common

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func ConvertBaseComponentProbeToCoreProbe(bcp *BaseComponentProbe, defaultProbe *corev1.Probe) *corev1.Probe {
	if bcp == nil {
		return nil
	}
	return CustomizeBaseComponentProbeDefaults(bcp, defaultProbe)
}

func CustomizeBaseComponentProbeDefaults(config *BaseComponentProbe, defaultProbe *corev1.Probe) *corev1.Probe {
	probe := defaultProbe
	if probe == nil {
		probe = &corev1.Probe{}
	}
	if config == nil {
		return probe
	}
	if config.BaseComponentProbeHandler.Exec != nil {
		probe.ProbeHandler.Exec = config.BaseComponentProbeHandler.Exec
	}
	if config.BaseComponentProbeHandler.GRPC != nil {
		probe.ProbeHandler.GRPC = config.BaseComponentProbeHandler.GRPC
	}
	if config.BaseComponentProbeHandler.TCPSocket != nil {
		probe.ProbeHandler.TCPSocket = config.BaseComponentProbeHandler.TCPSocket
	}
	if config.BaseComponentProbeHandler.HTTPGet != nil {
		probe.ProbeHandler.HTTPGet = convertOptionalHTTPGetActionToHTTPGetAction(config.BaseComponentProbeHandler.HTTPGet, probe.ProbeHandler.HTTPGet)
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

func convertOptionalHTTPGetActionToHTTPGetAction(optAction *OptionalHTTPGetAction, defaultHTTPGetAction *corev1.HTTPGetAction) *corev1.HTTPGetAction {
	action := defaultHTTPGetAction
	if action == nil {
		action = &corev1.HTTPGetAction{}
	}
	if optAction == nil {
		return action
	}
	if optAction.Host != nil {
		action.Host = *optAction.Host
	}
	if optAction.Path != nil {
		action.Path = *optAction.Path
	}
	if optAction.Port != nil {
		action.Port = *optAction.Port
	}
	if optAction.Scheme != nil {
		action.Scheme = *optAction.Scheme
	}
	if optAction.HTTPHeaders != nil {
		action.HTTPHeaders = *optAction.HTTPHeaders
	}
	return action
}

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
