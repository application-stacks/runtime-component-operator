module github.com/application-stacks/runtime-component-operator

go 1.13

require (
	github.com/apex/log v1.9.0
	github.com/coreos/prometheus-operator v0.41.1
	github.com/go-logr/logr v0.2.1-0.20200730175230-ee2de8da5be6
	github.com/google/go-containerregistry v0.1.4 // indirect
	github.com/jetstack/cert-manager v1.0.3
	github.com/kubernetes-sigs/application v0.8.1 // indirect
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v3.9.1-0.20190924102528-32369d4db2ad+incompatible
	github.com/openshift/library-go v0.0.0-20201026125231-a28d3d1bad23
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.20.0 // indirect
	go.pedge.io/lion v0.0.0-20190619200210-304b2f426641 // indirect
	gopkg.in/inconshreveable/log15.v2 v2.0.0-20200109203555-b30bc20e4fd1 // indirect
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/serving v0.18.1
	sigs.k8s.io/application v0.8.1
	sigs.k8s.io/controller-runtime v0.6.2
)

replace (
	github.com/go-logr/zapr => github.com/go-logr/zapr v0.2.0
	k8s.io/client-go => k8s.io/client-go v0.19.3

	//To resolve license issue - https://github.com/operator-framework/operator-registry/issues/190
	github.com/otiai10/copy => github.com/otiai10/copy v1.0.2
	github.com/otiai10/mint => github.com/otiai10/mint v1.3.0
)

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm

replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190924102528-32369d4db2ad // Required until https://github.com/operator-framework/operator-lifecycle-manager/pull/1241 is resolved
