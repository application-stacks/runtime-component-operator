# Current Operator version.
VERSION ?= 0.8.1
OPERATOR_SDK_RELEASE_VERSION ?= v1.24.0

# CHANNELS define the bundle channels used in the bundle.
CHANNELS ?= beta2
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
DEFAULT_CHANNEL ?= beta2
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# OPERATOR_IMAGE defines the docker.io namespace and part of the image name for remote images.
OPERATOR_IMAGE ?= applicationstacks/operator

# BUNDLE_IMG defines the image:tag used for the bundle.
BUNDLE_IMG ?= applicationstacks/operator:bundle-daily

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
    BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Image URL to use all building/pushing image targets.
IMG ?= applicationstacks/operator:daily

# The image tag given to the resulting catalog image.
CATALOG_IMG ?= applicationstacks/operator:catalog-daily

PUBLISH_REGISTRY=docker.io
PIPELINE_REGISTRY ?= cp.stg.icr.io
PIPELINE_REGISTRY_NAMESPACE ?= cp
PIPELINE_OPERATOR_IMAGE ?= ${PIPELINE_REGISTRY_NAMESPACE}/rco-operator

# Type of release. Can be "daily", "releases", or a release tag.
RELEASE_TARGET := $(or ${RELEASE_TARGET}, ${TRAVIS_TAG}, daily)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set).
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

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion).
CRD_OPTIONS ?= "crd:crdVersions=v1,generateEmbeddedObjectMeta=true"

# Produce files under deploy/kustomize/daily with default namespace.
KUSTOMIZE_NAMESPACE = default

.PHONY: all
all: manager

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Setup

## Location to install dependencies to.
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# find or download controller-gen
# download controller-gen if necessary
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2

KUSTOMIZE ?= $(LOCALBIN)/kustomize
KUSTOMIZE_VERSION ?= 3.8.7
KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/release-kustomize-v3.8/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	test -s $(LOCALBIN)/kustomize || curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s $(KUSTOMIZE_VERSION) $(LOCALBIN)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.15.1/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

.PHONY: setup
setup: ## Install Operator SDK if necessary.
	./scripts/installers/install-operator-sdk.sh ${OPERATOR_SDK_RELEASE_VERSION}

.PHONY: setup-manifest
setup-manifest: ## Install manifest tool.
	./scripts/installers/install-manifest-tool.sh

# Install Podman.
install-podman:
	./scripts/installers/install-podman.sh

# Install OPM.
install-opm:
	./scripts/installers/install-opm.sh

##@ Development

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	rm -f config/manifests/patches/csvAnnotations.yaml.bak

.PHONY: bundle
bundle: manifests setup kustomize ## Generate bundle manifests and metadata, then validate generated files.
	sed -i.bak "s,IMAGE,${IMG},g;s,CREATEDAT,${CREATEDAT},g" config/manifests/patches/csvAnnotations.yaml
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle $(BUNDLE_GEN_FLAGS)
	
	$(KUSTOMIZE) build config/kustomize/crd -o deploy/kustomize/daily/base/runtime-component-crd.yaml
	cd config/kustomize/operator && $(KUSTOMIZE) edit set namespace $(KUSTOMIZE_NAMESPACE)
	$(KUSTOMIZE) build config/kustomize/operator -o deploy/kustomize/daily/base/runtime-component-operator.yaml
	
	mv config/manifests/patches/csvAnnotations.yaml.bak config/manifests/patches/csvAnnotations.yaml
	operator-sdk bundle validate ./bundle

.PHONY: kustomize-build
kustomize-build: manifests kustomize ## Generate build controller, and roles & role bindings under deploy/kustomize directory.
	cd deploy/kustomize/daily/base && $(KUSTOMIZE) edit set namespace ${KUSTOMIZE_NAMESPACE}

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

ENVTEST_ASSETS_DIR = $(shell pwd)/testbin
.PHONY: test
test: generate fmt vet manifests ## Run tests.
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.6.3/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

.PHONY: unit-test
unit-test: ## Run unit tests.
	go test -v -mod=vendor -tags=unit github.com/application-stacks/runtime-component-operator/...

.PHONY: run
run: generate fmt vet manifests ## Run a controller against the configured Kubernetes cluster in ~/.kube/config from your host.
	go run ./main.go

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: manifests kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -


##@ Build

.PHONY: manager
manager: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: docker-login
docker-login: ## Log in to a Docker registry.
	docker login -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}" 

.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: docker-login ## Push the bundle image.
	docker push "${PUBLISH_REGISTRY}/${BUNDLE_IMG}"

build-manifest: setup-manifest
	./scripts/build-manifest.sh --image "${PUBLISH_REGISTRY}/${OPERATOR_IMAGE}" --target "${RELEASE_TARGET}"

build-pipeline-manifest: setup-manifest
	./scripts/build-manifest.sh -u "${PIPELINE_USERNAME}" -p "${PIPELINE_PASSWORD}" --registry "${PIPELINE_REGISTRY}" --image "${PIPELINE_REGISTRY}/${PIPELINE_OPERATOR_IMAGE}"	--target "${RELEASE_TARGET}"

bundle-pipeline:
	./scripts/bundle-release.sh -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}" --registry "${PIPELINE_REGISTRY}" --prod-image "${PIPELINE_PRODUCTION_IMAGE}" --image "${PIPELINE_REGISTRY}/${PIPELINE_OPERATOR_IMAGE}" --release "${RELEASE_TARGET}"

catalog-pipeline-build: opm ## Build a catalog image.
	./scripts/catalog-build.sh -n "v${OPM_VERSION}" -b "${REDHAT_BASE_IMAGE}" -o "${OPM}" --container-tool "docker" -i "${PIPELINE_REGISTRY}/${PIPELINE_OPERATOR_IMAGE}-bundle:${RELEASE_TARGET}" -p "${PIPELINE_PRODUCTION_IMAGE}-bundle" -a "${PIPELINE_REGISTRY}/${PIPELINE_OPERATOR_IMAGE}-catalog:${RELEASE_TARGET}" -t "${PWD}/operator-build" -v "${VERSION}"

catalog-pipeline-push: ## Push a catalog image.
	$(MAKE) docker-push IMG="${PIPELINE_REGISTRY}/${PIPELINE_OPERATOR_IMAGE}-catalog:${RELEASE_TARGET}"

minikube-test-e2e:
	./scripts/e2e-minikube.sh --test-tag "${TRAVIS_BUILD_NUMBER}"

test-e2e:
	./scripts/e2e-release.sh --registry-name default-route --registry-namespace openshift-image-registry \
                     --test-tag "${TRAVIS_BUILD_NUMBER}" --target "${RELEASE_TARGET}"

test-pipeline-e2e:
	./scripts/pipeline/fyre-e2e.sh -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}" \
                     --cluster-url "${CLUSTER_URL}" --cluster-token "${CLUSTER_TOKEN}" \
                     --registry-name "${PIPELINE_REGISTRY}" --registry-namespace "${PIPELINE_REGISTRY_NAMESPACE}" \
					 --registry-user "${PIPELINE_USERNAME}" --registry-password "${PIPELINE_PASSWORD}" \
                     --test-tag "${TRAVIS_BUILD_NUMBER}" --release "${RELEASE_TARGET}"					 

build-releases:
	./scripts/build-releases.sh --image "${PUBLISH_REGISTRY}/${OPERATOR_IMAGE}" --target "${RELEASE_TARGET}"

build-pipeline-releases:
	./scripts/build-releases.sh -u "${PIPELINE_USERNAME}" -p "${PIPELINE_PASSWORD}" --registry "${PIPELINE_REGISTRY}" --image "${PIPELINE_REGISTRY}/${PIPELINE_OPERATOR_IMAGE}"	--target "${RELEASE_TARGET}"

bundle-releases:
	./scripts/bundle-releases.sh --image "${PUBLISH_REGISTRY}/${OPERATOR_IMAGE}" --target "${RELEASE_TARGET}"

bundle-pipeline-releases:	
	./scripts/bundle-releases.sh -u "${PIPELINE_USERNAME}" -p "${PIPELINE_PASSWORD}" --registry "${PIPELINE_REGISTRY}" --image "${PIPELINE_REGISTRY}/${PIPELINE_OPERATOR_IMAGE}" --target "${RELEASE_TARGET}"

bundle-build-podman:
	podman build -f bundle.Dockerfile -t "${BUNDLE_IMG}"

bundle-push-podman:
	podman push --format=docker "${BUNDLE_IMG}"

build-catalog:
	opm index add --bundles "${BUNDLE_IMG}" --tag "${CATALOG_IMG}"

push-catalog: docker-login
	podman push --format=docker "${CATALOG_IMG}"

push-pipeline-catalog: 
	podman push --format=docker "${CATALOG_IMG}"
