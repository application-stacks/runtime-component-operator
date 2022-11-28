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
	"fmt"
	"strconv"

	"github.com/application-stacks/runtime-component-operator/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ReconcilerBase) CheckApplicationStatus(ba common.BaseComponent) corev1.ConditionStatus {
	s := ba.GetStatus()

	status, msg, reason := corev1.ConditionFalse, "", ""

	// Check application and resources status
	scReconciled := s.GetCondition(common.StatusConditionTypeReconciled)

	// If not reconciled, resources and endpoints will not be ready and reconciled status will show the errors
	if scReconciled == nil || scReconciled.GetStatus() != corev1.ConditionTrue {
		msg = "Application is not reconciled."
		reason = "ApplicationNotReconciled"
	} else {
		// If reconciled, check resources status and endpoint information
		r.CheckResourcesStatus(ba)
		r.ReportExternalEndpointStatus(ba)

		scReady := s.GetCondition(common.StatusConditionTypeResourcesReady)
		if scReady == nil || scReady.GetStatus() != corev1.ConditionTrue {
			msg = "Resources are not ready."
			reason = "ResourcesNotReady"
		} else {
			status = corev1.ConditionTrue
			msg = common.StatusConditionTypeReadyMessage
		}
	}

	// Check Application Ready status condition is created/updated
	conditionType := common.StatusConditionTypeReady
	oldCondition := s.GetCondition(conditionType)
	newCondition := s.NewCondition(conditionType)
	newCondition.SetConditionFields(msg, reason, status)
	r.setCondition(ba, oldCondition, newCondition)

	return status
}

func (r *ReconcilerBase) CheckResourcesStatus(ba common.BaseComponent) {
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

	r.setCondition(ba, oldCondition, newCondition)
}

func (r *ReconcilerBase) setCondition(ba common.BaseComponent, oldCondition common.StatusCondition, newCondition common.StatusCondition) {
	s := ba.GetStatus()

	// Check if status or message changed
	if oldCondition == nil || oldCondition.GetStatus() != newCondition.GetStatus() || oldCondition.GetMessage() != newCondition.GetMessage() {
		// Set condition and update status
		s.SetCondition(newCondition)
	}
}

func (r *ReconcilerBase) areReplicasReady(ba common.BaseComponent, c common.StatusCondition) common.StatusCondition {
	obj := ba.(client.Object)
	namespacedName := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}

	resourceType, msg, reason := "", "", ""
	var replicas, readyReplicas, updatedReplicas, readyUpdatedReplicas int32
	var minReplicas int32 = 1

	expectedReplicas := ba.GetReplicas()
	autoScale := ba.GetAutoscaling()

	// If both are not specified, expected replica is set to 1
	if expectedReplicas == nil && autoScale == nil {
		expectedReplicas = &minReplicas
	}

	if ba.GetStatefulSet() == nil {
		// Check if deployment exists
		deployment := &appsv1.Deployment{}
		err := r.GetClient().Get(context.TODO(), namespacedName, deployment)
		if err != nil {
			msg, reason = "Deployment is not ready.", "NotCreated"
			return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
		}
		// Get replicas
		resourceType = "Deployment"
		ds := deployment.Status
		replicas, readyReplicas, updatedReplicas = ds.Replicas, ds.ReadyReplicas, ds.UpdatedReplicas
	} else {
		// Check if statefulSet exists
		statefulSet := &appsv1.StatefulSet{}
		err := r.GetClient().Get(context.TODO(), namespacedName, statefulSet)
		if err != nil {
			msg, reason = "StatefulSet is not ready.", "NotCreated"
			return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
		}
		// Get replicas
		resourceType = "StatefulSet"
		ss := statefulSet.Status
		replicas, readyReplicas, updatedReplicas = ss.Replicas, ss.ReadyReplicas, ss.UpdatedReplicas
	}

	// Get replicas that are ready and updated
	if readyReplicas <= updatedReplicas {
		readyUpdatedReplicas = readyReplicas
	} else {
		readyUpdatedReplicas = updatedReplicas
	}

	msg = resourceType + " replicas ready: " + strconv.Itoa(int(readyUpdatedReplicas))
	reason = "MinimumReplicasUnavailable"

	// Check autoscaling parameters
	if autoScale != nil {
		autoMinReplicas := autoScale.GetMinReplicas()
		autoMaxReplicas := autoScale.GetMaxReplicas()
		if autoMinReplicas == nil {
			autoMinReplicas = &minReplicas
		}
		// Check if the replicas are more than min and less than max
		if readyUpdatedReplicas < *autoMinReplicas {
			msg = msg + " < minReplicas: " + strconv.Itoa(int(*autoMinReplicas))
			return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
		} else if replicas > autoMaxReplicas {
			msg = "Replica set is progressing"
			reason = "ReplicaSetUpdating"
			return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
		}
		reason = "MinimumReplicasAvailable"
		return c.SetConditionFields(msg, reason, corev1.ConditionTrue)
	}

	// Check if all replicas are equal to the expected replicas
	msg = msg + "/" + strconv.Itoa(int(*expectedReplicas))
	if replicas == *expectedReplicas && readyReplicas == *expectedReplicas && updatedReplicas == *expectedReplicas {
		reason = "MinimumReplicasAvailable"
		return c.SetConditionFields(msg, reason, corev1.ConditionTrue)
	} else if replicas > *expectedReplicas {
		reason = "ReplicaSetUpdating"
		msg = "Replica set is progressing"
		return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
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
		msg, reason = "Knative service is not ready.", "NotCreated"
		return c.SetConditionFields(msg, reason, corev1.ConditionFalse)
	}

	msg = "Knative service is ready."
	return c.SetConditionFields(msg, reason, corev1.ConditionTrue)
}

func (r *ReconcilerBase) ReportExternalEndpointStatus(ba common.BaseComponent) {
	name := "Ingress"
	s := ba.GetStatus()

	// If application not exposed or uses Knative service, remove route/ingress endpoint information
	if ba.GetExpose() == nil || !*ba.GetExpose() || (ba.GetCreateKnativeService() != nil && *ba.GetCreateKnativeService()) {
		s.RemoveStatusEndpoint(name)
		return
	}

	host, path, protocol := r.GetIngressInfo(ba)
	// If route/ingress host is empty, host is set to wildcard
	if host == "" {
		host = "*"
	}

	endpoint := fmt.Sprintf("%s://%s%s", protocol, host, path)

	oldEndpoint := s.GetStatusEndpoint(name)
	newEndpoint := s.NewStatusEndpoint(name)

	// Check if endpoint information has been changed
	if oldEndpoint == nil || oldEndpoint.GetEndpointUri() != endpoint {
		// Set endpoint information fields and update status
		endpointType := "Application"
		endpointScope := common.StatusEndpointScopeExternal

		newEndpoint.SetStatusEndpointFields(endpointScope, endpointType, endpoint)
		s.SetStatusEndpoint(newEndpoint)
	}
}
