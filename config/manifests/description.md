This advanced Operator is capable of deploying any runtime component image with consistent, production-grade QoS. It enables enterprise architects to govern the way their applications get deployed & managed in the cluster, while dramatically reducing the learning curve for developers to deploy into Kubernetes - allowing them to focus on writing the code!
Here are some key features:
#### Application Lifecyle
You can deploy your runtime component container by either pointing to a container image, or an OpenShift ImageStream. When using an ImageStream the Operator will watch for any updates and will re-deploy the modified image.
#### Custom RBAC
This Operator is capable of using a custom ServiceAccount from the caller, allowing it to follow RBAC restrictions. By default it creates a ServiceAccount if one is not specified, which can also be bound with specific roles.
#### Environment Configuration
You can configure a variety of artifacts with your deployment, such as: labels, annotations, and environment variables from a ConfigMap, a Secret or a value.
#### Routing
Expose your application to external users via a single toggle to create a Route on OpenShift or an Ingress on other Kubernetes environments. Advanced configuration, such as TLS settings, are also easily enabled.  Expiring Route certificates are re-issued.
#### High Availability via Horizontal Pod Autoscaling
Run multiple instances of your application for high availability. Either specify a static number of replicas or easily configure horizontal auto scaling to create (and delete) instances based on resource consumption.
#### Persistence and advanced storage
Enable persistence for your application by specifying simple requirements: just tell us the size of the storage and where you would like it to be mounted and We will create and manage that storage for you.
This toggles a StatefulSet resource instead of a Deployment resource, so your container can recover transactions and state upon a pod restart.
We offer an advanced mode where the user specifies a built-in PersistentVolumeClaim, allowing them to configure many details of the persistent volume, such as its storage class and access mode.
#### Service Binding
Your runtime components can expose services by a simple toggle. We take care of the heavy lifting such as creating kubernetes Secrets with information other services can use to bind. We also keep the bindable information synchronized, so your applications can dynamically reconnect to its required services without any intervention or interruption.
#### Exposing metrics to Prometheus
The Runtime Component Operator exposes the runtime container's metrics via the [Prometheus Operator](https://operatorhub.io/operator/prometheus).
Users can pick between a basic mode, where they simply specify the label that Prometheus is watching to scrape the metrics from the container, or they can specify the full `ServiceMonitor` spec embedded into the RuntimeComponent's `spec.monitoring` key controlling things like the poll internal and security credentials.
#### Easily mount logs and transaction directories
If you need to mount the logs and transaction data from your runtime component to an external volume such as NFS (or any storage supported in your cluster), simply add the following (customizing the folder location and size) to your RuntimeComponent CR:
``` storage: size: 2Gi mountPath: "/logs" ```
#### Integration with OpenShift Serverless
Deploy your serverless runtime component using a single toggle.  The Operator will convert all of its generated resources into [Knative](https://knative.dev) resources, allowing your pod to automatically scale to 0 when it is idle.
#### Integration with OpenShift's Topology UI
We set the corresponding labels to support OpenShift's Developer Topology UI, which allows you to visualize your entire set of deployments and how they are connected.
See our [**documentation**](https://github.com/application-stacks/runtime-component-operator/tree/main/doc/) for more information.
