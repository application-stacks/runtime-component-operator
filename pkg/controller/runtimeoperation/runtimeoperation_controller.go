package runtimeoperation

import (
	"context"
	"os"

	appstacksv1beta1 "github.com/application-stacks/runtime-component-operator/pkg/apis/appstacks/v1beta1"
	utils "github.com/application-stacks/runtime-component-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_runtimeoperation")

// Add creates a new RuntimeOperation Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRuntimeOperation{client: mgr.GetClient(), scheme: mgr.GetScheme(), recorder: mgr.GetEventRecorderFor("runtime-component-operator"), restConfig: mgr.GetConfig()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("runtimeoperation-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	watchNamespaces, err := utils.GetWatchNamespaces()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	watchNamespacesMap := make(map[string]bool)
	for _, ns := range watchNamespaces {
		watchNamespacesMap[ns] = true
	}
	isClusterWide := len(watchNamespacesMap) == 1 && watchNamespacesMap[""]

	log.V(1).Info("Adding a new controller", "watchNamespaces", watchNamespaces, "isClusterWide", isClusterWide)

	pred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.MetaOld.GetGeneration() != e.MetaNew.GetGeneration() && (isClusterWide || watchNamespacesMap[e.MetaOld.GetNamespace()])
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Meta.GetNamespace()]
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Meta.GetNamespace()]
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return isClusterWide || watchNamespacesMap[e.Meta.GetNamespace()]
		},
	}

	// Watch for changes to primary resource RuntimeOperation
	err = c.Watch(&source.Kind{Type: &appstacksv1beta1.RuntimeOperation{}}, &handler.EnqueueRequestForObject{}, pred)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileRuntimeOperation implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileRuntimeOperation{}

// ReconcileRuntimeOperation reconciles a RuntimeOperation object
type ReconcileRuntimeOperation struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	recorder   record.EventRecorder
	restConfig *rest.Config
}

// Reconcile reads that state of the cluster for a RuntimeOperation object and makes changes based on the state read
// and what is in the RuntimeOperation.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileRuntimeOperation) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling RuntimeOperation")

	// Fetch the RuntimeOperation instance
	instance := &appstacksv1beta1.RuntimeOperation{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
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

	//do not reconcile if the RuntimeOperation already started
	oc := appstacksv1beta1.GetOperationCondition(instance.Status.Conditions, appstacksv1beta1.OperationStatusConditionTypeStarted)
	if oc != nil && oc.Status == corev1.ConditionTrue {
		message := "RuntimeOperation '" + instance.Name + "' in namespace '" + request.Namespace + "' already started and it can not be modified. Create another RuntimeOperation instance to execute the command."
		log.Info(message)
		r.recorder.Event(instance, "Warning", "ProcessingError", message)
		return reconcile.Result{}, err
	}

	//check if Pod exists and running
	pod := &corev1.Pod{}

	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.PodName, Namespace: request.Namespace}, pod)
	if err != nil || pod.Status.Phase != corev1.PodRunning {
		//handle error
		message := "Failed to find pod '" + instance.Spec.PodName + "' in namespace '" + request.Namespace + "'"
		log.Error(err, message)
		r.recorder.Event(instance, "Warning", "ProcessingError", message)
		c := appstacksv1beta1.OperationStatusCondition{
			Type:    appstacksv1beta1.OperationStatusConditionTypeStarted,
			Status:  corev1.ConditionFalse,
			Reason:  "Error",
			Message: "Failed to find pod '" + instance.Spec.PodName + "' or it's not in running state",
		}
		instance.Status.Conditions = appstacksv1beta1.SetOperationCondition(instance.Status.Conditions, c)
		r.client.Status().Update(context.TODO(), instance)
		return reconcile.Result{}, nil
	}

	c := appstacksv1beta1.OperationStatusCondition{
		Type:   appstacksv1beta1.OperationStatusConditionTypeStarted,
		Status: corev1.ConditionTrue,
	}

	instance.Status.Conditions = appstacksv1beta1.SetOperationCondition(instance.Status.Conditions, c)
	r.client.Status().Update(context.TODO(), instance)

	containerName := "app"
	if instance.Spec.ContainerName != "" {
		containerName = instance.Spec.ContainerName
	}

	_, err = utils.ExecuteCommandInContainer(r.restConfig, pod.Name, pod.Namespace, containerName, instance.Spec.Command)
	if err != nil {
		//handle error
		log.Error(err, "Execute command failed", "RuntimeOperation name", instance.Name, "command", instance.Spec.Command)
		r.recorder.Event(instance, "Warning", "ProcessingError", err.Error())
		c = appstacksv1beta1.OperationStatusCondition{
			Type:    appstacksv1beta1.OperationStatusConditionTypeCompleted,
			Status:  corev1.ConditionFalse,
			Reason:  "Error",
			Message: err.Error(),
		}
		instance.Status.Conditions = appstacksv1beta1.SetOperationCondition(instance.Status.Conditions, c)
		r.client.Status().Update(context.TODO(), instance)
		return reconcile.Result{}, nil

	}

	c = appstacksv1beta1.OperationStatusCondition{
		Type:   appstacksv1beta1.OperationStatusConditionTypeCompleted,
		Status: corev1.ConditionTrue,
	}

	instance.Status.Conditions = appstacksv1beta1.SetOperationCondition(instance.Status.Conditions, c)
	r.client.Status().Update(context.TODO(), instance)
	return reconcile.Result{}, nil
}
