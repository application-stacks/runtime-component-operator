#!/bin/bash

readonly usage="Usage: ocp-cluster-e2e.sh -u <docker-username> -p <docker-password> --cluster-url <url> --cluster-token <token> --registry-name <name> --registry-namespace <namespace> --registry-user <user> --registry-password <password> --release <daily|release-tag> --test-tag <test-id>"
readonly OC_CLIENT_VERSION="4.6.0"
readonly CONTROLLER_MANAGER_NAME="rco-controller-manager"

# setup_env: Download oc cli, log into our persistent cluster, and create a test project
setup_env() {
    echo "****** Installing OC CLI..."
    # Install kubectl and oc
    curl -L https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_CLIENT_VERSION}/openshift-client-linux.tar.gz | tar xvz
    sudo mv oc kubectl /usr/local/bin/

    # Start a cluster and login
    echo "****** Logging into remote cluster..."
    oc login "${CLUSTER_URL}" -u "${CLUSTER_USER:-kubeadmin}" -p "${CLUSTER_TOKEN}" --insecure-skip-tls-verify=true

    # Set variables for rest of script to use
    readonly TEST_NAMESPACE="runtime-operator-test-${TEST_TAG}"

    echo "****** Creating test namespace: ${TEST_NAMESPACE} for release ${RELEASE}"
    oc new-project "${TEST_NAMESPACE}" || oc project "${TEST_NAMESPACE}"

    ## Create service account for Kuttl tests
    oc apply -f config/rbac/kuttl-rbac.yaml
}

## cleanup_env : Delete generated resources that are not bound to a test TEST_NAMESPACE.
cleanup_env() {
  oc delete project "${TEST_NAMESPACE}"
}

## trap_cleanup : Call cleanup_env and exit. For use by a trap to detect if the script is exited at any point.
trap_cleanup() {
  last_status=$?
  if [[ $last_status != 0 ]]; then
    cleanup_env
  fi
  exit $last_status
}

#push_images() {
#    echo "****** Logging into private registry..."
#    oc sa get-token "${SERVICE_ACCOUNT}" -n default | docker login -u unused --password-stdin "${DEFAULT_REGISTRY}" || {
#        echo "Failed to log into docker registry as ${SERVICE_ACCOUNT}, exiting..."
#        exit 1
#    }

#    echo "****** Creating pull secret using Docker config..."
#    oc create secret generic regcred --from-file=.dockerconfigjson="${HOME}/.docker/config.json" --type=kubernetes.io/dockerconfigjson

#    docker push "${BUILD_IMAGE}" || {
#        echo "Failed to push ref: ${BUILD_IMAGE} to docker registry, exiting..."
#        exit 1
#    }

#    docker push "${BUNDLE_IMAGE}" || {
#        echo "Failed to push ref: ${BUNDLE_IMAGE} to docker registry, exiting..."
#        exit 1
#    }
#}

main() {
    parse_args "$@"

    if [[ -z "${RELEASE}" ]]; then
        echo "****** Missing release, see usage"
    fi

    if [[ -z "${DOCKER_USERNAME}" || -z "${DOCKER_PASSWORD}" ]]; then
        echo "****** Missing docker authentication information, see usage"
        echo "${usage}"
        exit 1
    fi

    if [[ -z "${CLUSTER_URL}" ]] || [[ -z "${CLUSTER_TOKEN}" ]]; then
        echo "****** Missing OCP URL or token, see usage"
        echo "${usage}"
        exit 1
    fi

    if [[ -z "${REGISTRY_NAME}" ]]; then
        echo "****** Missing OCP registry name, see usage"
        echo "${usage}"
        exit 1
    fi

    if [[ -z "${REGISTRY_IMAGE}" ]]; then
        echo "****** Missing REGISTRY_IMAGE definition, see usage"
        echo "${usage}"
        exit 1
    fi

    if [[ -z "${REGISTRY_USER}" ]] || [[ -z "${REGISTRY_PASSWORD}" ]]; then
        echo "****** Missing registry authentication information, see usage"
        echo "${usage}"
        exit 1
    fi

    if [[ -z "${TEST_TAG}" ]]; then
        echo "****** Missing test tag, see usage"
        echo "${usage}"
        exit 1
    fi

    if [[ -z "${CATALOG_IMAGE}" ]]; then
        echo "****** Missing catalog image, see usage"
        echo "${usage}"
        exit 1
    fi

    if [[ -z "${CHANNEL}" ]]; then
        echo "****** Missing channel, see usage"
        echo "${usage}"
        exit 1
    fi

    echo "****** Setting up test environment..."
    setup_env

    if [[ -z "${DEBUG_FAILURE}" ]]; then
        trap trap_cleanup EXIT
    else
        echo "#####################################################################################"
        echo "WARNING: --debug-failure is set. If e2e tests fail, any created resources will remain"
        echo "on the cluster for debugging/troubleshooting. YOU MUST DELETE THESE RESOURCES when"
        echo "you're done, or else they will cause future tests to fail. To cleanup manually, just"
        echo "delete the namespace \"${TEST_NAMESPACE}\": oc delete project \"${TEST_NAMESPACE}\" "
        echo "#####################################################################################"
    fi

    # login to docker to avoid rate limiting during build
    echo "${DOCKER_PASSWORD}" | docker login -u "${DOCKER_USERNAME}" --password-stdin

    trap "rm -f /tmp/pull-secret-*.yaml" EXIT

    echo "****** Logging into private registry..."
    echo "${REGISTRY_PASSWORD}" | docker login ${REGISTRY_NAME} -u "${REGISTRY_USER}" --password-stdin

    echo "****** Creating pull secret..."
    oc create secret docker-registry regcred --docker-server=${REGISTRY_NAME} "--docker-username=${REGISTRY_USER}" "--docker-password=${REGISTRY_PASSWORD}" --docker-email=unused 

    oc get secret/regcred -o jsonpath='{.data.\.dockerconfigjson}' | base64 --decode > /tmp/pull-secret-new.yaml
    oc get secret/pull-secret -n openshift-config -o jsonpath='{.data.\.dockerconfigjson}' | base64 --decode > /tmp/pull-secret-global.yaml

    jq -s '.[1] * .[0]' /tmp/pull-secret-new.yaml /tmp/pull-secret-global.yaml > /tmp/pull-secret-merged.yaml

    echo "Updating global pull secret"
    oc set data secret/pull-secret -n openshift-config --from-file=.dockerconfigjson=/tmp/pull-secret-merged.yaml

    echo "****** Installing operator from catalog: ${CATALOG_IMAGE}"
    install_operator

    # Wait for operator deployment to be ready
    while [[ $(oc get deploy "${CONTROLLER_MANAGER_NAME}" -o jsonpath='{ .status.readyReplicas }') -ne "1" ]]; do
        echo "****** Waiting for ${CONTROLLER_MANAGER_NAME} to be ready..."
        sleep 10
    done

    echo "****** ${CONTROLLER_MANAGER_NAME} deployment is ready..."

    echo "****** Starting scorecard tests..."
    operator-sdk scorecard --verbose --kubeconfig  ${HOME}/.kube/config --selector=suite=kuttlsuite --namespace="${TEST_NAMESPACE}" --service-account="scorecard-kuttl" --wait-time 30m ./bundle || {
        echo "****** Scorecard tests failed..."
        exit 1
    }
    result=$?

    echo "****** Cleaning up test environment..."
    cleanup_env

    return $result
}

install_operator() {
    # Apply the catalog
    echo "****** Applying the catalog source..."
    cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: rco-catalog
  namespace: $TEST_NAMESPACE
spec:
  sourceType: grpc
  image: $CATALOG_IMAGE
  displayName: Runtime Component Operator Catalog
  publisher: IBM
EOF

    echo "****** Applying the operator group..."
    cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: runtime-component-operator-group
  namespace: $TEST_NAMESPACE
spec:
  targetNamespaces:
    - $TEST_NAMESPACE
EOF

    echo "****** Applying the subscription..."
    cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: runtime-component-operator-subscription
  namespace: $TEST_NAMESPACE
spec:
  channel: $DEFAULT_CHANNEL
  name: runtime-component
  source: rco-catalog
  sourceNamespace: $TEST_NAMESPACE
  installPlanApproval: Automatic
EOF
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
    -u)
      shift
      readonly DOCKER_USERNAME="${1}"
      ;;
    -p)
      shift
      readonly DOCKER_PASSWORD="${1}"
      ;;
    --cluster-url)
      shift
      readonly CLUSTER_URL="${1}"
      ;;
    --cluster-user)
      shift
      readonly CLUSTER_USER="${1}"
      ;;
    --cluster-token)
      shift
      readonly CLUSTER_TOKEN="${1}"
      ;;
    --registry-name)
      shift
      readonly REGISTRY_NAME="${1}"
      ;;
    --registry-image)
      shift
      readonly REGISTRY_IMAGE="${1}"
      ;;
    --registry-user)
      shift
      readonly REGISTRY_USER="${1}"
      ;;
    --registry-password)
      shift
      readonly REGISTRY_PASSWORD="${1}"
      ;;  
    --release)
      shift
      readonly RELEASE="${1}"
      ;;
    --test-tag)
      shift
      readonly TEST_TAG="${1}"
      ;;
    --debug-failure)
      readonly DEBUG_FAILURE=true
      ;;
    --catalog-image)
      shift
      readonly CATALOG_IMAGE="${1}"
      ;;
    --channel)
      shift
      readonly CHANNEL="${1}"
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