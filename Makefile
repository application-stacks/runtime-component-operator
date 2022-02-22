OPERATOR_SDK_RELEASE_VERSION ?= v1.6.4

# Current Operator version
VERSION ?= 0.8.1

OPERATOR_IMAGE ?= applicationstacks/operator
PIPELINE_REGISTRY ?= cp.stg.icr.io
PIPELINE_REGISTRY_NAMESPACE ?= cp
PIPELINE_OPERATOR_IMAGE ?= ${PIPELINE_REGISTRY_NAMESPACE}/rco-operator

# Type of release. Can be "daily", "releases", or a release tag.
RELEASE_TARGET := $(or ${RELEASE_TARGET}, ${TRAVIS_TAG}, daily)

PUBLISH_REGISTRY=docker.io

# Default bundle image tag
BUNDLE_IMG ?= applicationstacks/operator:bundle-daily

# Options for 'bundle-build'
CHANNELS ?= beta2
DEFAULT_CHANNEL ?= beta2
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

CREATEDAT ?= AUTO

ifeq ($(CREATEDAT), AUTO)
CREATEDAT := $(shell date +%y-%m-%dT%TZ)
endif

# Default catalog image tag
CATALOG_IMG ?= applicationstacks/operator:catalog-daily

# Image URL to use all building/pushing image targets
IMG ?= applicationstacks/operator:daily

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1,trivialVersions=true,preserveUnknownFields=false,generateEmbeddedObjectMeta=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
ENVTEST_ASSETS_DIR = $(shell pwd)/testbin
test: generate fmt vet manifests
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.6.3/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -


# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Undeploy controller in the configured Kubernetes cluster in ~/.kube/config
undeploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	rm -f config/manifests/patches/csvAnnotations.yaml.bak
# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests setup
	sed -i.bak "s,IMAGE,${IMG},g;s,CREATEDAT,${CREATEDAT},g" config/manifests/patches/csvAnnotations.yaml
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	mv config/manifests/patches/csvAnnotations.yaml.bak config/manifests/patches/csvAnnotations.yaml
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: docker-login
	docker push "${PUBLISH_REGISTRY}/${BUNDLE_IMG}"

setup: ## Ensure Operator SDK is installed
	./scripts/installers/install-operator-sdk.sh ${OPERATOR_SDK_RELEASE_VERSION}

unit-test: ## Run unit tests
	go test -v -mod=vendor -tags=unit github.com/application-stacks/runtime-component-operator/...

docker-login:
	docker login -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}" 
	
build-manifest: setup-manifest
	./scripts/build-manifest.sh --image "${PUBLISH_REGISTRY}/${OPERATOR_IMAGE}" --target "${RELEASE_TARGET}"

build-pipeline-manifest: setup-manifest
	./scripts/build-manifest.sh -u "${PIPELINE_USERNAME}" -p "${PIPELINE_PASSWORD}" --registry "${PIPELINE_REGISTRY}" --image "${PIPELINE_REGISTRY}/${PIPELINE_OPERATOR_IMAGE}"	--target "${RELEASE_TARGET}"

setup-manifest:
	./scripts/installers/install-manifest-tool.sh

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

install-podman:
	./scripts/installers/install-podman.sh

install-opm:
	./scripts/installers/install-opm.sh

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