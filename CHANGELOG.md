<!--
This file includes chronologically ordered list of notable changes visible to end users for each version of the Runtime Component Operator. Keep a summary of the change and link to the pull request.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
-->

# Changelog
All notable changes to this project will be documented in this file.

## [Unreleased]

## [0.5.1

### Fixed

- Operator crash when creating Ingress without `spec.route` field defined
- Do not add `kubectl.kubernetes.io/last-applied-configuration` annotation to created resources to prevent uncessary pod restarts


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


[Unreleased]: https://github.com/application-stacks/runtime-component-operator/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.5.0
[0.4.2]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.4.2
[0.4.1]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.4.1
[0.4.0]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.4.0

