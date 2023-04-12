#!/bin/bash

readonly usage="Usage: scripts/e2e-kind.sh --test-tag <test-id> -u <fyre-user> -k <fyre-API-key> -p <fyre-root-password> -pgid <fyre-product-group-id>"

readonly KUBE_CLUSTER_NAME="kind-e2e-cluster"
readonly BUILD_IMAGE="runtime-component-operator:latest"

readonly RUNASUSER="\n  securityContext:\n    runAsUser: 1001"
readonly APPIMAGE='applicationImage:\s'
readonly IMAGE='k8s.gcr.io\/pause:3.2'

main() {
    parse_args "$@"

    if [[ "${DEBUG_FAILURE}" != true ]]; then
        trap trap_cleanup EXIT
    else
        echo "#####################################################################################"
        echo "WARNING: --debug-failure is set. If e2e tests fail, the Fyre VM used will not be"
        echo "cleaned up for debugging/troubleshooting. YOU MUST DELETE THE KIND CLUSTER VM when"
        echo "you're done, or else provisioning new test VMs will eventually fail. To cleanup"
        echo "manually, just delete run the following command:"
        echo "$(dirname $0)/delete-fyre-stack.sh --cluster-name ${REMOTE_CLUSTER_NAME} --user ${FYRE_USER} --key <fyre-api-key>"
        echo "#####################################################################################"
    fi

    echo "****** Setting up test environment..."
    setup_env
    install_tools
    build_push
    install_rco
    setup_test

    if [[ "${SETUP_ONLY}" == true ]]; then
        exit 0
    fi

    echo "****** Starting kind scorecard tests..."
    operator-sdk scorecard --verbose --kubeconfig ${HOME}/.kube/config --selector=suite=kuttlsuite --namespace "${TEST_NAMESPACE}" --service-account scorecard-kuttl --wait-time 45m ./bundle || {
        echo "****** Scorecard tests failed..."
        exit 1
    }

    exit 0
}

# setup_env: Create a Fyre VM with kind installed and 
setup_env() {
    if test -f "/usr/share/keyrings/docker-archive-keyring.gpg"; then
        rm /usr/share/keyrings/docker-archive-keyring.gpg
    fi
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
    local release="$(cat /etc/os-release | grep UBUNTU_CODENAME | cut -d '=' -f 2)"
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu ${release} stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

    sudo apt-get update -y
    sudo apt-get install -y docker-ce docker-ce-cli containerd.io sshpass jq

    if ! command -v kubectl &> /dev/null; then
        echo "****** Installing kubectl v1.24.2..."
        curl -Lo /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.24.2/bin/linux/amd64/kubectl && chmod +x /usr/local/bin/kubectl
    fi

    # Create a remote Kind cluster
    $(dirname $0)/create-fyre-stack.sh --cluster-name "${REMOTE_CLUSTER_NAME}" --user "${FYRE_USER}" --key "${FYRE_KEY}" --pass "${FYRE_PASS}" --product-group-id "${FYRE_PRODUCT_GROUP_ID}" --init-script $(dirname $0)/setup-kind-cluster.sh || {
        echo "Error: unable to build kind cluster"
        exit 1
    }

    # Grab kube config
    mkdir -p ${HOME}/.kube
    sshpass -p "${FYRE_PASS}" scp -o LogLevel=ERROR -o StrictHostKeyChecking=no root@${REMOTE_CLUSTER}:/root/.kube/config ${HOME}/.kube/config

    kubectl config set-context ${KUBE_CLUSTER_NAME} --namespace="${TEST_NAMESPACE}" || {
        echo "Error: Failed to set kube context"
        exit 1
    }

    ## Add label to node for affinity test
    kubectl label node --overwrite "e2e-cluster-worker" kuttlTest=test1


    ## Create service account for Kuttl tests
    kubectl apply -f config/rbac/kind-kuttl-rbac.yaml
}

install_tools() {
    echo "****** Installing Prometheus"
    kubectl apply --server-side -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml

    echo "****** Installing Knative"
    kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.7.4/serving-crds.yaml
    kubectl apply -f https://github.com/knative/eventing/releases/download/knative-v1.7.4/eventing-crds.yaml

    echo "****** Installing Cert Manager"
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.9.2/cert-manager.yaml

    echo "****** Enabling Ingress"
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
    wait_for ingress-nginx ingress-nginx-controller
}

build_push() {
    ## Build Docker image and push to private registry
    docker build -t "${LOCAL_REGISTRY}/${BUILD_IMAGE}" . || {
        echo "Error: Failed to build Runtime Component Operator"
        exit 1
    }

    docker save ${LOCAL_REGISTRY}/${BUILD_IMAGE} | sshpass -p "${FYRE_PASS}" ssh -o LogLevel=ERROR -o StrictHostKeyChecking=no -C ${REMOTE_CLUSTER} "docker load && docker push ${LOCAL_REGISTRY}/${BUILD_IMAGE}" || {
        echo "Error: Failed to push Runtime Component Operator"
        exit 1
    }
}

# install_rco: Kustomize and install RuntimeComponent-Operator
install_rco() {
    echo "****** Installing RCO in namespace: ${TEST_NAMESPACE}"
    kubectl create -f bundle/manifests/rc.app.stacks_runtimecomponents.yaml
    kubectl create -f bundle/manifests/rc.app.stacks_runtimeoperations.yaml

    sed -i "s|image: .*|image: ${LOCAL_REGISTRY}/${BUILD_IMAGE}|
            s|namespace: .*|namespace: ${TEST_NAMESPACE}|" deploy/kustomize/daily/base/runtime-component-operator.yaml
    
    kubectl create -f deploy/kustomize/daily/base/runtime-component-operator.yaml -n ${TEST_NAMESPACE}

    # Wait for operator deployment to be ready
    wait_for ${TEST_NAMESPACE} rco-controller-manager
}

setup_test() {
    echo "****** Installing kuttl"
    mkdir krew && cd krew
    curl -OL https://github.com/kubernetes-sigs/krew/releases/latest/download/krew-linux_amd64.tar.gz \
    && tar -xvzf krew-linux_amd64.tar.gz \
    && ./krew-linux_amd64 install krew
    cd .. && rm -rf krew
    export PATH="$HOME/.krew/bin:$PATH"
    kubectl krew install kuttl

    ## Add tests for kind cluster
    mv bundle/tests/scorecard/kind-kuttl/ingress bundle/tests/scorecard/kuttl/
    mv bundle/tests/scorecard/kind-kuttl/ingress-certificate bundle/tests/scorecard/kuttl/
    mv bundle/tests/scorecard/kind-kuttl/ingress-manage-tls bundle/tests/scorecard/kuttl/
    
    ## Remove tests that do not apply for kind cluster
    mv bundle/tests/scorecard/kuttl/network-policy bundle/tests/scorecard/kind-kuttl/
    mv bundle/tests/scorecard/kuttl/network-policy-multiple-apps bundle/tests/scorecard/kind-kuttl/
    mv bundle/tests/scorecard/kuttl/routes bundle/tests/scorecard/kind-kuttl/
    mv bundle/tests/scorecard/kuttl/route-certificate bundle/tests/scorecard/kind-kuttl/
    mv bundle/tests/scorecard/kuttl/manage-tls bundle/tests/scorecard/kind-kuttl/
    mv bundle/tests/scorecard/kuttl/image-stream bundle/tests/scorecard/kind-kuttl/

    files=($(grep -rwl 'bundle/tests/scorecard/kuttl/' -e $APPIMAGE$IMAGE))
    for file in "${files[@]}"; do
        sed -i "s/$IMAGE/$IMAGE$RUNASUSER/" $file
    done
}

## cleanup: Delete generated resources that are not bound to a test TEST_NAMESPACE.
cleanup() {
    echo
    echo "****** Cleaning up test environment..."
    $(dirname $0)/delete-fyre-stack.sh --cluster-name ${REMOTE_CLUSTER_NAME} --user "${FYRE_USER}" --key "${FYRE_KEY}"

    ## Restore tests and configs
    git clean -fd bundle/tests/scorecard
    git restore bundle/tests/scorecard internal/deploy
}

trap_cleanup() {
    # Preserve exit code
    last_status=$?

    if [[ "${SETUP_ONLY}" != true ]]; then
        cleanup
    fi

    exit ${last_status}
}

wait_for() {
    local namespace="${1}"
    local deployment="${2}"

    while [[ $(kubectl get deploy -n "${namespace}" "${deployment}" -o jsonpath='{.status.readyReplicas}') -ne "1" ]]; do
        echo "****** Waiting for ${namespace}/${deployment} to be ready..."
        sleep 10
    done
    echo "****** ${namespace}/${deployment} is ready..."
}


parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
        --test-tag)
            shift
            readonly TEST_TAG="${1}"
            ;;
        -u|--user)
            shift
            readonly FYRE_USER="${1}"
            ;;
        -k|--key)
            shift
            readonly FYRE_KEY="${1}"
            ;;
        -p|--pass)
            shift
            readonly FYRE_PASS="${1}"
            ;;
        -pgid|--product-group-id)
            shift
            readonly FYRE_PRODUCT_GROUP_ID="${1}"
            ;;
        --setup-only)
            readonly SETUP_ONLY=true
            ;;
        --debug-failure)
            readonly DEBUG_FAILURE=true
            ;;
        *)
            echo "Error: Invalid argument - $1"
            echo "$usage"
            exit 1
            ;;
        esac
        shift
    done

    if [[ -z "${TEST_TAG}" ]]; then
        echo "****** Missing test id, see usage"
        echo "${usage}"
        exit 1
    fi

    readonly TEST_NAMESPACE="rco-test"
    readonly REMOTE_CLUSTER_NAME="rco-test-${TEST_TAG}-cluster"
    readonly REMOTE_CLUSTER="${REMOTE_CLUSTER_NAME}1.fyre.ibm.com"
    readonly LOCAL_REGISTRY="localhost:5000"
}

main "$@"
