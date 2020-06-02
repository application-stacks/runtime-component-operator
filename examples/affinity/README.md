# Assigning Pods to Nodes

## Introduction

This scenario illustrates how to deploy applications using Runtime Component Operator and assigning application pods to specific nodes in a Kubernetes cluster.

You will deploy instances of two applications, `coffeeshop-frontend` and `coffeeshop-backend`, co-located on the same nodes with SSD storage type.

This scenario is inspired by examples from [Assigning Pods to Nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) Kubernetes tutorial.

## Affinity

For this example, imagine a multi-node cluster has a mix of nodes including nodes that use SSD storage. The nodes that use SSD storage are labelled with `disktype: ssd`. You can use the following command to label nodes:

```console
oc label nodes <node-name> <label-key>=<label-value>
```

You can verify labels specified on nodes by re-running the following command and checking that the node now has a label:

```console
oc get nodes --show-labels
```

In this cluster, we want the `coffeeshop-frontend` and `coffeeshop-backend` applications to be co-located on the same nodes with SSD storage types.

Here is a YAML snippet of the `RuntimeComponent` custom resource (CR) for the `coffeeshop-backend` application with two replicas. The custom resource has `.spec.affinity.podAntiAffinity` configured to ensure the scheduler does not co-locate pods for the application on a single node. As described in the [user guide](https://github.com/application-stacks/runtime-component-operator/blob/master/doc/user-guide.adoc#labels), the application pods are labelled with `app.kubernetes.io/instance: metadata.name`.

The custom resource also has `spec.affinity.nodeAffinityLabels` to ensure the application pods will run on nodes with the `disktype: ssd` label.

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: coffeeshop-backend
spec:
  applicationImage: 'k8s.gcr.io/pause:2.0'
  replicas: 2
  affinity:
    nodeAffinityLabels:
      disktype: ssd
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/instance
                operator: In
                values:
                  - coffeeshop-backend
          topologyKey: kubernetes.io/hostname
```

The YAML snippet below shows the `RuntimeComponent` CR for the `coffeeshop-frontend` application. The snippet has the `.spec.affinity.podAffinity` configured which informs the scheduler that all its replicas are to be co-located with pods that have selector label `app.kubernetes.io/instance: coffeeshop-backend`. The CR also includes `.spec.affinity.podAntiAffinity` to ensure that each `coffeeshop-frontend` replica does not co-locate on a single node. In addition, the `spec.affinity.nodeAffinityLabels` parameter is specified to ensure the application pods will run on nodes with the `disktype: ssd` label.

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: coffeeshop-frontend
spec:
  applicationImage: 'k8s.gcr.io/pause:2.0'
  replicas: 2
  affinity:
    nodeAffinityLabels:
      disktype: ssd
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/instance
                operator: In
                values:
                  - coffeeshop-frontend
          topologyKey: kubernetes.io/hostname
    podAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/instance
                operator: In
                values:
                  - coffeeshop-backend
          topologyKey: kubernetes.io/hostname
```

To see where the which nodes the pods are running, run the following command:

```console
oc get pods -o wide
```

And the output would look like the following:

```console
NAME                                   READY   STATUS    RESTARTS   AGE     IP              NODE
coffeeshop-backend-f57557d99-5lm24     1/1     Running   0          6m8s    10.254.20.173   worker0.shames.os.fyre.ibm.com
coffeeshop-backend-f57557d99-fv62n     1/1     Running   0          6m8s    10.254.5.39     worker1.shames.os.fyre.ibm.com
coffeeshop-frontend-789c585488-5k7sh   1/1     Running   0          5m52s   10.254.5.38     worker1.shames.os.fyre.ibm.com
coffeeshop-frontend-789c585488-9m529   1/1     Running   0          5m52s   10.254.20.172   worker0.shames.os.fyre.ibm.com
```

In this example, you saw how to use the [Affinity](https://github.com/application-stacks/runtime-component-operator/blob/master/doc/user-guide.adoc#affinity) feature in the Runtime Component Operator to assign pods to certain nodes within a cluster.