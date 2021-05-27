module github.com/application-stacks/runtime-component-operator

go 1.15

require (
	github.com/apex/log v1.9.0
	github.com/coreos/prometheus-operator v0.41.1
	github.com/go-logr/logr v0.3.0
	github.com/google/go-containerregistry v0.1.4 // indirect
	github.com/jetstack/cert-manager v1.0.3
	github.com/openshift/api v0.0.0-20201019163320-c6a5ec25f267
	github.com/openshift/library-go v0.0.0-20201026125231-a28d3d1bad23
	github.com/pkg/errors v0.9.1
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/serving v0.18.1
	sigs.k8s.io/controller-runtime v0.7.2
)

replace (
	github.com/go-logr/zapr => github.com/go-logr/zapr v0.2.0

	//To resolve license issue - https://github.com/operator-framework/operator-registry/issues/190
	github.com/otiai10/copy => github.com/otiai10/copy v1.0.2
	github.com/otiai10/mint => github.com/otiai10/mint v1.3.0
	k8s.io/client-go => k8s.io/client-go v0.19.2
)

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm