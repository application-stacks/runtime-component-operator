# Troubleshooting

Here are some basic troubleshooting methods to check if the operator is running fine:

* Run the following and check if the output is similar to the following:

  ```console
  $ oc get pods -l name=application-stacks-operator

  NAME                                            READY     STATUS    RESTARTS   AGE
  application-stacks-operator-584d6bd86d-fzq2n   1/1       Running   0          33m
  ```

* Check the operators events:

  ```console
  $ oc describe pod application-stacks-operator-584d6bd86d-fzq2n
  ```

* Check the operator logs:

  ```console
  $ oc logs application-stacks-operator-584d6bd86d-fzq2n
  ```

If the operator is running fine, check the status of the `RuntimeComponent` Custom Resource (CR) instance:

* Check the CR status:

  ```console
  $ oc get runtimecomponent my-app -o wide

  NAME                      IMAGE                                             EXPOSED   RECONCILED   REASON    MESSAGE   AGE
  my-app            quay.io/my-repo/my-app:1.0                                false     True                             1h
  ```

* Check the CR effective fields:

  ```console
  $ oc get runtimecomponent my-app -o yaml
  ```

  Ensure that the effective CR values are what you want.

* Check the `status` section of the CR. If the CR was successfully reconciled, the output should look like the following:

  ```console
  $ oc get runtimecomponent my-app -o yaml

  apiVersion: app.stacks/v1beta1
  kind: RuntimeComponent
  ...
  status:
    conditions:
    - lastTransitionTime: 2019-08-21T22:20:49Z
      lastUpdateTime: 2019-08-21T22:39:42Z
      status: "True"
      type: Reconciled
  ```

* Check the CR events:

  ```console
  $ oc describe runtimecomponent my-app
  ```
