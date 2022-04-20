/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"strconv"

	"github.com/application-stacks/runtime-component-operator/common"
	routev1 "github.com/openshift/api/route/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ReconcilerBase) CheckApplicationStatus(ba common.BaseComponent) (error, corev1.ConditionStatus) {
	s := ba.GetStatus()

	// Check resources status condition and if condition is updated
	rStatusUpdated := r.CheckResourcesStatus(ba)

	status, msg, reason := corev1.ConditionFalse, "", ""

	// Check application and resources status
	scReconciled := s.GetCondition(common.StatusConditionTypeReconciled)
	scReady := s.GetCondition(common.StatusConditionTypeResourcesReady)

	if scReconciled == nil || scReconciled.GetStatus() != corev1.ConditionTrue {
		msg = "Application is not reconciled."
		reason = "ApplicationNotReconciled"
	} else if scReady == nil || scReady.GetStatus() != corev1.ConditionTrue {
		msg = "Resources are not ready."
		reason = "ResourcesNotReady"
	} else {
		status = corev1.ConditionTrue
		msg = "Application is reconciled and resources are ready."
	}

	// Check Application Ready status condition is created/updated
	conditionType := common.StatusConditionTypeReady
	oldCondition := s.GetCondition(conditionType)
	newCondition := s.NewCondition(conditionType)
	newCondition.SetConditionFields(msg, reason, status)
	appStatusUpdated := r.setCondition(ba, oldCondition, newCondition)

	if rStatusUpdated || appStatusUpdated {
		// Detect errors while updating status
		if err := r.UpdateStatus(ba.(client.Object)); err != nil {
			log.Error(err, "Unable to update status")
			return err, status
		}
	}
	return nil, status
}

func (r *ReconcilerBase) CheckResourcesStatus(ba common.BaseComponent) bool {
	// Create Resource Ready status condition if it does not exit
	s := ba.GetStatus()
	conditionType := common.StatusConditionTypeResourcesReady
	oldCondition := s.GetCondition(conditionType)
	newCondition := s.NewCondition(conditionType)

	// Check for Deployment, StatefulSet replicas or Knative service status
	if ba.GetCreateKnativeService() == nil || !*ba.GetCreateKnativeService() {
		newCondition = r.areReplicasReady(ba, newCondition)
	} else {
		newCondition = r.isKnativeReady(ba, newCondition)
	}

	return r.setCondition(ba, oldCondition, newCondition)
}

func (r *ReconcilerBase) setCondition(ba common.BaseComponent, oldCondition common.StatusCondition, newCondition common.StatusCondition) bool {
	s := ba.GetStatus()

	// Check if status or message changed
	if oldCondition == nil || oldCondition.GetStatus() != newCondition.GetStatus() || oldCondition.GetMessage() != newCondition.GetMessage() {
		// Set condition and update status
		s.SetCondition(newCondition)
		return true
	}
	return false
}

func (r *ReconcilerBase) areReplicasReady(ba common.BaseComponent, c common.StatusCondition) common.StatusCondition {
	obj := ba.(client.Object)
	namespacedName := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}

	resourceType, msg, reason := "", "", ""

	var replicas, readyReplicas, updatedReplicas int32

	expectedReplicas := ba.GetReplicas()
	autoScale := ba.GetAutoscaling()

	if ba.GetStatefulSet() == nil {
		deployment := &appsv1.Deployment{}

		// Check if deployment exists
		err := r.GetClient().Get(context.TODO(), namespacedName, deployment)
		if err != nil || (expectedReplicas == nil && (autoScale == nil)) {
			msg, reason = "Deployment is not ready.", "NotCreated"
			return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
		}

		resourceType = "Deployment"
		ds := deployment.Status
		replicas, readyReplicas, updatedReplicas = ds.Replicas, ds.ReadyReplicas, ds.UpdatedReplicas
	} else {
		statefulSet := &appsv1.StatefulSet{}

		// Check if statefulSet exists
		err := r.GetClient().Get(context.TODO(), namespacedName, statefulSet)
		if err != nil || (expectedReplicas == nil && autoScale == nil) {
			msg, reason = "StatefulSet is not ready.", "NotCreated"
			return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
		}

		resourceType = "StatefulSet"
		ss := statefulSet.Status
		replicas, readyReplicas, updatedReplicas = ss.Replicas, ss.ReadyReplicas, ss.UpdatedReplicas
	}

	msg = resourceType + " replicas ready: " + strconv.Itoa(int(readyReplicas))
	reason = "MinimumReplicasUnavailable"

	// Check autoscaling parameters
	if autoScale != nil {
		var minReplicas int32 = 1
		autoMinReplicas := autoScale.GetMinReplicas()
		if autoMinReplicas == nil {
			autoMinReplicas = &minReplicas
		}
		if readyReplicas < *autoMinReplicas {
			msg = msg + " < minReplicas: " + strconv.Itoa(int(*autoMinReplicas))
			return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
		}
		reason = "MinimumReplicasAvailable"
		return c.SetConditionFields(msg, reason, corev1.ConditionTrue)
	}
	// Replicas check
	msg = msg + "/" + strconv.Itoa(int(*expectedReplicas))

	if replicas == *expectedReplicas && readyReplicas == *expectedReplicas && updatedReplicas == *expectedReplicas {
		reason = "MinimumReplicasAvailable"
		return c.SetConditionFields(msg, reason, corev1.ConditionTrue)
	} else if replicas > *expectedReplicas {
		reason = "ReplicaSetUpdating"
		msg = "Replica set is progressing"
		return c.SetConditionFields(msg, reason, corev1.ConditionTrue)
	}
	return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
}

func (r *ReconcilerBase) isKnativeReady(ba common.BaseComponent, c common.StatusCondition) common.StatusCondition {
	knative := &servingv1.Service{}
	obj := ba.(client.Object)
	namespacedName := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}

	msg, reason := "", ""

	// Check if knative service exists
	if err := r.GetClient().Get(context.TODO(), namespacedName, knative); err != nil {
		msg, reason = "Knative is not ready.", "NotCreated"
		return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
	}

	msg = "Knative service is ready."
	return c.SetConditionFields(msg, reason, corev1.ConditionTrue)
}

func (r *ReconcilerBase) ReportExternalEndpointStatus(ba common.BaseComponent) error {
	// If application not exposed or uses Knative service, remove route/ingress endpoint information
	if ba.GetExpose() == nil || !*ba.GetExpose() || (ba.GetCreateKnativeService() != nil && *ba.GetCreateKnativeService()) {
		return r.RemoveExternalEndpointStatus(ba)
	}

	obj := ba.(client.Object)
	namespacedName := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}

	host, path := "", ""

	if ok, _ := r.IsGroupVersionSupported(routev1.SchemeGroupVersion.String(), "Route"); ok {
		// Check if route exists and get host + path
		route := &routev1.Route{}
		if err := r.GetClient().Get(context.TODO(), namespacedName, route); err != nil {
			log.Error(err, "Route resource not found")
			return err
		}
		host = route.Spec.Host
		path = route.Spec.Path
	} else {
		if ok, _ := r.IsGroupVersionSupported(networkingv1.SchemeGroupVersion.String(), "Ingress"); ok {
			// Check if ingress exists and get host + path
			ingress := &networkingv1.Ingress{}
			if err := r.GetClient().Get(context.TODO(), namespacedName, ingress); err != nil {
				log.Error(err, "Ingress resource not found")
				return err
			}

			if ingress.Spec.Rules != nil && len(ingress.Spec.Rules) != 0 {
				host = ingress.Spec.Rules[0].Host
				if ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths != nil && len(ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths) != 0 {
					path = ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path
				}
			}
		}
	}

	// If route/ingress host is empty, host is set to wildcard
	if host == "" {
		host = "*"
	}

	s := ba.GetStatus()
	oldEndpoint := s.GetStatusEndpoint(obj.GetName())
	newEndpoint := s.NewStatusEndpoint(obj.GetName())

	// If endpoint has been changed
	endpoint := host + path
	if oldEndpoint == nil || oldEndpoint.GetEndpointUri() != endpoint {
		// Set endpoint information fields and update status
		endpointType := "Application"
		endpointScope := common.StatusEndpointScopeExternal

		newEndpoint.SetStatusEndpointFields(endpointScope, endpointType, endpoint)
		s.SetStatusEndpoint(newEndpoint)

		// Detect errors while updating status
		if err := r.UpdateStatus(obj); err != nil {
			log.Error(err, "Unable to update status")
			return err
		}
	}
	return nil
}

func (r *ReconcilerBase) RemoveExternalEndpointStatus(ba common.BaseComponent) error {
	// Remove endpoint in status if not resource is no longer exposed
	obj := ba.(client.Object)
	s := ba.GetStatus()
	s.RemoveStatusEndpoint(obj.GetName())

	// Detect errors while updating status
	if err := r.UpdateStatus(obj); err != nil {
		log.Error(err, "Unable to update status")
		return err
	}
	return nil
}
