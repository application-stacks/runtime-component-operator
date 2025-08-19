# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 1.4.4
OPERATOR_SDK_RELEASE_VERSION ?= v1.39.2

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "preview,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=preview,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="preview,fast,stable")
CHANNELS ?= v1.4
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
DEFAULT_CHANNEL ?= v1.4
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# openliberty.io/op-test-bundle:$VERSION and openliberty.io/op-test-catalog:$VERSION.
IMAGE_TAG_BASE ?= icr.io/appcafe/runtime-component-operator

# OPERATOR_IMAGE defines the docker.io namespace and part of the image name for remote images.
OPERATOR_IMAGE ?= icr.io/appcafe/runtime-component-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:daily

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
    BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Image URL to use all building/pushing image targets
IMG ?= icr.io/appcafe/runtime-component-operator:daily

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

PUBLISH_REGISTRY=docker.io

# Type of release. Can be "daily", "releases", or a release tag.
RELEASE_TARGET := $(or ${RELEASE_TARGET}, ${TRAVIS_TAG}, daily)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

CREATEDAT ?= AUTO
ifeq ($(CREATEDAT), AUTO)
CREATEDAT := $(shell date +%Y-%m-%dT%TZ)
endif

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1,generateEmbeddedObjectMeta=true"

# Produce files under internal/deploy/kustomize/daily with runtime-component namespace
KUSTOMIZE_NAMESPACE = runtime-component
KUSTOMIZE_IMG = icr.io/appcafe/runtime-component-operator:daily

# Use docker if available. Otherwise default to podman. 
# Override choice by setting CONTAINER_COMMAND
CHECK_DOCKER_RC=$(shell docker -v > /dev/null 2>&1; echo $$?)
ifneq (0, $(CHECK_DOCKER_RC))
CONTAINER_COMMAND ?= podman
# Setup parameters for TLS verify, default if unspecified is true
ifeq (false, $(TLS_VERIFY))
PODMAN_SKIP_TLS_VERIFY="--tls-verify=false"
SKIP_TLS_VERIFY=--skip-tls
else
TLS_VERIFY ?= true
PODMAN_SKIP_TLS_VERIFY="--tls-verify=true"
endif
else
CONTAINER_COMMAND ?= docker
endif

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Setup

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= 5.4.3
KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/kustomize/v${KUSTOMIZE_VERSION}/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	test -s $(LOCALBIN)/kustomize || GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@v$(KUSTOMIZE_VERSION)

CONTROLLER_TOOLS_VERSION ?= 0.16.5
.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_TOOLS_VERSION)

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.31.0
.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: setup
setup: ## Ensure Operator SDK is installed.
	./operators/scripts/installers/install-operator-sdk.sh ${OPERATOR_SDK_RELEASE_VERSION}

.PHONY: setup-go
setup-go: ## Ensure Go is installed.
	./operators/scripts/installers/install-go.sh ${GO_RELEASE_VERSION}

##@ Development

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..." 
.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: bundle
bundle: manifests setup kustomize ## Generate bundle manifests and metadata, then validate generated files.
	scripts/update-sample.sh

	sed -i.bak "s,OPERATOR_IMAGE,${IMG},g" config/manager/manager.yaml
	sed -i.bak "s,IMAGE,${IMG},g;s,CREATEDAT,${CREATEDAT},g" config/manifests/patches/csvAnnotations.yaml
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle $(BUNDLE_GEN_FLAGS)
	./scripts/csv_description_update.sh update_csv

	$(KUSTOMIZE) build config/kustomize/crd -o internal/deploy/kustomize/daily/base/runtime-component-crd.yaml
	cd config/kustomize/operator && $(KUSTOMIZE) edit set namespace $(KUSTOMIZE_NAMESPACE)
	$(KUSTOMIZE) build config/kustomize/operator -o internal/deploy/kustomize/daily/base/runtime-component-operator.yaml
	sed -i.bak "s,${IMG},${KUSTOMIZE_IMG},g;s,serviceAccountName: controller-manager,serviceAccountName: rco-controller-manager,g" internal/deploy/kustomize/daily/base/runtime-component-operator.yaml
	$(KUSTOMIZE) build config/kustomize/roles -o internal/deploy/kustomize/daily/base/runtime-component-roles.yaml
	
	$(KUSTOMIZE) build config/kubectl/crd -o internal/deploy/kubectl/runtime-component-crd.yaml
	$(KUSTOMIZE) build config/kubectl/operator -o internal/deploy/kubectl/runtime-component-operator.yaml
	$(KUSTOMIZE) build config/kubectl/rbac-watch-all -o internal/deploy/kubectl/runtime-component-rbac-watch-all.yaml
	$(KUSTOMIZE) build config/kubectl/rbac-watch-another -o internal/deploy/kubectl/runtime-component-rbac-watch-another.yaml

	$(KUSTOMIZE) build config/kustomize/watch-all -o internal/deploy/kustomize/daily/overlays/watch-all-namespaces/cluster-roles.yaml
	$(KUSTOMIZE) build config/kustomize/watch-another -o internal/deploy/kustomize/daily/overlays/watch-another-namespace/rco-watched-ns/watched-roles.yaml

	mv config/manager/manager.yaml.bak config/manager/manager.yaml
	mv config/manifests/patches/csvAnnotations.yaml.bak config/manifests/patches/csvAnnotations.yaml
	rm internal/deploy/kustomize/daily/base/runtime-component-operator.yaml.bak

	operator-sdk bundle validate ./bundle

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)"  go test ./... -coverprofile cover.out

.PHONY: unit-test
unit-test: ## Run unit tests
	go test -v -mod=vendor -tags=unit github.com/application-stacks/runtime-component-operator/...

.PHONY: test-cover
test-cover: test
	go tool cover -html=cover.out

.PHONY: run
run: manifests generate fmt vet ## Run a controller against the configured Kubernetes cluster in ~/.kube/config from your host.
	go run ./cmd/main.go

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl create -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager ./cmd/main.go

.PHONY: docker-login
docker-login:
	docker login -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}"

.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	ARCH=$(shell go env GOARCH) && $(CONTAINER_COMMAND) build -t ${IMG} . --build-arg GO_PLATFORM=$${ARCH}

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_COMMAND) push $(PODMAN_SKIP_TLS_VERIFY) ${IMG}

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	$(CONTAINER_COMMAND) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(CONTAINER_COMMAND) push $(PODMAN_SKIP_TLS_VERIFY) $(BUNDLE_IMG)

.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

kind-e2e-test:
	./operators/scripts/test/e2e-kind.sh --test-tag "${BUILD_NUMBER}"

build-manifest: setup-manifest
	./operators/scripts/build/build-manifest.sh --registry "${PUBLISH_REGISTRY}" --image "${OPERATOR_IMAGE}" --tag "${RELEASE_TARGET}"

build-operator-pipeline:
	./operators/scripts/build/build-operator.sh --registry "${REGISTRY}" --image "${PIPELINE_OPERATOR_IMAGE}" --tag "${RELEASE_TARGET}"

build-manifest-pipeline:
	./operators/scripts/build/build-manifest.sh --registry "${REGISTRY}" --image "${IMAGE}" --tag "${RELEASE_TARGET}"

build-bundle-pipeline:
	./operators/scripts/build/build-bundle.sh --prod-image "${PIPELINE_PRODUCTION_IMAGE}" --registry "${REGISTRY}" --image "${PIPELINE_OPERATOR_IMAGE}" --tag "${RELEASE_TARGET}"

build-catalog-pipeline:
	./operators/scripts/build/build-catalog.sh --prod-image "${OPERATOR_IMAGE}" --registry "${REGISTRY}" --image "${PIPELINE_OPERATOR_IMAGE}" --tag "${RELEASE_TARGET}" --version "${VERSION}"

test-pipeline-e2e:
	./operators/scripts/test/e2e-ocp.sh --cluster-url "${CLUSTER_URL}" --cluster-user "${CLUSTER_USER}" --cluster-token "${CLUSTER_TOKEN}" \
                                            --test-tag "${BUILD_NUMBER}" --install-mode "${INSTALL_MODE}" --channel "${DEFAULT_CHANNEL}" \
                                            --architecture "${ARCHITECTURE}" --digest "${DIGEST}" --version "${VERSION}"

bundle-build-podman:
	podman build -f bundle.Dockerfile -t "${BUNDLE_IMG}"

bundle-push-podman:
	podman push --format=docker "${BUNDLE_IMG}"

push-catalog: docker-login
	podman push --format=docker "${CATALOG_IMG}"

dev: 
	./scripts/dev.sh all -channel ${DEFAULT_CHANNEL}
