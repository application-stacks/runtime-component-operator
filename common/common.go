package common

import (
	"slices"

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

// GetComponentAnnotationsGetter returns the getter for annotationType annotations configured in the BaseComponent instance.
func GetComponentAnnotationsGetter(ba BaseComponent, annotationType StatusTrackedAnnotationType) func() map[string]string {
	var annotationGetter func() map[string]string
	annotationGetter = nil
	switch annotationType {
	case StatusTrackedAnnotationTypeGlobal:
		annotationGetter = ba.GetAnnotations
	case StatusTrackedAnnotationTypeDeployment:
		if ba.GetDeployment() != nil {
			annotationGetter = ba.GetDeployment().GetAnnotations
		}
	case StatusTrackedAnnotationTypeStatefulSet:
		if ba.GetStatefulSet() != nil {
			annotationGetter = ba.GetStatefulSet().GetAnnotations
		}
	case StatusTrackedAnnotationTypeService:
		if ba.GetService() != nil {
			annotationGetter = ba.GetService().GetAnnotations
		}
	case StatusTrackedAnnotationTypeRoute:
		if ba.GetRoute() != nil {
			annotationGetter = ba.GetRoute().GetAnnotations
		}
	}
	return annotationGetter
}

// SaveTrackedAnnotations stores current annotations configured in the BaseComponent into the BaseComponentStatus.
func SaveTrackedAnnotations(ba BaseComponent, annotationTypes ...StatusTrackedAnnotationType) {
	for _, annotationType := range annotationTypes {
		if annotationGetter := GetComponentAnnotationsGetter(ba, annotationType); annotationGetter != nil {
			currentAnnotationKeys := []string{}
			for key := range annotationGetter() {
				currentAnnotationKeys = append(currentAnnotationKeys, key)
			}
			if len(currentAnnotationKeys) > 0 {
				slices.Sort(currentAnnotationKeys)
				ba.GetStatus().SetTrackedAnnotation(annotationType, currentAnnotationKeys)
			} else {
				ba.GetStatus().SetTrackedAnnotation(annotationType, nil)
			}
		}
	}
}

// DeleteMissingTrackedAnnotations returns the map of currentAnnotations after removing annotationType annotations
// that are no longer configured in BaseComponent instance but still found within the tracked annotations in BaseComponentStatus.
func DeleteMissingTrackedAnnotations(currentAnnotations map[string]string, ba BaseComponent, annotationTypes ...StatusTrackedAnnotationType) map[string]string {
	missingTrackedAnnotations := FilterMissingTrackedAnnotations(ba, StatusTrackedAnnotationTypeGlobal, ba.GetAnnotations())
	for _, missingTrackedAnnotation := range missingTrackedAnnotations {
		delete(currentAnnotations, missingTrackedAnnotation)
	}
	for _, annotationType := range annotationTypes {
		if annotationType != StatusTrackedAnnotationTypeGlobal {
			if annotationGetter := GetComponentAnnotationsGetter(ba, annotationType); annotationGetter != nil {
				missingTrackedAnnotations := FilterMissingTrackedAnnotations(ba, annotationType, annotationGetter())
				for _, missingTrackedAnnotation := range missingTrackedAnnotations {
					delete(currentAnnotations, missingTrackedAnnotation)
				}
			}
		}
	}
	return currentAnnotations
}

// FilterMissingTrackedAnnotations returns an array of tracked annotations that are no longer configured in the BaseComponent instance
// but still exist in the tracked annotations of BaseComponentStatus.
func FilterMissingTrackedAnnotations(ba BaseComponent, annotationType StatusTrackedAnnotationType, annotations map[string]string) []string {
	if ba.GetStatus() == nil {
		return []string{}
	}
	trackedAnnotations := ba.GetStatus().GetTrackedAnnotation(annotationType)
	if len(trackedAnnotations) == 0 || annotations == nil {
		return []string{}
	}
	for annotationKey := range annotations {
		if slices.Contains(trackedAnnotations, annotationKey) {
			// remove annotationKey from trackedAnnotations
			deleteIndex := -1
			for i := range trackedAnnotations {
				if trackedAnnotations[i] == annotationKey {
					deleteIndex = i
					break
				}
			}
			if deleteIndex != -1 {
				trackedAnnotations = append(trackedAnnotations[:deleteIndex], trackedAnnotations[deleteIndex+1:]...)
			}
		}
	}
	return trackedAnnotations
}
