Runtime Component Operator allows you to deploy and manage any runtime components securely and easily on Red Hat OpenShift as well as other Kubernetes-based platforms in a consistent way. You can also perform day-2 operations using the operator.
## Documentation
See our user guide [here](https://ibm.biz/rco-docs). If you are **upgrading from versions 0.8.x or below**, make sure to review the documentation, prior to the upgrade, on [behavioural changes](https://ibm.biz/rco-upgrade-v1) that could impact your applications.
## Supported platforms
Kubernetes platform installed on one of the following platforms:
- Linux&reg; x86_64 (amd64)
- Linux&reg; on IBM&reg; Z (s390x)
- Linux&reg; on Power&reg; (ppc64le)
## Details
Key features provided by the operator:
### Integration with Certificate Managers
The [cert-manager APIs](https://cert-manager.io/) when available on the cluster will be used to generate certificates for the application. Otherwise, on Red Hat OpenShift, the operator will generate certificates using OpenShift's Certificate Manager. The operator will automatically provision TLS certificates for applications' pods and they are automatically refreshed when the certificates are updated. Optionally, you can bring your own (BYO) certificate authority (CA) or Issuer to generate certificates to secure your applications.
### Automatically restrict network communication
Network policies are created for each application by default to limit incoming traffic to pods in the same namespace that are part of the same application. Only the ports configured by the service are allowed. The network policy can be configured to allow either namespaces and/or pods with certain labels. On OpenShift, the operator automatically configures network policy to allow traffic from ingress, when the application is exposed, and from the monitoring stack.
### Exposing metrics to Prometheus
Expose the application's metrics via the Prometheus Operator. You can pick between a basic mode, where you simply specify the label that Prometheus is watching to scrape the metrics from the container, or you can specify the full `ServiceMonitor` spec embedded into the RuntimeComponent's `.spec.monitoring` field to control configurations such as poll interval and security credentials.
### Easily mount logs and transaction directories
Do you need to mount the logs and transaction data from your application to an external volume such as NFS (or any storage supported in your cluster)? Simply specify the volume size and the location to persist and the operator takes care of the rest. For example, add the following configuration into the RuntimeComponent's `.spec.storage` field :
``` size: 2Gi mountPath: "/logs" ```
### Service Binding
Your runtime components can expose services by a simple toggle. We take care of the heavy lifting such as creating Kubernetes secrets with information other services can use to bind. We also keep the bindable information synchronized, so your applications can dynamically reconnect to their required services without any intervention or interruption.
### Integration with Knative (OpenShift Serverless)
Deploy your serverless runtime component using a single toggle. The operator will convert all of its generated resources into [Knative](https://knative.dev) resources, allowing the application to automatically scale to 0 when it is idle.
### Application Lifecycle
You can deploy your application container by either pointing to a container image, or an OpenShift ImageStream. When using an ImageStream the operator will watch for any updates and will automatically re-deploy the new image.
### Custom RBAC
This Operator is capable of using a custom Kubernetes service account from the caller, allowing it to follow RBAC restrictions. By default, it creates a service account if one is not specified, which can also be bound with specific roles.
### Environment Configuration
You can configure a variety of artifacts with your deployment, such as labels, annotations, and environment variables from a ConfigMap, a Secret, or a value.
### Routing
Expose your application to external users via a single toggle to create a Route on OpenShift or an Ingress on other Kubernetes environments. Advanced configurations, such as for TLS, are also easily enabled. Renewed certificates are automatically made available to the runtime component.
### High Availability via Horizontal Pod Autoscaling
Run multiple instances of your application for high availability. Either specify a static number of replicas or easily configure horizontal auto-scaling to create (and delete) instances based on resource consumption.
### Persistence and advanced storage
Enable persistence for your application by specifying simple requirements: just tell us the size of the storage and where you would like it to be mounted and we will create and manage that storage for you. This toggles a StatefulSet resource instead of a Deployment resource, so your container can recover transactions and state upon a pod restart. We offer an advanced mode where you can specify a built-in PersistentVolumeClaim, allowing you to configure many details of the persistent volume, such as its storage class and access mode.
### Integration with OpenShift's Topology UI
We set the corresponding labels to support OpenShift's Developer Topology UI, which allows you to visualize your entire set of deployments and services and how they are connected.
