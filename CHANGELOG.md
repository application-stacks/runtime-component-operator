<!--
This file includes chronologically ordered list of notable changes visible to end users for each version of the Runtime Component Operator. Keep a summary of the change and link to the pull request.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
-->

# Changelog
All notable changes to this project will be documented in this file.

## [0.8.2]

### Fixed

- Check for secrets only if service account pull secrets exist. ([#377](https://github.com/application-stacks/runtime-component-operator/pull/377))
- Increase memory limit for manager deployment ([#359](https://github.com/application-stacks/runtime-component-operator/issues/359))

### Added

- If TLS termination is set to Passthrough, also set InsecureEdgeTerminationPolicy as Redirect is only possible option([#293](https://github.com/application-stacks/runtime-component-operator/pull/293))

## [0.8.1]

### Fixed

- Fix to allow multiple /'s in application image ([#333](https://github.com/application-stacks/runtime-component-operator/pull/333))

### Added

- Added p and z-linux images to Docker Hub to support install options with kubectl or kustomize. OLM install option only supports amd64. ([#332](https://github.com/application-stacks/runtime-component-operator/pull/332))

## [0.8.0]

**Breaking changes:** API version of the custom resources (CRs) `RuntimeComponent` and `RuntimeOperation` have changed. Custom resources with `apiVersion: app.stacks/v1beta1` are not handled by Runtime Component Operator versions 0.8.0 and above. You must delete existing custom resources with `apiVersion: app.stacks/v1beta1` and create new custom resources with `apiVersion: rc.app.stacks/v1beta2`.

See the [new user guide](https://github.com/application-stacks/runtime-component-operator/blob/main/doc/user-guide-v1beta2.adoc) for more information on the changes to the fields listed below.

### Removed

- Following fields have been removed:
  - .spec.architecture
  - .spec.bindings.*
  - .spec.createAppDefinition
  - .spec.route.certificate
  - .spec.service.certificate
  - .spec.service.consumes.*
  - .spec.service.provides.*

### Changed

- Following fields have been renamed or moved:
  - .spec.livenessProbe --> .spec.probes.liveness
  - .spec.readinessProbe --> .spec.probes.readiness
  - .spec.resourceConstraints.* --> .spec.resources.*
  - .spec.storage --> .spec.statefulSet.storage
  - .spec.version --> .spec.applicationVersion

### Added

- Following fields were added: 
  - .spec.deployment.*
  - .spec.probes.startup
  - .spec.route.pathType
  - .spec.service.bindable
  - .spec.statefulSet.*

## [0.7.1]

### Fixed

- Updated Image Stream lookup logic to query for Image Stream Tags ([#156](https://github.com/application-stacks/runtime-component-operator/pull/156))

### Added

- Added support for Day-2 operations ([#157](https://github.com/application-stacks/runtime-component-operator/pull/157))

## [0.7.0]

### Added

- Added support to use application as a backing service ([#148](https://github.com/application-stacks/runtime-component-operator/pull/148))
- Added examples for specifying affinity ([#133](https://github.com/application-stacks/runtime-component-operator/pull/133))

## [0.6.0]

### Added

- Added support for embedding service bindings into a custom resource ([#111](https://github.com/application-stacks/runtime-component-operator/pull/111))
- Added support for affinity settings such as _nodeAffinity, podAffinity_ and _podAntiAffinity_ ([#116](https://github.com/application-stacks/runtime-component-operator/pull/116))

### Changed

- **Breaking change:** In order for the operator to detect Service Binding custom resources automatically ("auto-detect" functionality), the name of the binding resource must follow the `<CR_NAME>-binding` format (e.g. `my-app-binding`) ([#111](https://github.com/application-stacks/runtime-component-operator/pull/111))

### Fixed

- Some monitoring configurations such as `bearerTokenSecret` are not propagated to the created `ServiceMonitor` ([#157](https://github.com/OpenLiberty/open-liberty-operator/issues/157), [#116](https://github.com/application-stacks/runtime-component-operator/pull/116))

## [0.5.1]

### Fixed

- Operator crash when Ingress is created without defining spec.route field ([#94](https://github.com/application-stacks/runtime-component-operator/pull/94))
- Unnecessary pod restarts due to adding kubectl.kubernetes.io/last-applied-configuration to resources created by the operator ([#94](https://github.com/application-stacks/runtime-component-operator/pull/94))


## [0.5.0]

### Added

- Added Ingress (vanilla) support ([#79](https://github.com/application-stacks/runtime-component-operator/pull/79))
- Added support for external service bindings ([#76](https://github.com/application-stacks/runtime-component-operator/pull/76))
- Added additional service ports support ([#80](https://github.com/application-stacks/runtime-component-operator/pull/80))
- Added support to specify NodePort on service ([#60](https://github.com/application-stacks/runtime-component-operator/pull/60))

## [0.4.2]

### Fixed

- Auto-scaling (HPA) not working as expected ([#72](https://github.com/application-stacks/runtime-component-operator/pull/72))
- Operator crashes on some cluster due to optional CRDs (Knative Service, ServiceMonitor) not being present ([#67](https://github.com/application-stacks/runtime-component-operator/pull/67))
- Update the predicates for watching StatefulSet and Deployment sub-resource to check for generation to minimize number of reconciles ([#75](https://github.com/application-stacks/runtime-component-operator/pull/75))

## [0.4.1]

### Added

- Added optional targetPort field to service in the CRD ([#51](https://github.com/application-stacks/runtime-component-operator/pull/51))
- Added OpenShift specific annotations ([#54](https://github.com/application-stacks/runtime-component-operator/pull/54))
- Set port name for Knative service if specified ([#55](https://github.com/application-stacks/runtime-component-operator/pull/55))

## [0.4.0]

The initial release of the Runtime Component Operator 🎉


[Unreleased]: https://github.com/application-stacks/runtime-component-operator/compare/v0.8.1...HEAD
[0.8.2]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.8.2
[0.8.1]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.8.1
[0.8.0]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.8.0
[0.7.1]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.7.1
[0.7.0]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.7.0
[0.6.0]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.6.0
[0.5.1]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.5.1
[0.5.0]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.5.0
[0.4.2]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.4.2
[0.4.1]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.4.1
[0.4.0]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.4.0

