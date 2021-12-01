package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/application-stacks/runtime-component-operator/common"
	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"

	appstacksv1beta2 "github.com/application-stacks/runtime-component-operator/api/v1beta2"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
)

// CustomizeDeployment ...
func CustomizeDeployment(deploy *appsv1.Deployment, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	deploy.Labels = ba.GetLabels()
	deploy.Annotations = MergeMaps(deploy.Annotations, ba.GetAnnotations())

	if ba.GetAutoscaling() == nil {
		deploy.Spec.Replicas = ba.GetReplicas()
	}

	if deploy.Spec.Selector == nil {
		deploy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/instance": obj.GetName(),
			},
		}
	}

	dp := ba.GetDeployment()
	if dp != nil && dp.GetDeploymentUpdateStrategy() != nil {
		deploy.Spec.Strategy = *dp.GetDeploymentUpdateStrategy()
	} else {
		deploy.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}
	}
	if dp != nil && dp.GetAnnotations() != nil {
		deploy.Annotations = MergeMaps(deploy.Annotations, dp.GetAnnotations())
	}

}

// CustomizeStatefulSet ...
func CustomizeStatefulSet(statefulSet *appsv1.StatefulSet, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	statefulSet.Labels = ba.GetLabels()
	statefulSet.Annotations = MergeMaps(statefulSet.Annotations, ba.GetAnnotations())

	if ba.GetAutoscaling() == nil {
		statefulSet.Spec.Replicas = ba.GetReplicas()
	}
	statefulSet.Spec.ServiceName = obj.GetName() + "-headless"
	if statefulSet.Spec.Selector == nil {
		statefulSet.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/instance": obj.GetName(),
			},
		}
	}

	ss := ba.GetStatefulSet()
	if ss != nil {
		if ss.GetStatefulSetUpdateStrategy() != nil {
			statefulSet.Spec.UpdateStrategy = *ss.GetStatefulSetUpdateStrategy()
		} else {
			statefulSet.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType}
		}
	}
	if ss != nil && ss.GetAnnotations() != nil {
		statefulSet.Annotations = MergeMaps(statefulSet.Annotations, ss.GetAnnotations())
	}
}

// CustomizeRoute ...
func CustomizeRoute(route *routev1.Route, ba common.BaseComponent, key string, crt string, ca string, destCACert string) {
	obj := ba.(metav1.Object)
	route.Labels = ba.GetLabels()
	route.Annotations = MergeMaps(route.Annotations, ba.GetAnnotations())

	if ba.GetRoute() != nil {
		rt := ba.GetRoute()
		route.Annotations = MergeMaps(route.Annotations, rt.GetAnnotations())

		host := rt.GetHost()
		if host == "" && common.Config[common.OpConfigDefaultHostname] != "" {
			host = obj.GetName() + "-" + obj.GetNamespace() + "." + common.Config[common.OpConfigDefaultHostname]
		}
		route.Spec.Host = host
		route.Spec.Path = rt.GetPath()
		if ba.GetRoute().GetTermination() != nil {
			if route.Spec.TLS == nil {
				route.Spec.TLS = &routev1.TLSConfig{}
			}
			route.Spec.TLS.Termination = *rt.GetTermination()
			if route.Spec.TLS.Termination == routev1.TLSTerminationReencrypt {
				route.Spec.TLS.Certificate = crt
				route.Spec.TLS.CACertificate = ca
				route.Spec.TLS.Key = key
				route.Spec.TLS.DestinationCACertificate = destCACert
				if rt.GetInsecureEdgeTerminationPolicy() != nil {
					route.Spec.TLS.InsecureEdgeTerminationPolicy = *rt.GetInsecureEdgeTerminationPolicy()
				}
			} else if route.Spec.TLS.Termination == routev1.TLSTerminationPassthrough {
				route.Spec.TLS.Certificate = ""
				route.Spec.TLS.CACertificate = ""
				route.Spec.TLS.Key = ""
				route.Spec.TLS.DestinationCACertificate = ""
				route.Spec.TLS.InsecureEdgeTerminationPolicy = ""
			} else if route.Spec.TLS.Termination == routev1.TLSTerminationEdge {
				route.Spec.TLS.Certificate = crt
				route.Spec.TLS.CACertificate = ca
				route.Spec.TLS.Key = key
				route.Spec.TLS.DestinationCACertificate = ""
				if rt.GetInsecureEdgeTerminationPolicy() != nil {
					route.Spec.TLS.InsecureEdgeTerminationPolicy = *rt.GetInsecureEdgeTerminationPolicy()
				}
			}
		}
	}
	if ba.GetRoute() == nil || ba.GetRoute().GetTermination() == nil {
		route.Spec.TLS = nil
	}
	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = obj.GetName()
	weight := int32(100)
	route.Spec.To.Weight = &weight
	if route.Spec.Port == nil {
		route.Spec.Port = &routev1.RoutePort{}
	}

	if ba.GetService().GetPortName() != "" {
		route.Spec.Port.TargetPort = intstr.FromString(ba.GetService().GetPortName())
	} else {
		route.Spec.Port.TargetPort = intstr.FromString(strconv.Itoa(int(ba.GetService().GetPort())) + "-tcp")
	}
}

// ErrorIsNoMatchesForKind ...
func ErrorIsNoMatchesForKind(err error, kind string, version string) bool {
	return strings.HasPrefix(err.Error(), fmt.Sprintf("no matches for kind \"%s\" in version \"%s\"", kind, version))
}

// CustomizeService ...
func CustomizeService(svc *corev1.Service, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	svc.Labels = ba.GetLabels()
	svc.Annotations = MergeMaps(svc.Annotations, ba.GetAnnotations())

	if len(svc.Spec.Ports) == 0 {
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{})
	}

	svc.Spec.Ports[0].Port = ba.GetService().GetPort()
	svc.Spec.Ports[0].TargetPort = intstr.FromInt(int(ba.GetService().GetPort()))

	if ba.GetService().GetPortName() != "" {
		svc.Spec.Ports[0].Name = ba.GetService().GetPortName()
	} else {
		svc.Spec.Ports[0].Name = strconv.Itoa(int(ba.GetService().GetPort())) + "-tcp"
	}

	if *ba.GetService().GetType() == corev1.ServiceTypeNodePort && ba.GetService().GetNodePort() != nil {
		svc.Spec.Ports[0].NodePort = *ba.GetService().GetNodePort()
	}

	if *ba.GetService().GetType() == corev1.ServiceTypeClusterIP || strings.HasSuffix(svc.Name, "-headless") == true {
		svc.Spec.Ports[0].NodePort = 0
	}

	svc.Spec.Type = *ba.GetService().GetType()
	svc.Spec.Selector = map[string]string{
		"app.kubernetes.io/instance": obj.GetName(),
	}

	if ba.GetService().GetTargetPort() != nil {
		svc.Spec.Ports[0].TargetPort = intstr.FromInt(int(*ba.GetService().GetTargetPort()))
	}

	numOfAdditionalPorts := len(ba.GetService().GetPorts())
	numOfCurrentPorts := len(svc.Spec.Ports) - 1
	for i := 0; i < numOfAdditionalPorts; i++ {
		for numOfCurrentPorts < numOfAdditionalPorts {
			svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{})
			numOfCurrentPorts++
		}
		for numOfCurrentPorts > numOfAdditionalPorts && len(svc.Spec.Ports) != 0 {
			svc.Spec.Ports = svc.Spec.Ports[:len(svc.Spec.Ports)-1]
			numOfCurrentPorts--
		}
		svc.Spec.Ports[i+1].Port = ba.GetService().GetPorts()[i].Port
		svc.Spec.Ports[i+1].TargetPort = intstr.FromInt(int(ba.GetService().GetPorts()[i].Port))

		if ba.GetService().GetPorts()[i].Name != "" {
			svc.Spec.Ports[i+1].Name = ba.GetService().GetPorts()[i].Name
		} else {
			svc.Spec.Ports[i+1].Name = strconv.Itoa(int(ba.GetService().GetPorts()[i].Port)) + "-tcp"
		}

		if ba.GetService().GetPorts()[i].TargetPort.String() != "" {
			svc.Spec.Ports[i+1].TargetPort = intstr.FromInt(ba.GetService().GetPorts()[i].TargetPort.IntValue())
		}

		if *ba.GetService().GetType() == corev1.ServiceTypeNodePort && ba.GetService().GetPorts()[i].NodePort != 0 {
			svc.Spec.Ports[i+1].NodePort = ba.GetService().GetPorts()[i].NodePort
		}

		if *ba.GetService().GetType() == corev1.ServiceTypeClusterIP {
			svc.Spec.Ports[i+1].NodePort = 0
		}
	}
	if len(ba.GetService().GetPorts()) == 0 {
		for numOfCurrentPorts > 0 {
			svc.Spec.Ports = svc.Spec.Ports[:len(svc.Spec.Ports)-1]
			numOfCurrentPorts--
		}
	}
}

// CustomizeAffinity ...
func CustomizeAffinity(affinity *corev1.Affinity, ba common.BaseComponent) {

	var archs []string

	if ba.GetAffinity() != nil {
		affinity.NodeAffinity = ba.GetAffinity().GetNodeAffinity()
		affinity.PodAffinity = ba.GetAffinity().GetPodAffinity()
		affinity.PodAntiAffinity = ba.GetAffinity().GetPodAntiAffinity()

		archs = ba.GetAffinity().GetArchitecture()

		if len(ba.GetAffinity().GetNodeAffinityLabels()) > 0 {
			if affinity.NodeAffinity == nil {
				affinity.NodeAffinity = &corev1.NodeAffinity{}
			}
			if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			}
			nodeSelector := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution

			if len(nodeSelector.NodeSelectorTerms) == 0 {
				nodeSelector.NodeSelectorTerms = append(nodeSelector.NodeSelectorTerms, corev1.NodeSelectorTerm{})
			}
			labels := ba.GetAffinity().GetNodeAffinityLabels()

			keys := make([]string, 0, len(labels))
			for k := range labels {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for i := range nodeSelector.NodeSelectorTerms {

				for _, key := range keys {
					values := strings.Split(labels[key], ",")
					for i := range values {
						values[i] = strings.TrimSpace(values[i])
					}

					requirement := corev1.NodeSelectorRequirement{
						Key:      key,
						Operator: corev1.NodeSelectorOpIn,
						Values:   values,
					}

					nodeSelector.NodeSelectorTerms[i].MatchExpressions = append(nodeSelector.NodeSelectorTerms[i].MatchExpressions, requirement)
				}

			}
		}
	}

	if len(archs) > 0 {
		if affinity.NodeAffinity == nil {
			affinity.NodeAffinity = &corev1.NodeAffinity{}
		}
		if affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
		}

		nodeSelector := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution

		if len(nodeSelector.NodeSelectorTerms) == 0 {
			nodeSelector.NodeSelectorTerms = append(nodeSelector.NodeSelectorTerms, corev1.NodeSelectorTerm{})
		}

		for i := range nodeSelector.NodeSelectorTerms {
			nodeSelector.NodeSelectorTerms[i].MatchExpressions = append(nodeSelector.NodeSelectorTerms[i].MatchExpressions,
				corev1.NodeSelectorRequirement{
					Key:      "kubernetes.io/arch",
					Operator: corev1.NodeSelectorOpIn,
					Values:   archs,
				},
			)
		}

		for i := range archs {
			term := corev1.PreferredSchedulingTerm{
				Weight: int32(len(archs)) - int32(i),
				Preference: corev1.NodeSelectorTerm{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "kubernetes.io/arch",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{archs[i]},
						},
					},
				},
			}
			affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, term)
		}
	}

}

// CustomizePodSpec ...
func CustomizePodSpec(pts *corev1.PodTemplateSpec, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	pts.Labels = ba.GetLabels()
	pts.Annotations = MergeMaps(pts.Annotations, ba.GetAnnotations())

	// If they exist, add annotations from the StatefulSet or Deployment to the pods
	// Both structs can exist, but if StatefulSet =! nil, then that is 'active' and the
	// deployment should be ignored
	dp := ba.GetDeployment()
	rcss := ba.GetStatefulSet()
	if rcss != nil {
		if rcss.GetAnnotations() != nil {
			pts.Annotations = MergeMaps(pts.Annotations, rcss.GetAnnotations())
		}
	} else {
		if dp != nil && dp.GetAnnotations() != nil {
			pts.Annotations = MergeMaps(pts.Annotations, dp.GetAnnotations())
		}
	}

	var appContainer corev1.Container
	if len(pts.Spec.Containers) == 0 {
		appContainer = corev1.Container{}
	} else {
		appContainer = *GetAppContainer(pts.Spec.Containers)
	}

	appContainer.Name = "app"
	if len(appContainer.Ports) == 0 {
		appContainer.Ports = append(appContainer.Ports, corev1.ContainerPort{})
	}

	if ba.GetService().GetTargetPort() != nil {
		appContainer.Ports[0].ContainerPort = *ba.GetService().GetTargetPort()
	} else {
		appContainer.Ports[0].ContainerPort = ba.GetService().GetPort()
	}

	appContainer.Image = ba.GetStatus().GetImageReference()
	if ba.GetService().GetPortName() != "" {
		appContainer.Ports[0].Name = ba.GetService().GetPortName()
	} else {
		appContainer.Ports[0].Name = strconv.Itoa(int(appContainer.Ports[0].ContainerPort)) + "-tcp"
	}
	if ba.GetResourceConstraints() != nil {
		appContainer.Resources = *ba.GetResourceConstraints()
	}

	if ba.GetProbes() != nil {
		appContainer.ReadinessProbe = ba.GetProbes().GetReadinessProbe()
		appContainer.LivenessProbe = ba.GetProbes().GetLivenessProbe()
		appContainer.StartupProbe = ba.GetProbes().GetStartupProbe()
	} else {
		appContainer.ReadinessProbe = nil
		appContainer.LivenessProbe = nil
		appContainer.StartupProbe = nil
	}

	if ba.GetPullPolicy() != nil {
		appContainer.ImagePullPolicy = *ba.GetPullPolicy()
	}
	appContainer.Env = ba.GetEnv()
	appContainer.EnvFrom = ba.GetEnvFrom()

	pts.Spec.InitContainers = ba.GetInitContainers()

	appContainer.VolumeMounts = ba.GetVolumeMounts()
	pts.Spec.Volumes = ba.GetVolumes()

	if ba.GetService().GetCertificateSecretRef() != nil {
		secretName := obj.GetName() + "-svc-tls"
		if ba.GetService().GetCertificateSecretRef() != nil {
			secretName = *ba.GetService().GetCertificateSecretRef()
		}
		appContainer.Env = append(appContainer.Env, corev1.EnvVar{Name: "TLS_DIR", Value: "/etc/x509/certs"})
		pts.Spec.Volumes = append(pts.Spec.Volumes, corev1.Volume{
			Name: "svc-certificate",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		})
		appContainer.VolumeMounts = append(appContainer.VolumeMounts, corev1.VolumeMount{
			Name:      "svc-certificate",
			MountPath: "/etc/x509/certs",
			ReadOnly:  true,
		})
	}

	pts.Spec.Containers = append([]corev1.Container{appContainer}, ba.GetSidecarContainers()...)

	if ba.GetServiceAccountName() != nil && *ba.GetServiceAccountName() != "" {
		pts.Spec.ServiceAccountName = *ba.GetServiceAccountName()
	} else {
		pts.Spec.ServiceAccountName = obj.GetName()
	}
	pts.Spec.RestartPolicy = corev1.RestartPolicyAlways
	pts.Spec.DNSPolicy = corev1.DNSClusterFirst

	if ba.GetAffinity() != nil {
		pts.Spec.Affinity = &corev1.Affinity{}
		CustomizeAffinity(pts.Spec.Affinity, ba)
	} else {
		pts.Spec.Affinity = nil
	}
}

// CustomizePersistence ...
func CustomizePersistence(statefulSet *appsv1.StatefulSet, ba common.BaseComponent) {
	obj, ss := ba.(metav1.Object), ba.GetStatefulSet()
	if ss.GetStorage() != nil {
		if len(statefulSet.Spec.VolumeClaimTemplates) == 0 {
			var pvc *corev1.PersistentVolumeClaim
			if ss.GetStorage().GetVolumeClaimTemplate() != nil {
				pvc = ss.GetStorage().GetVolumeClaimTemplate()
			} else {
				pvc = &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc",
						Namespace: obj.GetNamespace(),
						Labels:    ba.GetLabels(),
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(ss.GetStorage().GetSize()),
							},
						},
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
					},
				}
				pvc.Annotations = MergeMaps(pvc.Annotations, ba.GetAnnotations())
			}
			statefulSet.Spec.VolumeClaimTemplates = append(statefulSet.Spec.VolumeClaimTemplates, *pvc)
		}

		appContainer := GetAppContainer(statefulSet.Spec.Template.Spec.Containers)

		if ss.GetStorage().GetMountPath() != "" {
			found := false
			for _, v := range appContainer.VolumeMounts {
				if v.Name == statefulSet.Spec.VolumeClaimTemplates[0].Name {
					found = true
				}
			}

			if !found {
				vm := corev1.VolumeMount{
					Name:      statefulSet.Spec.VolumeClaimTemplates[0].Name,
					MountPath: ss.GetStorage().GetMountPath(),
				}
				appContainer.VolumeMounts = append(appContainer.VolumeMounts, vm)
			}
		}
	}
}

// CustomizeServiceAccount ...
func CustomizeServiceAccount(sa *corev1.ServiceAccount, ba common.BaseComponent) {
	sa.Labels = ba.GetLabels()
	sa.Annotations = MergeMaps(sa.Annotations, ba.GetAnnotations())

	if ba.GetPullSecret() != nil {
		if len(sa.ImagePullSecrets) == 0 {
			sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{
				Name: *ba.GetPullSecret(),
			})
		} else {
			sa.ImagePullSecrets[0].Name = *ba.GetPullSecret()
		}
	}
}

// CustomizeKnativeService ...
func CustomizeKnativeService(ksvc *servingv1.Service, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	ksvc.Labels = ba.GetLabels()
	ksvc.Annotations = MergeMaps(ksvc.Annotations, ba.GetAnnotations())

	// If `expose` is not set to `true`, make Knative route a private route by adding `serving.knative.dev/visibility: cluster-local`
	// to the Knative service. If `serving.knative.dev/visibility: XYZ` is defined in cr.Labels, `expose` always wins.
	if ba.GetExpose() != nil && *ba.GetExpose() {
		delete(ksvc.Labels, "serving.knative.dev/visibility")
	} else {
		ksvc.Labels["serving.knative.dev/visibility"] = "cluster-local"
	}

	if len(ksvc.Spec.Template.Spec.Containers) == 0 {
		ksvc.Spec.Template.Spec.Containers = append(ksvc.Spec.Template.Spec.Containers, corev1.Container{})
	}

	if len(ksvc.Spec.Template.Spec.Containers[0].Ports) == 0 {
		ksvc.Spec.Template.Spec.Containers[0].Ports = append(ksvc.Spec.Template.Spec.Containers[0].Ports, corev1.ContainerPort{})
	}
	ksvc.Spec.Template.ObjectMeta.Labels = ba.GetLabels()
	ksvc.Spec.Template.ObjectMeta.Annotations = MergeMaps(ksvc.Spec.Template.ObjectMeta.Annotations, ba.GetAnnotations())

	if ba.GetService().GetTargetPort() != nil {
		ksvc.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = *ba.GetService().GetTargetPort()
	} else {
		ksvc.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = ba.GetService().GetPort()
	}

	if ba.GetService().GetPortName() != "" {
		ksvc.Spec.Template.Spec.Containers[0].Ports[0].Name = ba.GetService().GetPortName()
	}

	ksvc.Spec.Template.Spec.Containers[0].Image = ba.GetStatus().GetImageReference()
	// Knative sets its own resource constraints
	//ksvc.Spec.Template.Spec.Containers[0].Resources = *cr.Spec.ResourceConstraints

	if ba.GetProbes() != nil {
		ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe = ba.GetProbes().GetReadinessProbe()
		ksvc.Spec.Template.Spec.Containers[0].LivenessProbe = ba.GetProbes().GetLivenessProbe()
		ksvc.Spec.Template.Spec.Containers[0].StartupProbe = ba.GetProbes().GetStartupProbe()
	} else {
		ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe = nil
		ksvc.Spec.Template.Spec.Containers[0].LivenessProbe = nil
		ksvc.Spec.Template.Spec.Containers[0].StartupProbe = nil
	}

	ksvc.Spec.Template.Spec.Containers[0].ImagePullPolicy = *ba.GetPullPolicy()
	ksvc.Spec.Template.Spec.Containers[0].Env = ba.GetEnv()
	ksvc.Spec.Template.Spec.Containers[0].EnvFrom = ba.GetEnvFrom()

	ksvc.Spec.Template.Spec.Containers[0].VolumeMounts = ba.GetVolumeMounts()
	ksvc.Spec.Template.Spec.Volumes = ba.GetVolumes()

	if ba.GetServiceAccountName() != nil && *ba.GetServiceAccountName() != "" {
		ksvc.Spec.Template.Spec.ServiceAccountName = *ba.GetServiceAccountName()
	} else {
		ksvc.Spec.Template.Spec.ServiceAccountName = obj.GetName()
	}

	if ksvc.Spec.Template.Spec.Containers[0].LivenessProbe != nil {
		if ksvc.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet != nil {
			ksvc.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Port = intstr.IntOrString{}
		}
		if ksvc.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket != nil {
			ksvc.Spec.Template.Spec.Containers[0].LivenessProbe.TCPSocket.Port = intstr.IntOrString{}
		}
	}

	if ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe != nil {
		if ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet != nil {
			ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Port = intstr.IntOrString{}
		}
		if ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe.TCPSocket != nil {
			ksvc.Spec.Template.Spec.Containers[0].ReadinessProbe.TCPSocket.Port = intstr.IntOrString{}
		}
	}

	if ksvc.Spec.Template.Spec.Containers[0].StartupProbe != nil {
		if ksvc.Spec.Template.Spec.Containers[0].StartupProbe.HTTPGet != nil {
			ksvc.Spec.Template.Spec.Containers[0].StartupProbe.HTTPGet.Port = intstr.IntOrString{}
		}
		if ksvc.Spec.Template.Spec.Containers[0].StartupProbe.TCPSocket != nil {
			ksvc.Spec.Template.Spec.Containers[0].StartupProbe.TCPSocket.Port = intstr.IntOrString{}
		}
	}
}

// CustomizeHPA ...
func CustomizeHPA(hpa *autoscalingv1.HorizontalPodAutoscaler, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	hpa.Labels = ba.GetLabels()
	hpa.Annotations = MergeMaps(hpa.Annotations, ba.GetAnnotations())

	hpa.Spec.MaxReplicas = ba.GetAutoscaling().GetMaxReplicas()
	hpa.Spec.MinReplicas = ba.GetAutoscaling().GetMinReplicas()
	hpa.Spec.TargetCPUUtilizationPercentage = ba.GetAutoscaling().GetTargetCPUUtilizationPercentage()

	hpa.Spec.ScaleTargetRef.Name = obj.GetName()
	hpa.Spec.ScaleTargetRef.APIVersion = "apps/v1"

	if ba.GetStatefulSet() != nil {
		hpa.Spec.ScaleTargetRef.Kind = "StatefulSet"
	} else {
		hpa.Spec.ScaleTargetRef.Kind = "Deployment"
	}
}

// Validate if the BaseComponent is valid
func Validate(ba common.BaseComponent) (bool, error) {
	// Storage validation
	ss := ba.GetStatefulSet()
	if ss != nil && ss.GetStorage() != nil {
		if ss.GetStorage().GetVolumeClaimTemplate() == nil {
			if ss.GetStorage().GetSize() == "" {
				return false, fmt.Errorf("validation failed: " + requiredFieldMessage("spec.statefulSet.storage.size"))
			}
			if _, err := resource.ParseQuantity(ss.GetStorage().GetSize()); err != nil {
				return false, fmt.Errorf("validation failed: cannot parse '%v': %v", ss.GetStorage().GetSize(), err)
			}
		}
	}

	return true, nil
}

func createValidationError(msg string) error {
	return fmt.Errorf("validation failed: " + msg)
}

func requiredFieldMessage(fieldPaths ...string) string {
	return "must set the field(s): " + strings.Join(fieldPaths, ", ")
}

// CustomizeServiceMonitor ...
func CustomizeServiceMonitor(sm *prometheusv1.ServiceMonitor, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	sm.Labels = ba.GetLabels()
	sm.Annotations = MergeMaps(sm.Annotations, ba.GetAnnotations())

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/instance":                obj.GetName(),
			"monitor." + ba.GetGroupName() + "/enabled": "true",
		},
	}
	if len(sm.Spec.Endpoints) == 0 {
		sm.Spec.Endpoints = append(sm.Spec.Endpoints, prometheusv1.Endpoint{})
	}
	sm.Spec.Endpoints[0].Port = ""
	sm.Spec.Endpoints[0].TargetPort = nil
	if len(ba.GetMonitoring().GetEndpoints()) > 0 {
		port := ba.GetMonitoring().GetEndpoints()[0].Port
		targetPort := ba.GetMonitoring().GetEndpoints()[0].TargetPort
		if port != "" {
			sm.Spec.Endpoints[0].Port = port
		}
		if targetPort != nil {
			sm.Spec.Endpoints[0].TargetPort = targetPort
		}
		if port != "" && targetPort != nil {
			sm.Spec.Endpoints[0].TargetPort = nil
		}
	}
	if sm.Spec.Endpoints[0].Port == "" && sm.Spec.Endpoints[0].TargetPort == nil {
		if ba.GetService().GetPortName() != "" {
			sm.Spec.Endpoints[0].Port = ba.GetService().GetPortName()
		} else {
			sm.Spec.Endpoints[0].Port = strconv.Itoa(int(ba.GetService().GetPort())) + "-tcp"
		}
	}
	if len(ba.GetMonitoring().GetLabels()) > 0 {
		for k, v := range ba.GetMonitoring().GetLabels() {
			sm.Labels[k] = v
		}
	}

	if len(ba.GetMonitoring().GetEndpoints()) > 0 {
		endpoints := ba.GetMonitoring().GetEndpoints()
		if endpoints[0].Scheme != "" {
			sm.Spec.Endpoints[0].Scheme = endpoints[0].Scheme
		}
		if endpoints[0].Interval != "" {
			sm.Spec.Endpoints[0].Interval = endpoints[0].Interval
		}
		if endpoints[0].Path != "" {
			sm.Spec.Endpoints[0].Path = endpoints[0].Path
		}

		if endpoints[0].TLSConfig != nil {
			sm.Spec.Endpoints[0].TLSConfig = endpoints[0].TLSConfig
		}

		if endpoints[0].BasicAuth != nil {
			sm.Spec.Endpoints[0].BasicAuth = endpoints[0].BasicAuth
		}

		if endpoints[0].Params != nil {
			sm.Spec.Endpoints[0].Params = endpoints[0].Params
		}
		if endpoints[0].ScrapeTimeout != "" {
			sm.Spec.Endpoints[0].ScrapeTimeout = endpoints[0].ScrapeTimeout
		}
		if endpoints[0].BearerTokenFile != "" {
			sm.Spec.Endpoints[0].BearerTokenFile = endpoints[0].BearerTokenFile
		}
		sm.Spec.Endpoints[0].BearerTokenSecret = endpoints[0].BearerTokenSecret
		sm.Spec.Endpoints[0].ProxyURL = endpoints[0].ProxyURL
		sm.Spec.Endpoints[0].RelabelConfigs = endpoints[0].RelabelConfigs
		sm.Spec.Endpoints[0].MetricRelabelConfigs = endpoints[0].MetricRelabelConfigs
		sm.Spec.Endpoints[0].HonorTimestamps = endpoints[0].HonorTimestamps
		sm.Spec.Endpoints[0].HonorLabels = endpoints[0].HonorLabels

	}

}

// GetCondition ...
func GetCondition(conditionType appstacksv1beta2.StatusConditionType, status *appstacksv1beta2.RuntimeComponentStatus) *appstacksv1beta2.StatusCondition {
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return &status.Conditions[i]
		}
	}

	return nil
}

// SetCondition ...
func SetCondition(condition appstacksv1beta2.StatusCondition, status *appstacksv1beta2.RuntimeComponentStatus) {
	for i := range status.Conditions {
		if status.Conditions[i].Type == condition.Type {
			status.Conditions[i] = condition
			return
		}
	}

	status.Conditions = append(status.Conditions, condition)
}

// GetWatchNamespaces returns a slice of namespaces the operator should watch based on WATCH_NAMESPSCE value
// WATCH_NAMESPSCE value could be empty for watching the whole cluster or a comma-separated list of namespaces
func GetWatchNamespaces() ([]string, error) {
	watchNamespace, err := GetWatchNamespace()
	if err != nil {
		return nil, err
	}

	var watchNamespaces []string
	for _, ns := range strings.Split(watchNamespace, ",") {
		watchNamespaces = append(watchNamespaces, strings.TrimSpace(ns))
	}

	return watchNamespaces, nil
}

// MergeMaps returns a map containing the union of al the key-value pairs from the input maps. The order of the maps passed into the
// func, defines the importance. e.g. if (keyA, value1) is in map1, and (keyA, value2) is in map2, mergeMaps(map1, map2) would contain (keyA, value2).
// If the input map is nil, it is treated as empty map.
func MergeMaps(maps ...map[string]string) map[string]string {
	dest := make(map[string]string)

	for i := range maps {
		for key, value := range maps[i] {
			dest[key] = value
		}
	}

	return dest
}

// BuildServiceBindingSecretName returns secret name of a consumable service
func BuildServiceBindingSecretName(name, namespace string) string {
	return fmt.Sprintf("%s-%s", namespace, name)
}

// ContainsString returns true if `s` is in the slice. Otherwise, returns false
func ContainsString(slice []string, s string) bool {
	for _, str := range slice {
		if str == s {
			return true
		}
	}
	return false
}

// AppendIfNotSubstring appends `a` to comma-separated list of strings in `s`
func AppendIfNotSubstring(a, s string) string {
	if s == "" {
		return a
	}
	subs := strings.Split(s, ",")
	if !ContainsString(subs, a) {
		subs = append(subs, a)
	}
	return strings.Join(subs, ",")
}

// EnsureOwnerRef adds the ownerref if needed. Removes ownerrefs with conflicting UIDs.
// Returns true if the input is mutated. Copied from "https://github.com/openshift/library-go/blob/release-4.5/pkg/controller/ownerref.go"
func EnsureOwnerRef(metadata metav1.Object, newOwnerRef metav1.OwnerReference) bool {
	foundButNotEqual := false
	for _, existingOwnerRef := range metadata.GetOwnerReferences() {
		if existingOwnerRef.APIVersion == newOwnerRef.APIVersion &&
			existingOwnerRef.Kind == newOwnerRef.Kind &&
			existingOwnerRef.Name == newOwnerRef.Name {

			// if we're completely the same, there's nothing to do
			if equality.Semantic.DeepEqual(existingOwnerRef, newOwnerRef) {
				return false
			}

			foundButNotEqual = true
			break
		}
	}

	// if we weren't found, then we just need to add ourselves
	if !foundButNotEqual {
		metadata.SetOwnerReferences(append(metadata.GetOwnerReferences(), newOwnerRef))
		return true
	}

	// if we need to remove an existing ownerRef, just do the easy thing and build it back from scratch
	newOwnerRefs := []metav1.OwnerReference{newOwnerRef}
	for i := range metadata.GetOwnerReferences() {
		existingOwnerRef := metadata.GetOwnerReferences()[i]
		if existingOwnerRef.APIVersion == newOwnerRef.APIVersion &&
			existingOwnerRef.Kind == newOwnerRef.Kind &&
			existingOwnerRef.Name == newOwnerRef.Name {
			continue
		}
		newOwnerRefs = append(newOwnerRefs, existingOwnerRef)
	}
	metadata.SetOwnerReferences(newOwnerRefs)
	return true
}

func normalizeEnvVariableName(name string) string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToUpper(name))
}

// GetOpenShiftAnnotations returns OpenShift specific annotations
func GetOpenShiftAnnotations(ba common.BaseComponent) map[string]string {
	// Conversion table between the pseudo Open Container Initiative <-> OpenShift annotations
	conversionMap := map[string]string{
		"image.opencontainers.org/source":   "app.openshift.io/vcs-uri",
		"image.opencontainers.org/revision": "app.openshift.io/vcs-ref",
	}

	annos := map[string]string{}
	for from, to := range conversionMap {
		if annoVal, ok := ba.GetAnnotations()[from]; ok {
			annos[to] = annoVal
		}
	}

	return annos
}

// IsClusterWide returns true if watchNamespaces is set to [""]
func IsClusterWide(watchNamespaces []string) bool {
	return len(watchNamespaces) == 1 && watchNamespaces[0] == ""
}

// GetAppContainer returns the container that is running the app
func GetAppContainer(containerList []corev1.Container) *corev1.Container {
	for i := 0; i < len(containerList); i++ {
		if containerList[i].Name == "app" {
			return &containerList[i]
		}
	}
	return &containerList[0]
}

// CustomizeIngress customizes ingress resource
func CustomizeIngress(ing *networkingv1.Ingress, ba common.BaseComponent) {
	obj := ba.(metav1.Object)
	ing.Labels = ba.GetLabels()
	servicePort := strconv.Itoa(int(ba.GetService().GetPort())) + "-tcp"
	host := ""
	path := ""
	var pathType networkingv1.PathType
	rt := ba.GetRoute()
	if rt != nil {
		host = rt.GetHost()
		path = rt.GetPath()
		pathType = rt.GetPathType()
		ing.Annotations = MergeMaps(ing.Annotations, ba.GetAnnotations(), rt.GetAnnotations())
	} else {
		ing.Annotations = MergeMaps(ing.Annotations, ba.GetAnnotations())
	}

	if ba.GetService().GetPortName() != "" {
		servicePort = ba.GetService().GetPortName()
	}

	if host == "" && common.Config[common.OpConfigDefaultHostname] != "" {
		host = obj.GetName() + "-" + obj.GetNamespace() + "." + common.Config[common.OpConfigDefaultHostname]
	}
	if host == "" {
		l := log.WithValues("Request.Namespace", obj.GetNamespace(), "Request.Name", obj.GetName())
		l.Info("No Ingress hostname is provided. Ingress might not function correctly without hostname. It is recommended to set Ingress host or to provide default value through operator's config map.")
	}

	ing.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     path,
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: obj.GetName(),
									Port: networkingv1.ServiceBackendPort{
										Name: servicePort,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	tlsSecretName := ""
	if rt != nil && rt.GetCertificateSecretRef() != nil && *rt.GetCertificateSecretRef() != "" {
		tlsSecretName = *rt.GetCertificateSecretRef()
	}
	if tlsSecretName != "" && host != "" {
		ing.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{host},
				SecretName: tlsSecretName,
			},
		}
	} else {
		ing.Spec.TLS = nil
	}
}

// ExecuteCommandInContainer Execute command inside a container in a pod through API
func ExecuteCommandInContainer(config *rest.Config, podName, podNamespace, containerName string, command []string) (string, error) {

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create Clientset")
		return "", fmt.Errorf("Failed to create Clientset: %v", err.Error())
	}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(podNamespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command:   command,
		Container: containerName,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("Encountered error while creating Executor: %v", err.Error())
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	if err != nil {
		return stderr.String(), fmt.Errorf("Encountered error while running command: %v ; Stderr: %v ; Error: %v", command, stderr.String(), err.Error())
	}

	return stderr.String(), nil
}

// GetWatchNamespace returns the Namespace the operator should be watching for changes
func GetWatchNamespace() (string, error) {
	// WatchNamespaceEnvVar is the constant for env variable WATCH_NAMESPACE
	// which specifies the Namespace to watch.
	// An empty value means the operator is running with cluster scope.
	var watchNamespaceEnvVar = "WATCH_NAMESPACE"

	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	return ns, nil
}

// GetOperatorNamespace returns the Namespace the operator installed in
func GetOperatorNamespace() (string, error) {
	var podNamespaceEnvVar = "POD_NAMESPACE"

	ns, found := os.LookupEnv(podNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", podNamespaceEnvVar)
	}
	return ns, nil
}

func equals(sl1, sl2 []string) bool {
	if len(sl1) != len(sl2) {
		return false
	}
	for i, v := range sl1 {
		if v != sl2[i] {
			return false
		}
	}
	return true
}

func (r *ReconcilerBase) toJSONFromRaw(content *runtime.RawExtension) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(content.Raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}
