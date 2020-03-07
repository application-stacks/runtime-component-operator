# Runtime Component Operator v0.4.0

## Installation

The Runtime Component Operator can be installed to:

- watch own namespace
- watch another namespace
- watch multiple namespaces
- watch all namespaces in the cluster

Appropriate cluster role and binding are required to watch another namespace, watch multiple namespaces or watch all namespaces.

---

1. Install `RuntimeComponent` Custom Resource Definition (CRD). This needs to be done only ONCE per cluster:

    ```console
    kubectl apply -f https://raw.githubusercontent.com/application-stacks/runtime-component-operator/master/deploy/releases/0.4.0/runtime-component-crd.yaml
    ```

2. Install the Runtime Component Operator:

    **Important: In Step 2.1, ensure that you replace  `<SPECIFY_OPERATOR_NAMESPACE_HERE>` and `<SPECIFY_WATCH_NAMESPACE_HERE>` with proper values:**

    2.1. Set operator namespace and the namespace to watch:

    - To watch all namespaces in the cluster, set `WATCH_NAMESPACE='""'`
    - To watch multiple namespaces in the cluster, set `WATCH_NAMESPACE` to a comma-separated list of namespaces e.g. `WATCH_NAMESPACE=example-ns-1,example-ns-2,example-ns-3`

    ```console
    OPERATOR_NAMESPACE=<SPECIFY_OPERATOR_NAMESPACE_HERE>
    WATCH_NAMESPACE=<SPECIFY_WATCH_NAMESPACE_HERE>
    ```

    2.2. _Optional_: Install cluster-level role-based access. This step can be skipped if the operator is only watching own namespace:
  
    ```console
    curl -L https://raw.githubusercontent.com/application-stacks/runtime-component-operator/master/deploy/releases/0.4.0/runtime-component-cluster-rbac.yaml \
      | sed -e "s/RUNTIME_COMPONENT_OPERATOR_NAMESPACE/${OPERATOR_NAMESPACE}/" \
      | kubectl apply -f -
    ```

    2.3. Install the operator:

    ```console
    curl -L https://raw.githubusercontent.com/application-stacks/runtime-component-operator/master/deploy/releases/0.4.0/runtime-component-operator.yaml \
      | sed -e "s/RUNTIME_COMPONENT_WATCH_NAMESPACE/${WATCH_NAMESPACE}/" \
      | kubectl apply -n ${OPERATOR_NAMESPACE} -f -
    ```

## Uninstallation

To uninstall the operator, run commands from Step 2.3 first and then Step 2.2 (if applicable), but after replacing `kubectl apply` with `kubectl delete`.

To delete the CRD, run command from Step 1, but after replacing `kubectl apply` with `kubectl delete`.

_Deleting the CRD will also delete all `RuntimeComponent` in the cluster_

## Current Limitations

- Knative support is limited. Values specified for `autoscaling`, `resources` and `replicas` parameters would not apply for Knative when enabled using `createKnativeService` parameter.
- The auto-creation of an application definition by kAppNav is not supported when Knative is enabled.
- Monitoring feature does not support integration with Knative Service. Prometheus Operator is required to use ServiceMonitor.
- After the initial deployment of `RuntimeComponent`, any changes to its labels would be applied only when one of the parameters from `spec` is updated.