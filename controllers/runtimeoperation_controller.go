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

package controllers

import (
	"context"
	"math"
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appstacksv1 "github.com/application-stacks/runtime-component-operator/api/v1"
	"github.com/application-stacks/runtime-component-operator/utils"
	corev1 "k8s.io/api/core/v1"
)

// RuntimeOperationReconciler reconciles a RuntimeOperation object
type RuntimeOperationReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	RestConfig *rest.Config
}

// +kubebuilder:rbac:groups=rc.app.stacks,resources=runtimeoperations;runtimeoperations/status;runtimeoperations/finalizers,verbs=get;list;watch;create;update;patch;delete,namespace=runtime-component-operator
// +kubebuilder:rbac:groups=core,resources=pods;pods/exec,verbs=get;list;watch;create;update;patch;delete,namespace=runtime-component-operator

func (r *RuntimeOperationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling RuntimeOperation")

	// Fetch the RuntimeOperation instance
	instance := &appstacksv1.RuntimeOperation{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	//do not reconcile if the RuntimeOperation already completed
	oc := appstacksv1.GetOperationCondition(instance.Status.Conditions, appstacksv1.OperationStatusConditionTypeCompleted)
	if oc != nil && oc.Status == corev1.ConditionTrue {
		message := "RuntimeOperation '" + instance.Name + "' in namespace '" + req.Namespace + "' already completed. Create another RuntimeOperation instance to execute the command."
		r.Log.Info(message)
		r.Recorder.Event(instance, "Warning", "ProcessingError", message)
		return reconcile.Result{}, err
	}

	//do not reconcile if the RuntimeOperation already started
	oc = appstacksv1.GetOperationCondition(instance.Status.Conditions, appstacksv1.OperationStatusConditionTypeStarted)
	if oc != nil && oc.Status == corev1.ConditionTrue {
		message := "RuntimeOperation '" + instance.Name + "' in namespace '" + req.Namespace + "' already started and it can not be modified. Create another RuntimeOperation instance to execute the command."
		r.Log.Info(message)
		r.Recorder.Event(instance, "Warning", "ProcessingError", message)
		return reconcile.Result{}, err
	}

	//check if Pod exists and is in running state
	pod := &corev1.Pod{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.PodName, Namespace: req.Namespace}, pod)
	if err != nil || pod.Status.Phase != corev1.PodRunning {
		//handle error
		message := "Failed to find pod '" + instance.Spec.PodName + "' in namespace '" + req.Namespace + "'"
		if err == nil {
			message = message + " in running state"
		}
		return handleStartErrorAndRequeue(r, instance, err, message)
	}

	containerName := "app"
	if instance.Spec.ContainerName != "" {
		containerName = instance.Spec.ContainerName
	}

	//check if the specified container exists in the Pod
	foundContainer := false
	containerList := pod.Spec.Containers
	for i := 0; i < len(containerList); i++ {
		if containerList[i].Name == containerName {
			foundContainer = true
			break
		}
	}
	if !foundContainer {
		message := "Failed to find container '" + containerName + "' in pod '" + instance.Spec.PodName + "' in namespace '" + req.Namespace + "'"
		return handleStartErrorAndRequeue(r, instance, nil, message)
	}

	c := appstacksv1.OperationStatusCondition{
		Type:   appstacksv1.OperationStatusConditionTypeStarted,
		Status: corev1.ConditionTrue,
	}

	instance.Status.Conditions = appstacksv1.SetOperationCondition(instance.Status.Conditions, c)
	r.Client.Status().Update(context.TODO(), instance)

	_, err = utils.ExecuteCommandInContainer(r.RestConfig, pod.Name, pod.Namespace, containerName, instance.Spec.Command)
	if err != nil {
		//handle error
		r.Log.Error(err, "Execute command failed", "RuntimeOperation name", instance.Name, "command", instance.Spec.Command)
		r.Recorder.Event(instance, "Warning", "ProcessingError", err.Error())
		c = appstacksv1.OperationStatusCondition{
			Type:    appstacksv1.OperationStatusConditionTypeCompleted,
			Status:  corev1.ConditionFalse,
			Reason:  "Error",
			Message: err.Error(),
		}
		instance.Status.Conditions = appstacksv1.SetOperationCondition(instance.Status.Conditions, c)
		r.Client.Status().Update(context.TODO(), instance)
		return reconcile.Result{}, nil

	}

	c = appstacksv1.OperationStatusCondition{
		Type:   appstacksv1.OperationStatusConditionTypeCompleted,
		Status: corev1.ConditionTrue,
	}

	instance.Status.Conditions = appstacksv1.SetOperationCondition(instance.Status.Conditions, c)
	r.Client.Status().Update(context.TODO(), instance)
	return reconcile.Result{}, nil
}

func (r *RuntimeOperationReconciler) SetupWithManager(mgr ctrl.Manager) error {

	watchNamespaces, err := utils.GetWatchNamespaces()
	if err != nil {
		r.Log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	watchNamespacesMap := make(map[string]bool)
	for _, ns := range watchNamespaces {
		watchNamespacesMap[ns] = true
	}
	isClusterWide := len(watchNamespacesMap) == 1 && watchNamespacesMap[""]

	r.Log.V(1).Info("Adding a new controller", "watchNamespaces", watchNamespaces, "isClusterWide", isClusterWide)

	pred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() && (isClusterWide || watchNamespacesMap[e.ObjectOld.GetNamespace()])
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Object.GetNamespace()]
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Object.GetNamespace()]
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Object.GetNamespace()]
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&appstacksv1.RuntimeOperation{}, builder.WithPredicates(pred)).
		Complete(r)
}

// handleStartErrorAndRequeue updates OperationStatusConditionTypeStarted and requeues. It doubles the retry interval when the failure is due to same error.
func handleStartErrorAndRequeue(r *RuntimeOperationReconciler, instance *appstacksv1.RuntimeOperation, err error, message string) (reconcile.Result, error) {
	r.Log.Error(err, message)
	r.Recorder.Event(instance, "Warning", "ProcessingError", message)

	c := appstacksv1.OperationStatusCondition{
		Type:    appstacksv1.OperationStatusConditionTypeStarted,
		Status:  corev1.ConditionFalse,
		Reason:  "Error",
		Message: message,
	}

	var retryInterval time.Duration
	oldCondition := appstacksv1.GetOperationCondition(instance.Status.Conditions, c.Type)
	if oldCondition == nil || oldCondition.LastUpdateTime.Time.IsZero() || oldCondition.Message != c.Message {
		retryInterval = time.Second
	} else {
		retryInterval = time.Now().Sub(oldCondition.LastUpdateTime.Time).Round(time.Second)
	}

	instance.Status.Conditions = appstacksv1.SetOperationCondition(instance.Status.Conditions, c)
	r.Client.Status().Update(context.TODO(), instance)
	return reconcile.Result{
		RequeueAfter: time.Duration(math.Min(float64(retryInterval.Nanoseconds()*2), float64(time.Hour.Nanoseconds()*6))),
		Requeue:      true,
	}, nil
}
