package v1beta1

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OperationStatusCondition ...
// +k8s:openapi-gen=true
type OperationStatusCondition struct {
	LastTransitionTime *metav1.Time                 `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     metav1.Time                  `json:"lastUpdateTime,omitempty"`
	Reason             string                       `json:"reason,omitempty"`
	Message            string                       `json:"message,omitempty"`
	Status             corev1.ConditionStatus       `json:"status,omitempty"`
	Type               OperationStatusConditionType `json:"type,omitempty"`
}

// OperationStatusConditionType ...
type OperationStatusConditionType string

const (
	// OperationStatusConditionTypeStarted indicates whether operation has been started
	OperationStatusConditionTypeStarted OperationStatusConditionType = "Started"
	// OperationStatusConditionTypeCompleted indicates whether operation has been completed
	OperationStatusConditionTypeCompleted OperationStatusConditionType = "Completed"
)

// GetOperationCondition returns condition of specific type
func GetOperationCondition(c []OperationStatusCondition, t OperationStatusConditionType) *OperationStatusCondition {
	for i := range c {
		if c[i].Type == t {
			return &c[i]
		}
	}
	return nil
}

// SetOperationCondition set condition of specific type or appends if not present
func SetOperationCondition(c []OperationStatusCondition, oc OperationStatusCondition) []OperationStatusCondition {
	condition := GetOperationCondition(c, oc.Type)

	if condition != nil {
		if condition.Status != oc.Status {
			condition.LastTransitionTime = &metav1.Time{Time: time.Now()}
		}
		condition.Status = oc.Status
		condition.LastUpdateTime = metav1.Time{Time: time.Now()}
		condition.Reason = oc.Reason
		condition.Message = oc.Message
		return c
	}
	oc.LastUpdateTime = metav1.Time{Time: time.Now()}
	c = append(c, oc)
	return c
}
