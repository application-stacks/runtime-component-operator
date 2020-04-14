<!--
This file includes chronologically ordered list of notable changes visible to end users for each version of the Runtime Component Operator. Keep a summary of the change and link to the pull request.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
-->

# Changelog
All notable changes to this project will be documented in this file.

## [Unreleased]

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


[Unreleased]: https://github.com/application-stacks/runtime-component-operator/compare/v0.4.2...HEAD
[0.4.2]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.4.2
[0.4.1]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.4.1
[0.4.0]: https://github.com/application-stacks/runtime-component-operator/releases/tag/v0.4.0

