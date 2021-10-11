#!/bin/bash

readonly usage="Usage: e2e.sh --cluster-url <url> --cluster-token <token> --registry-name <name> --registry-namespace <namespace>"
readonly SERVICE_ACCOUNT="travis-tests"

# setup_env: Download oc cli, log into our persistent cluster, and create a test project
setup_env() {
    echo "****** Installing OC CLI..."
    # Install kubectl and oc
    curl -L https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable-4.3/openshift-client-linux.tar.gz | tar xvz
    sudo mv oc kubectl /usr/local/bin/

    # Start a cluster and login
    echo "****** Logging into remote cluster ..."
    oc login "${OC_URL}" --token="${OC_TOKEN}"

    # Set variables for rest of script to use
    readonly DEFAULT_REGISTRY=$(oc get route "${REGISTRY_NAME}" -o jsonpath="{ .spec.host }" -n "${REGISTRY_NAMESPACE}")
    readonly TEST_NAMESPACE="runtime-operator-test-${TRAVIS_BUILD_NUMBER}"
    readonly BUILD_IMAGE=${DEFAULT_REGISTRY}/${TEST_NAMESPACE}/runtime-operator

    echo "****** Creating test namespace: ${TEST_NAMESPACE}"
    oc new-project "${TEST_NAMESPACE}"

    echo "****** Logging into private registry..."
    oc sa get-token "${SERVICE_ACCOUNT}" -n default | docker login -u unused --password-stdin "${DEFAULT_REGISTRY}" || {
      echo "Failed to log into docker registry as ${SERVICE_ACCOUNT}, exiting..."
      exit 1
    }

    echo "****** Creating pull secret using Docker config..."
    oc create secret generic regcred --from-file=.dockerconfigjson="${HOME}/.docker/config.json" --type=kubernetes.io/dockerconfigjson
}

## cleanup : Delete generated resources that are not bound to a test TEST_NAMESPACE.
cleanup_env() {
  oc delete project "${TEST_NAMESPACE}"
  # Remove image related resources after the test has finished
  oc delete imagestream "runtime-operator:${TRAVIS_BUILD_NUMBER}" -n openshift
}

main() {
    parse_args "$@"
    echo "****** Setting up test environment..."
    setup_env

    echo "****** Building image"
    operator-sdk build "${BUILD_IMAGE}"
    echo "****** Pushing image into registry..."
    docker push "${BUILD_IMAGE}" || {
        echo "Failed to push ref: ${BUILD_IMAGE} to docker registry, exiting..."
        exit 1
    }

    echo "****** Building bundle..."
    readonly BUNDLE_IMAGE="${DEFAULT_REGISTRY}/${TEST_NAMESPACE}/rco-bundle"
    operator-sdk run bundle --install-mode OwnNamespace --pull-secret-name regcred "${BUNDLE_IMAGE}"

    echo "****** Starting e2e tests..."
    readonly test_location="github.com/application-stacks/runtime-component-operator/test/e2e"
    operator-sdk test local "${test_location}" --debug --verbose  --go-test-flags "-timeout 35m" --image "${BUILD_IMAGE}"

    result=$?
    echo "****** Cleaning up test environment..."
    cleanup_env

    return $result
}

parse_args() {
    while [ $# -gt 0 ]; do
    case "$1" in
    --cluster-url)
      shift
      readonly OC_URL="${1}"
      ;;
    --cluster-token)
      shift
      readonly OC_TOKEN="${1}"
      ;;
    --registry-name)
      shift
      readonly REGISTRY_NAME="${1}"
      ;;
    --registry-namespace)
      shift
      readonly REGISTRY_NAMESPACE="${1}"
      ;;
    *)
      echo "Error: Invalid argument - $1"
      echo "$usage"
      exit 1
      ;;
    esac
    shift
  done
}


main "$@"
