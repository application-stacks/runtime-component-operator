#!/bin/bash

readonly usage="Usage: e2e-minikube.sh -n <test-namespace>"
readonly SERVICE_ACCOUNT="travis-tests"

# setup_env: Download kubectl cli and Minikube, start Minikube, and create a test project
setup_env() {
    # Install Minikube and Start a cluster
    echo "****** Installing and starting Minikube"
    scripts/installers/install-minikube.sh

    echo "****** Creating test namespace: ${TEST_NAMESPACE}"
    kubectl create namespace "${TEST_NAMESPACE}"
    kubectl config set-context $(kubectl config current-context) --namespace="${TEST_NAMESPACE}"

    ## Create service account for Kuttl tests
    kubectl apply -f config/rbac/minikube-kuttl-rbac.yaml
}

# install_rco: Kustomize and install Runtime-Component-Operator
install_rco() {
    echo "****** Install RCO in namespace: ${TEST_NAMESPACE}"
    kubectl apply -f bundle/manifests/rc.app.stacks_runtimecomponents.yaml
    kubectl apply -f bundle/manifests/rc.app.stacks_runtimeoperations.yaml
    kubectl apply -f deploy/kustomize/daily/base/runtime-component-operator.yaml -n ${TEST_NAMESPACE}
}

function install_tools() {
  echo "****** Install Prometheus"
  kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml
  
  echo "****** Enable Ingress"
  minikube addons enable ingress
}

## cleanup_env : Delete generated resources that are not bound to a test TEST_NAMESPACE.
cleanup_env() {
  kubectl delete namespace "${TEST_NAMESPACE}"
  minikube stop
}

main() {
    parse_args "$@"
     
    if [[ -z "${TEST_NAMESPACE}" ]]; then
        echo "****** Missing test namespace, see usage"
        echo "${usage}"
        exit 1
    fi

    echo "****** Setting up test environment..."
    setup_env
    install_rco
    install_tools

    # Wait for operator deployment to be ready
    while [[ $(kubectl get deploy rco-controller-manager -o jsonpath='{ .status.readyReplicas }') -ne "1" ]]; do
        echo "****** Waiting for rco-controller-manager to be ready..."
        sleep 10
    done
    echo "****** rco-controller-manager deployment is ready..."
    
    echo "****** Starting minikube scorecard tests..."
    mv bundle/tests/scorecard/kuttl/ bundle/tests/scorecard/disable-kuttl/
    mv bundle/tests/scorecard/minikube-kuttl/ bundle/tests/scorecard/kuttl/
    
    operator-sdk scorecard --verbose --selector=suite=kuttlsuite --namespace "${TEST_NAMESPACE}" --service-account scorecard-kuttl --wait-time 10m ./bundle || {
      echo "****** Scorecard tests failed..."
    }
    result=$?
    
    mv bundle/tests/scorecard/kuttl/ bundle/tests/scorecard/minikube-kuttl/
    mv bundle/tests/scorecard/disable-kuttl/ bundle/tests/scorecard/kuttl/

    echo "****** Cleaning up test environment..."
    cleanup_env
    
    echo "****** Minikube stopped"
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
    -n)
      shift
      readonly TEST_NAMESPACE="${1}"
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
