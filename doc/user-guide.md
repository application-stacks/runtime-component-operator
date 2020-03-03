# Runtime Component Operator

This generic Operator is capable of deploying any application image and can be imported into any runtime-specific Operator as library of application capabilities.  This architecture ensures compatibility and consistency between all runtime Operators, allowing everyone to benefit from the functionality added in this project.

## Operator installation

Use the instructions for one of the releases to install the operator into a Kubernetes cluster.

The Runtime Component Operator can be installed to:

- watch own namespace
- watch another namespace
- watch multiple namespaces
- watch all namespaces in the cluster

Appropriate cluster roles and bindings are required to watch another namespace, watch multiple namespaces or watch all namespaces.

## Overview

The architecture of the Runtime Component Operator follows the basic controller pattern:  the Operator container with the controller is deployed into a Pod and listens for incoming resources with `Kind: RuntimeComponent`. Creating a `RuntimeComponent` custom resource (CR) triggers the Runtime Component Operator to create, update or delete Kubernetes resources needed by the application to run on your cluster.

Each instance of `RuntimeComponent` CR represents the application to be deployed on the cluster:

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  service:
    type: ClusterIP
    port: 9080
  expose: true
  storage:
    size: 2Gi
    mountPath: "/logs"
```

## Configuration

### Custom Resource Definition (CRD)

The following table lists configurable parameters of the `RuntimeComponent` CRD. For complete OpenAPI v3 representation of these values please see [`RuntimeComponent` CRD](../deploy/crds/app.stacks_runtimecomponents_crd.yaml).

Each `RuntimeComponent` CR must at least specify the `applicationImage` parameter. Specifying other parameters is optional.

| Parameter | Description |
|---|---|
| `version` | The current version of the application. Label `app.kubernetes.io/version` will be added to all resources when the version is defined. |
| `serviceAccountName` | The name of the OpenShift service account to be used during deployment. |
| `applicationImage` | The Docker image name to be deployed. On OpenShift, it can also be set to `<project name>/<image stream name>[:<tag>]` to reference an image from an image stream. If `<project name>` and `<tag>` values are not defined, they default to the namespace of the CR and the value of `latest`, respectively. |
| `pullPolicy` | The policy used when pulling the image.  One of: `Always`, `Never`, and `IfNotPresent`. |
| `pullSecret` | If using a registry that requires authentication, the name of the secret containing credentials. |
| `initContainers` | The list of [Init Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#container-v1-core) definitions. |
| `architecture` | An array of architectures to be considered for deployment. Their position in the array indicates preference. |
| `service.port` | The port exposed by the container. |
| `service.type` | The Kubernetes [Service Type](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types). |
| `service.annotations` | Annotations to be added to the service. |
| `service.certificate` | A YAML object representing a [Certificate](https://cert-manager.io/docs/reference/api-docs/#cert-manager.io/v1alpha2.CertificateSpec). |
| `service.certificateSecretRef` | A name of a secret that already contains TLS key, certificate and CA to be mounted in the pod.  |
| `service.provides.category` | Service binding type to be provided by this CR. At this time, the only allowed value is `openapi`. |
| `service.provides.protocol` | Protocol of the provided service. Defauts to `http`. |
| `service.provides.context` | Specifies context root of the service. |
| `service.provides.auth.username` | Optional value to specify username as [SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#secretkeyselector-v1-core). |
| `service.provides.auth.password` | Optional value to specify password as [SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#secretkeyselector-v1-core). |
| `service.consumes` | An array consisting of services to be consumed by the `RuntimeComponent`. |
| `service.consumes[].category` | The type of service binding to be consumed. At this time, the only allowed value is `openapi`. |
| `service.consumes[].name` | The name of the service to be consumed. If binding to a `RuntimeComponent`, then this would be the provider's CR name. |
| `service.consumes[].namespace` | The namespace of the service to be consumed. If binding to a `RuntimeComponent`, then this would be the provider's CR name. ||
| `service.consumes[].mountPath` | Optional field to specify which location in the pod, service binding secret should be mounted. If not specified, the secret keys would be injected as environment variables. |
| `createKnativeService`   | A boolean to toggle the creation of Knative resources and usage of Knative serving. |
| `expose`   | A boolean that toggles the external exposure of this deployment via a Route or a Knative Route resource.|
| `replicas` | The static number of desired replica pods that run simultaneously. |
| `autoscaling.maxReplicas` | Required field for autoscaling. Upper limit for the number of pods that can be set by the autoscaler. It cannot be lower than the minimum number of replicas. |
| `autoscaling.minReplicas`   | Lower limit for the number of pods that can be set by the autoscaler. |
| `autoscaling.targetCPUUtilizationPercentage`   | Target average CPU utilization (represented as a percentage of requested CPU) over all the pods. |
| `resourceConstraints.requests.cpu` | The minimum required CPU core. Specify integers, fractions (e.g. 0.5), or millicore values(e.g. 100m, where 100m is equivalent to .1 core). Required field for autoscaling. |
| `resourceConstraints.requests.memory` | The minimum memory in bytes. Specify integers with one of these suffixes: E, P, T, G, M, K, or power-of-two equivalents: Ei, Pi, Ti, Gi, Mi, Ki.|
| `resourceConstraints.limits.cpu` | The upper limit of CPU core. Specify integers, fractions (e.g. 0.5), or millicores values(e.g. 100m, where 100m is equivalent to .1 core). |
| `resourceConstraints.limits.memory` | The memory upper limit in bytes. Specify integers with suffixes: E, P, T, G, M, K, or power-of-two equivalents: Ei, Pi, Ti, Gi, Mi, Ki.|
| `env`   | An array of environment variables following the format of `{name, value}`, where value is a simple string. It may also follow the format of `{name, valueFrom}`, where valueFrom refers to a value in a `ConfigMap` or `Secret` resource. See [Environment variables](https://github.com/application-stacks/operator/blob/master/doc/user-guide.md#environment-variables) for more info.|
| `envFrom`   | An array of references to `ConfigMap` or `Secret` resources containing environment variables. Keys from `ConfigMap` or `Secret` resources become environment variable names in your container. See [Environment variables](https://github.com/application-stacks/operator/blob/master/doc/user-guide.md#environment-variables) for more info.|
| `readinessProbe`   | A YAML object configuring the [Kubernetes readiness probe](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/#define-readiness-probes) that controls when the pod is ready to receive traffic. |
| `livenessProbe` | A YAML object configuring the [Kubernetes liveness probe](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/#define-a-liveness-http-request) that controls when Kubernetes needs to restart the pod.|
| `volumes` | A YAML object representing a [pod volume](https://kubernetes.io/docs/concepts/storage/volumes). |
| `volumeMounts` | A YAML object representing a [pod volumeMount](https://kubernetes.io/docs/concepts/storage/volumes/). |
| `storage.size` | A convenient field to set the size of the persisted storage. Can be overridden by the `storage.volumeClaimTemplate` property. |
| `storage.mountPath` | The directory inside the container where this persisted storage will be bound to. |
| `storage.volumeClaimTemplate` | A YAML object representing a [volumeClaimTemplate](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#components) component of a `StatefulSet`. |
| `monitoring.labels` | Labels to set on [ServiceMonitor](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#servicemonitor). |
| `monitoring.endpoints` | A YAML snippet representing an array of [Endpoint](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#endpoint) component from ServiceMonitor. |
| `createAppDefinition`   | A boolean to toggle the automatic configuration of `RuntimeComponent`'s Kubernetes resources to allow creation of an application definition by [kAppNav](https://kappnav.io/). The default value is `true`. See [Application Navigator](#kubernetes-application-navigator-kappnav-support) for more information. |
| `route.annotations` | Annotations to be added to the Route. |
| `route.host`   | Hostname to be used for the Route. |
| `route.path`   | Path to be used for Route. |
| `route.termination`   | TLS termination policy. Can be one of `edge`, `reencrypt` and `passthrough`. |
| `route.insecureEdgeTerminationPolicy`   | HTTP traffic policy with TLS enabled. Can be one of `Allow`, `Redirect` and `None`. |
| `route.certificate`  | A YAML object representing a [Certificate](https://cert-manager.io/docs/reference/api-docs/#cert-manager.io/v1alpha2.CertificateSpec). |
| `route.certificateSecretRef` | A name of a secret that already contains TLS key, certificate and CA to be used in the route. Also can contain destination CA certificate.  |

### Basic usage

To deploy a Docker image containing a runtime omponent to a Kubernetes environment you can use the following CR:

 ```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
```

The `applicationImage` value must be defined in the `RuntimeComponent` CR. On OpenShift, the operator tries to find an image stream name with the `applicationImage` value. The operator falls back to the registry lookup if it is not able to find any image stream that matches the value. If you want to distinguish an image stream called `my-company/my-app` (project: `my-company`, image stream name: `my-app`) from the Docker Hub `my-company/my-app` image, you can use the full image reference as `docker.io/my-company/my-app`.

To get information on the deployed CR, use either of the following:

```sh
oc get runtimecomponent my-app
oc get app my-app
```

### Image Streams

To deploy an image from an image stream, use the following CR:

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: my-namespace/my-image-stream:1.0
```

The previous example looks up the `1.0` tag from the `my-image-stream` image stream in the `my-namespace` project and populates the CR `.status.imageReference` field with the exact referenced image similar to the following one: `image-registry.openshift-image-registry.svc:5000/my-namespace/my-image-stream@sha256:8a829d579b114a9115c0a7172d089413c5d5dd6120665406aae0600f338654d8`. The operator watches the specified image stream and deploys new images as new ones are available for the specified tag.

To reference an image stream, the `applicationImage` parameter must follow the `<project name>/<image stream name>[:<tag>]` format. If `<project name>` or `<tag>` is not specified, the operator defaults the values to the namespace of the CR and the value of `latest`, respectively. For example, the `applicationImage: my-image-stream` configuration is the same as the `applicationImage: my-namespace/my-image-stream:latest` configuration.

The Operator tries to find an image stream name first with the `<project name>/<image stream name>[:<tag>]` format and falls back to the registry lookup if it is not able to find any image stream that matches the value. 

_This feature is only available if you are running on OKD or OpenShift._

### Service account

The operator can create a `ServiceAccount` resource when deploying a runtime omponent. If `serviceAccountName` is not specified in a CR, the operator creates a service account with the same name as the CR (e.g. `my-app`).

Users can also specify `serviceAccountName` when they want to create a service account manually.

If applications require specific permissions but still want the operator to create a `ServiceAccount`, users can still manually create a role binding to bind a role to the service account created by the operator. To learn more about Role-based access control (RBAC), see Kubernetes [documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).

### Labels

By default, the operator adds the following labels into all resources created
for a `RuntimeComponent` CR: 

| Label                          | Default                        | Description                                                                                                                                                  |
|--------------------------------|--------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `app.kubernetes.io/instance`   | `metadata.name`                | A unique name or identifier for this component. This cannot be modified.                                                                                                                 |
| `app.kubernetes.io/name`       | `metadata.name`                | A name that represents this component.                                                                                                               |
| `app.kubernetes.io/managed-by` | `application-stacks-operator` | The tool being used to manage this component.                                                                                                                |
| `app.kubernetes.io/component`  | `backend`                      | The type of component being created. See OpenShift [documentation](https://github.com/gorkem/app-labels/blob/master/labels-annotation-for-openshift.adoc#labels) for full list. |
| `app.kubernetes.io/part-of`    | `metadata.name`                | The name of the higher-level application this component is a part of. Configure this if the component is not a standalone application. |
| `app.kubernetes.io/version`    | `version`                      | The version of the component.                                                                                                                                |

You can set new labels in addition to the pre-existing ones or overwrite them,
excluding the `app.kubernetes.io/instance` label. To set labels, specify them in
your CR as key/value pairs.

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
  labels:
    my-label-key: my-label-value
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
```

_After the initial deployment of `RuntimeComponent`, any changes to its labels would be applied only when one of the parameters from `spec` is updated._

#### OpenShift Recommended Labels

When running in OpenShift, there are additional labels and annotations that are
standard on the platform. It is recommended that you overwrite our defaults
where applicable and add any labels from the list that are not set by default using the above instructions. See [documentation](https://github.com/gorkem/app-labels/blob/master/labels-annotation-for-openshift.adoc#labels) for a full list.

### Annotations

To add new annotations into all resources created for a `RuntimeComponent`, specify them in your CR as key/value pairs. Annotations specified in CR would override any annotations specified on a resource, except for the annotations set on `Service` using `service.annotations`.

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
  annotations:
    my-annotation-key: my-annotation-value
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
```

_After the initial deployment of `RuntimeComponent`, any changes to its annotations would be applied only when one of the parameters from `spec` is updated._

#### OpenShift Recommended Annotations

When running in OpenShift, there are additional annotations that are
standard on the platform. It is recommended that you overwrite our defaults
where applicable and add any annotations from the list that are not set by
default using the above instructions. See [documentation](https://github.com/gorkem/app-labels/blob/master/labels-annotation-for-openshift.adoc#labels) for a full list.

### Environment variables

You can set environment variables for your application container. To set
environment variables, specify `env` and/or `envFrom` fields in your CR. The
environment variables can come directly from key/value pairs, `ConfigMap`s or
`Secret`s. The environment variables set using the `env` or `envFrom` fields will
override any environment variables specified in the container image.

 ```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  env:
    - name: DB_NAME
      value: "database"
    - name: DB_PORT
      valueFrom:
        configMapKeyRef:
          name: db-config
          key: db-port
    - name: DB_USERNAME
      valueFrom:
        secretKeyRef:
          name: db-credential
          key: adminUsername
    - name: DB_PASSWORD
      valueFrom:
        secretKeyRef:
          name: db-credential
          key: adminPassword
  envFrom:
    - configMapRef:
        name: env-configmap
    - secretRef:
        name: env-secrets
```

Use `envFrom` to define all data in a `ConfigMap` or a `Secret` as environment variables in a container. Keys from `ConfigMap` or `Secret` resources become environment variable name in your container.

### High availability

Run multiple instances of your application for high availability using one of the following mechanisms: 
 - specify a static number of instances to run at all times using `replicas` parameter
 
    _OR_

 - configure auto-scaling to create (and delete) instances based on resource consumption using the `autoscaling` parameter.
      - Parameters `autoscaling.maxReplicas` and `resourceConstraints.requests.cpu` MUST be specified for auto-scaling.

### Persistence

Runtime Component Operator is capable of creating a `StatefulSet` and `PersistentVolumeClaim` for each pod if storage is specified in the `RuntimeComponent` CR.

Users also can provide mount points for their application. There are 2 ways to enable storage.

#### Basic storage

With the `RuntimeComponent` CR definition below the operator will create `PersistentVolumeClaim` called `pvc` with the size of `1Gi` and `ReadWriteOnce` access mode.

The operator will also create a volume mount for the `StatefulSet` mounting to `/data` folder. You can use `volumeMounts` field instead of `storage.mountPath` if you require to persist more then one folder.

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  storage:
    size: 1Gi
    mountPath: "/data"
```

#### Advanced storage

Runtime Component Operator allows users to provide entire `volumeClaimTemplate` for full control over automatically created `PersistentVolumeClaim`.

It is also possible to create multiple volume mount points for persistent volume using `volumeMounts` field as shown below. You can still use `storage.mountPath` if you require only a single mount point.

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  volumeMounts:
  - name: pvc
    mountPath: /data_1
    subPath: data_1
  - name: pvc
    mountPath: /data_2
    subPath: data_2
  storage:
    volumeClaimTemplate:
      metadata:
        name: pvc
      spec:
        accessModes:
        - "ReadWriteMany"
        storageClassName: 'glusterfs'
        resources:
          requests:
            storage: 1Gi
```

### Service binding

Runtime Component Operator can be used to help with service binding in a cluster. The operator creates a secret on behalf of the **provider** `RuntimeComponent` and injects the secret into pods of the **consumer** `RuntimeComponent` as either environment variable or mounted files. See [Runtime Component Operator Design for Service Binding](https://docs.google.com/document/d/1riOX0iTnBBJpTKAHcQShYVMlgkaTNKb4m8fY7W1GqMA/edit) for more information on the architecture. At this time, the only supported service binding type is `openapi`.

The provider lists information about the REST API it provides:

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-provider
  namespace: pro-namespace
spec:
  applicationImage: quay.io/my-repo/my-provider:1.0
  service:
    port: 3000
    provides:
      category: openapi
      context: /my-context
      auth:
        password:
          name: my-secret
          key: password
        username:
          name: my-secret
          key: username
---
kind: Secret
apiVersion: v1
metadata:
  name: my-secret
  namespace: pro-namespace
data:
  password: bW9vb29vb28=
  username: dGhlbGF1Z2hpbmdjb3c=
type: Opaque
```

And the consumer lists the services it is intending to consume:

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-consumer
  namespace: con-namespace
spec:
  applicationImage: quay.io/my-repo/my-consumer:1.0
  expose: true
  service:
    port: 9080
    consumes:
    - category: openapi
      name: my-provider
      namespace: pro-namespace
      mountPath: /sample
```

In the above example, the operator creates a secret named `pro-namespace-my-provider` and adds the following key-value pairs: `username`, `password`, `url`, `context`, `protocol` and `hostname`. The `url` value format is `<protocol>://<name>.<namespace>.svc.cluster.local:<port>/<context>`. Since the provider and the consumer are in two different namespaces, the operator copies the provider secret into consumer's namespace. The operator then mounts the provider secret into a directory with the pattern `<mountPath>/<namespace>/<service_name>` on application container within pods. In the above example, the secret will be serialized into `/sample/pro-namespace/my-provider`, which means we will have a file for each key, where the filename is the key and the content is the key's value.

If the `namespace` is not provided in the above example under `consumes`, then the operator mounts the provider secret into a directory with pattern `<mountPath>/<service_name>`.

If consumer's CR does not include `mountPath`, the secret will be bound to environment variables with the pattern `<NAMESPACE>_<SERVICE-NAME>_<KEY>`, and the value of that env var is the key’s value. Due to syntax restrictions for Kubernetes environment variables, the string representing the namespace and the string representing the service name will have to be normalized by turning any non-`[azAZ09]` characters to become an underscore `(_)` character.

### Monitoring

Runtime Component Operator can create a `ServiceMonitor` resource to integrate with `Prometheus Operator`.

_This feature does not support integration with Knative Service. Prometheus Operator is required to use ServiceMonitor._

#### Basic monitoring specification

At minimum, a label needs to be provided that Prometheus expects to be set on `ServiceMonitor` objects. In this case, it is `apps-prometheus`.

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  monitoring:
    labels:
       apps-prometheus: ''
```

#### Advanced monitoring specification

For advanced scenarios, it is possible to set many `ServicerMonitor` settings such as authentication secret using [Prometheus Endpoint](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#endpoint)

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  monitoring:
    labels:
       app-prometheus: ''
    endpoints:
    - interval: '30s'
      basicAuth:
        username:
          key: username
          name: metrics-secret
        password:
          key: password
          name: metrics-secret
      tlsConfig:
        insecureSkipVerify: true
```

### Knative support

Runtime Component Operator can deploy serverless applications with [Knative](https://knative.dev/docs/) on a Kubernetes cluster. To achieve this, the operator creates a [Knative `Service`](https://github.com/knative/serving/blob/master/docs/spec/spec.md#service) resource which manages the whole life cycle of a workload.

To create Knative service, set `createKnativeService` to `true`:

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  createKnativeService: true
```

By setting this parameter, the operator creates a Knative service in the cluster and populates the resource with applicable `RuntimeComponent` fields. Also, it ensures non-Knative resources including Kubernetes `Service`, `Route`, `Deployment` and etc. are deleted.

The CRD fields which are used to populate the Knative service resource include `applicationImage`, `serviceAccountName`, `livenessProbe`, `readinessProbe`, `service.Port`, `volumes`, `volumeMounts`, `env`, `envFrom`, `pullSecret` and `pullPolicy`.

For more details on how to configure Knative for tasks such as enabling HTTPS connections and setting up a custom domain, checkout [Knative Documentation](https://knative.dev/docs/serving/).

_Autoscaling related fields in `RuntimeComponent` are not used to configure Knative Pod Autoscaler (KPA). To learn more about how to configure KPA, see [Configuring the Autoscaler](https://knative.dev/docs/serving/configuring-the-autoscaler/)._

_This feature is only available if you have Knative installed on your cluster._

### Exposing service externally

#### Non-Knative deployment

To expose your application externally, set `expose` to `true`:

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  expose: true
```

By setting this parameter, the operator creates an unsecured route based on your application service. Setting this parameter is the same as running `oc expose service <service-name>`.

To create a secured HTTPS route, see the [Certificate Manager Integration](#certificate-manager-integration) section for more information.

_This feature is only available if you are running on OKD or OpenShift._

#### Knative deployment

To expose your application as a Knative service externally, set `expose` to `true`:

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: my-app
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  createKnativeService: true
  expose: true
```

When `expose` is **not** set to `true`, the Knative service is labeled with `serving.knative.dev/visibility=cluster-local` which makes the Knative route to only be available on the cluster-local network (and not on the public Internet). However, if `expose` is set `true`, the Knative route would be accessible externally.

To configure secure HTTPS connections for your Knative deployment, see [Configuring HTTPS with TLS certificates](https://knative.dev/docs/serving/using-a-tls-cert/) for more information.

### Kubernetes Application Navigator (kAppNav) support

By default, Runtime Component Operator configures the Kubernetes resources it generates to allow automatic creation of an application definition by [kAppNav](https://kappnav.io/), Kubernetes Application Navigator. You can easily view and manage the deployed resources that comprise your application using Application Navigator. You can disable auto-creation by setting `createAppDefinition` to `false`.

To join an existing application definition, disable auto-creation and set the label(s) needed to join the application on `RuntimeComponent` CR. See [Labels](#labels) section for more information.

_This feature is only available if you have kAppNav installed on your cluster. Auto creation of an application definition is not supported when Knative service is created_

### Certificate Manager Integration

Runtime Component Operator is enabled to take advantage of [cert-manager](https://cert-manager.io/) tool, if it is installed on the cluster.
This allows to automatically provision TLS certificates for pods as well as routes.

Cert-manager installation instruction can be found [here](https://cert-manager.io/docs/installation/)

When creating certificates via the RuntimeComponent CR the user can specify a particular issuer name and toggle the scopes between `ClusterIssuer` (cluster scoped) and `Issuer` (namespace scoped). If not specified, these values are retrieved from a ConfigMap called `application-stacks-operator`, with keys `defaultIssuer` (default value of `self-signed`) and `useClusterIssuer` (default value of `"true"`)

_This feature does not support integration with Knative Service._


#### Create an ClusterIssuer or Issuer

Self signed:

```yaml
apiVersion: cert-manager.io/v1alpha2
kind: ClusterIssuer
metadata:
  name: self-signed
spec:
  selfSigned: {}
```

Using custom CA key:

```yaml
apiVersion: cert-manager.io/v1alpha2
kind: ClusterIssuer
metadata:
  name: mycompany-ca
spec:
  ca:
    secretName: mycompany-ca-tls
```


#### Simple scenario (Pods certificate)

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: myapp
  namespace: test
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  ....
  service:
    port: 9080
    certificate: {}
```

In this scenario the operator will generate `Certificate` resource with common name of `myapp.test.svc` that can be used for service to service communication.

Once this certificate request is resolved by cert-manager the resulting secret `myapp-svc-tls` will be 
mounted into each pod inside `/etc/x509/certs` folder. Mounted files will be always up to date with a secret.

It will contain private key, certificate and CA certificate.
It is up to the application container to consume these artifacts, applying any needed transformation or modification.


#### Simple scenario (Route certificate)

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: myapp
  namespace: test
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  expose: true
  route:
    host: myapp.mycompany.com
    termination: reencrypt
    certificate: {}
```
In this scenario the operator will generate `Certificate` resource with common name of `myapp.mycompany.com` that will be injected into `Route` resource.

#### Advanced scenario

In this example we are overriding Issuer to be used for application.
Certificate will be generated for specific organization and duration. Extra properties can be added as well.

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: myapp
  namespace: test
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  expose: true
  route:
    host: myapp.mycompany.com
    termination: reencrypt
    certificate:
      duration: 8760h0m0s
      organization:
        - My Company
      issuerRef:
        name: myComanyIssuer
        kind: ClusterIssuer
```

#### Use existing certificates

It is possible to bring your own certificates to be used in a pod and the route.
In this case the cert-manager is not required.

```yaml
apiVersion: app.stacks/v1beta1
kind: RuntimeComponent
metadata:
  name: myapp
  namespace: test
spec:
  applicationImage: quay.io/my-repo/my-app:1.0
  expose: true
  route:
    host: myapp.mycompany.com
    termination: reencrypt
    certificateSecretRef: myapp-rt-tls
  service:
    port: 9443
```

Example of the manually provided route secret

```yaml
kind: Secret
apiVersion: v1
metadata:
  name: myapp-rt-tls
data:
  ca.crt: >-
    Certificate Authority public certificate...(base64)
  tls.crt: >-
    Route public certificate...(base64)
  tls.key: >-
    Route private key...(base64)
  destCA: >-
    Pod/Service certificate Certificate Authority (base64). Might be required when using reencrypt termination policy.
type: kubernetes.io/tls
```

### Troubleshooting

See the [troubleshooting guide](troubleshooting.md) for information on how to investigate and resolve deployment problems.
