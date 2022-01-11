#!/bin/bash

readonly usage="Usage: e2e-minikube.sh --test-tag <test-id>"
readonly SERVICE_ACCOUNT="travis-tests"

# setup_env: Download kubectl cli and Minikube, start Minikube, and create a test project
setup_env() {
    # Install Minikube and Start a cluster
    echo "****** Installing and starting Minikube"
    scripts/installers/install-minikube.sh

    readonly TEST_NAMESPACE="rco-test-${TEST_TAG}"

    echo "****** Creating test namespace: ${TEST_NAMESPACE}"
    kubectl create namespace "${TEST_NAMESPACE}"
    kubectl config set-context $(kubectl config current-context) --namespace="${TEST_NAMESPACE}"

    ## Create service account for Kuttl tests
    kubectl apply -f config/rbac/minikube-kuttl-rbac.yaml
    
    ## Add label to node for affinity test
    kubectl label node "minikube" kuttlTest=test1
}

# install_rco: Kustomize and install Runtime-Component-Operator
install_rco() {
    echo "****** Installing RCO in namespace: ${TEST_NAMESPACE}"
    kubectl apply -f bundle/manifests/rc.app.stacks_runtimecomponents.yaml
    kubectl apply -f bundle/manifests/rc.app.stacks_runtimeoperations.yaml
    kubectl apply -f deploy/kustomize/daily/base/runtime-component-operator.yaml -n ${TEST_NAMESPACE}
}

function install_tools() {
    echo "****** Installing Prometheus"
    kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml
    
    echo "****** Installing Knative"
    minikube addons enable registry
    kubectl apply -f https://github.com/knative/serving/releases/download/v0.24.0/serving-crds.yaml
    kubectl apply -f https://github.com/knative/eventing/releases/download/v0.24.0/eventing-crds.yaml
  
    echo "****** Enabling Ingress"
    minikube addons enable ingress
}

## cleanup_env : Delete generated resources that are not bound to a test TEST_NAMESPACE.
cleanup_env() {
    kubectl delete namespace "${TEST_NAMESPACE}"
    minikube stop && minikube delete
}

function setup_test() {
    echo "****** Installing kuttl"
    mkdir krew && cd krew
    curl -OL https://github.com/kubernetes-sigs/krew/releases/latest/download/krew-linux_amd64.tar.gz \
    && tar -xvzf krew-linux_amd64.tar.gz \
    && ./krew-linux_amd64 install krew
    cd .. && rm -rf krew
    export PATH="$HOME/.krew/bin:$PATH"
    kubectl krew install kuttl

    ## Add tests for minikube
    mv bundle/tests/scorecard/minikube-kuttl/ingress bundle/tests/scorecard/kuttl/
    mv bundle/tests/scorecard/minikube-kuttl/ingress-certificate bundle/tests/scorecard/kuttl/
    
    ## Remove tests that do not apply for minikube
    mv bundle/tests/scorecard/kuttl/routes bundle/tests/scorecard/minikube-kuttl/
    mv bundle/tests/scorecard/kuttl/route-certificate bundle/tests/scorecard/minikube-kuttl/
    mv bundle/tests/scorecard/kuttl/stream bundle/tests/scorecard/minikube-kuttl/
}

function cleanup_test() {
    ## Restore tests
    mv bundle/tests/scorecard/kuttl/ingress bundle/tests/scorecard/minikube-kuttl/
    mv bundle/tests/scorecard/kuttl/ingress-certificate bundle/tests/scorecard/minikube-kuttl/
    
    mv bundle/tests/scorecard/minikube-kuttl/routes bundle/tests/scorecard/kuttl/ 
    mv bundle/tests/scorecard/minikube-kuttl/route-certificate bundle/tests/scorecard/kuttl/ 
    mv bundle/tests/scorecard/minikube-kuttl/stream bundle/tests/scorecard/kuttl/ 
}

main() {
    parse_args "$@"
     
    if [[ -z "${TEST_TAG}" ]]; then
        echo "****** Missing test id, see usage"
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
    
    setup_test
    
    echo "****** Starting minikube scorecard tests..."
    operator-sdk scorecard --verbose --selector=suite=kuttlsuite --namespace "${TEST_NAMESPACE}" --service-account scorecard-kuttl --wait-time 30m ./bundle || {
        echo "****** Scorecard tests failed..."
    }
    result=$?

    cleanup_test

    echo "****** Cleaning up test environment..."
    cleanup_env

    return $result
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
        --test-tag)
            shift
            readonly TEST_TAG="${1}"
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
