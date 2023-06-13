#!/bin/bash

readonly usage="Usage: ocp-cluster-e2e.sh -u <docker-username> -p <docker-password> --cluster-url <url> --cluster-token <token> --registry-name <name> --registry-image <ns/image> --registry-user <user> --registry-password <password> --release <daily|release-tag> --test-tag <test-id> --catalog-image <catalog-image> --channel <channel>"
readonly OC_CLIENT_VERSION="latest-4.10"
readonly CONTROLLER_MANAGER_NAME="rco-controller-manager"

# setup_env: Download oc cli, log into our persistent cluster, and create a test project
setup_env() {
    echo "****** Installing OC CLI..."
    # Install kubectl and oc
    curl -L https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_CLIENT_VERSION}/openshift-client-linux.tar.gz | tar xvz
    sudo mv oc kubectl /usr/local/bin/

    if [[ "$ARCHITECTURE" == "Z" ]]; then
    {
      echo "****** Installing kubectl-kuttl..."
      curl -L -o kubectl-kuttl https://github.com/kudobuilder/kuttl/releases/download/v0.15.0/kubectl-kuttl_0.15.0_linux_x86_64
      chmod +x kubectl-kuttl
      sudo mv kubectl-kuttl /usr/local/bin
    }
    fi

    # Start a cluster and login
    echo "****** Logging into remote cluster..."
    oc login "${CLUSTER_URL}" -u "${CLUSTER_USER:-kubeadmin}" -p "${CLUSTER_TOKEN}" --insecure-skip-tls-verify=true

    # Set variables for rest of script to use
    readonly TEST_NAMESPACE="rco-test-${TEST_TAG}"
    if [[ $INSTALL_MODE = "SingleNamespace" ]]; then
      readonly INSTALL_NAMESPACE="rco-test-single-namespace-${TEST_TAG}"
    elif [[ $INSTALL_MODE = "AllNamespaces" ]]; then
      readonly INSTALL_NAMESPACE="openshift-operators"
    else
      readonly INSTALL_NAMESPACE="rco-test-${TEST_TAG}"
    fi

    if [ $INSTALL_MODE != "AllNamespaces" ]; then
      echo "****** Creating install namespace: ${INSTALL_NAMESPACE} for release ${RELEASE}"
      oc new-project "${INSTALL_NAMESPACE}" || oc project "${INSTALL_NAMESPACE}"
    fi

    echo "****** Creating test namespace: ${TEST_NAMESPACE} for release ${RELEASE}"
    oc new-project "${TEST_NAMESPACE}" || oc project "${TEST_NAMESPACE}"

    ## Create service account for Kuttl tests
    oc -n $TEST_NAMESPACE apply -f config/rbac/kuttl-rbac.yaml
}

## cleanup_env : Delete generated resources that are not bound to a test INSTALL_NAMESPACE.
cleanup_env() {
  ## Delete CRDs
  RCO_CRD_NAMES=$(oc get crd -o name | grep rc.app.stacks | cut -d/ -f2)
  echo "*** Deleting CRDs ***"
  echo "*** ${RCO_CRD_NAMES}"
  oc delete crd $RCO_CRD_NAMES

  ## Delete Subscription
  RCO_SUBSCRIPTION_NAME=$(oc -n $INSTALL_NAMESPACE get subscription -o name | grep runtime-component | cut -d/ -f2)
  echo "*** Deleting Subscription ***"
  echo "*** ${RCO_SUBSCRIPTION_NAME}"
  oc -n $INSTALL_NAMESPACE delete subscription $RCO_SUBSCRIPTION_NAME

  ## Delete CSVs
  RCO_CSV_NAME=$(oc -n $INSTALL_NAMESPACE get csv -o name | grep runtime-component | cut -d/ -f2)
  echo "*** Deleting CSVs ***"
  echo "*** ${RCO_CSV_NAME}"
  oc -n $INSTALL_NAMESPACE delete csv $RCO_CSV_NAME

  if [ $INSTALL_MODE != "OwnNamespace" ]; then
    echo "*** Deleting project ${TEST_NAMESPACE}"
    oc delete project "${TEST_NAMESPACE}"
  fi

  if [ $INSTALL_MODE != "AllNamespaces" ]; then
    echo "*** Deleting project ${INSTALL_NAMESPACE}"
    oc delete project "${INSTALL_NAMESPACE}"
  fi
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

    if [[ -z "${INSTALL_MODE}" ]]; then
        echo "****** Missing install-mode, see usage"
        echo "${usage}"
        exit 1
    fi
    
    if [[ -z "${ARCHITECTURE}" ]]; then
        echo "****** Missing architecture, see usage"
        echo "${usage}"
        exit 1
    fi

    echo "****** Setting up test environment..."
    setup_env

    if [[ "${ARCHITECTURE}" != "X" ]]; then
        echo "****** Setting up tests for ${ARCHITECTURE} architecture"
        setup_tests
    fi

    if [[ -z "${DEBUG_FAILURE}" ]]; then
        trap trap_cleanup EXIT
    else
        echo "#####################################################################################"
        echo "WARNING: --debug-failure is set. If e2e tests fail, any created resources will remain"
        echo "on the cluster for debugging/troubleshooting. YOU MUST DELETE THESE RESOURCES when"
        echo "you're done, or else they will cause future tests to fail. To cleanup manually, just"
        echo "delete the namespace \"${INSTALL_NAMESPACE}\": oc delete project \"${INSTALL_NAMESPACE}\" "
        echo "#####################################################################################"
    fi

    # login to docker to avoid rate limiting during build
    echo "${DOCKER_PASSWORD}" | docker login -u "${DOCKER_USERNAME}" --password-stdin

    trap "rm -f /tmp/pull-secret-*.yaml" EXIT

    echo "****** Logging into private registry..."
    echo "${REGISTRY_PASSWORD}" | docker login ${REGISTRY_NAME} -u "${REGISTRY_USER}" --password-stdin

    echo "sleep for 3 minutes to wait for rook-cepth, knative and cert-manager to start installing, then start monitoring for completion"
    sleep 3m
    echo "monitoring knative"
    scripts/pipeline/wait.sh deployment knative-serving
    rc_kn=$?
    echo "rc_kn=$rc_kn"
    if [[ "$rc_kn" == 0 ]]; then
        echo "knative up"
    fi
    if [[ "${ARCHITECTURE}" == "X" ]]; then
      echo "monitoring rook-ceph if architecture is ${ARCHITECTURE}"
      scripts/pipeline/wait.sh deployment rook-ceph
      rc_rk=$?
      echo "rc_rk=$rc_rk"
      if [[ "$rc_rk" == 0 ]]; then
          echo "rook-ceph up"
      fi
    fi
    echo "****** Installing operator from catalog: ${CATALOG_IMAGE} using install mode of ${INSTALL_MODE}"
    echo "****** Install namespace is ${INSTALL_NAMESPACE}.  Test namespace is ${TEST_NAMESPACE}"    
    install_operator

    # Wait for operator deployment to be ready
    while [[ $(oc -n $INSTALL_NAMESPACE get deploy "${CONTROLLER_MANAGER_NAME}" -o jsonpath='{ .status.readyReplicas }') -ne "1" ]]; do
        echo "****** Waiting for ${CONTROLLER_MANAGER_NAME} to be ready..."
        sleep 10
    done

    echo "****** ${CONTROLLER_MANAGER_NAME} deployment is ready..."

    if [[ "$ARCHITECTURE" != "Z" ]]; then
      echo "****** Testing on ${ARCHITECTURE} so starting scorecard tests..." 
      operator-sdk scorecard --verbose --kubeconfig  ${HOME}/.kube/config --selector=suite=kuttlsuite --namespace="${TEST_NAMESPACE}" --service-account="scorecard-kuttl" --wait-time 45m ./bundle || {
        echo "****** Scorecard tests failed..."
        exit 1
    }
    else
    echo "****** Testing on ${ARCHITECTURE} so running kubectl-kuttl tests..."
    kubectl-kuttl test ./bundle/tests/scorecard/kuttl --namespace "${TEST_NAMESPACE}" --timeout 200 --suppress-log=events --parallel 1 || {
       echo "****** kubectl kuttl tests failed..."
       exit 1
    }
    fi    
    result=$?

    echo "****** Cleaning up test environment..."
    if [[ "${ARCHITECTURE}" != "X" ]]; then
      revert_tests
    fi
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
  name: runtime-component-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: $CATALOG_IMAGE
  displayName: Runtime Component Catalog
  publisher: IBM
EOF

if [ $INSTALL_MODE != "AllNamespaces" ]; then
    echo "****** Applying the operator group..."
    cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: runtime-component-operator-group
  namespace: $INSTALL_NAMESPACE
spec:
  targetNamespaces:
    - $TEST_NAMESPACE
EOF
fi

    echo "****** Applying the subscription..."
    cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: runtime-component-operator-subscription
  namespace: $INSTALL_NAMESPACE
spec:
  channel: $DEFAULT_CHANNEL
  name: runtime-component
  source: runtime-component-catalog
  sourceNamespace: openshift-marketplace
  installPlanApproval: Automatic
EOF
}

setup_tests () {
  echo " As the architecture is ${ARCHITECTURE}..."
  if [[ "$ARCHITECTURE" == "P" ]]; then
  echo "Change affinity tests to look for ppc64le nodes"
  sed -i.bak "s,amd64,ppc64le," $(find ./bundle/tests/scorecard/kuttl/affinity -type f)
  echo "Change storage test to set storageclass to managed-nfs-storage"
  sed -i.bak "s,rook-cephfs,managed-nfs-storage," $(find ./bundle/tests/scorecard/kuttl/storage -type f)
  # These will need changing if a different image is used
  echo "Change image-stream tests to the correct digest for correct architecture"
  sed -i.bak "s,sha256:928559729352bfc852388b0b0db6c99593c9964c67f63ee5081fef27a4eeaa74,sha256:a59ae007d52ceaf39dd3d4ae7689cbff77cb29910e4b2e7e11edd914a7cc0875," $(find ./bundle/tests/scorecard/kuttl/image-stream -type f)
  sed -i.bak "s,sha256:3d8bfaf38927e0feb81357de701b500df129547304594d54944e75c7b15930a9,sha256:c83478e91fde4285198aa718afec3cf1e6664291b5de1aa4251a125e91bbbb41," $(find ./bundle/tests/scorecard/kuttl/image-stream -type f)
  elif [[ "$ARCHITECTURE" == "Z" ]]; then
  echo "Change affinity tests to look for s390x nodes"
  sed -i.bak "s,amd64,s390x," $(find ./bundle/tests/scorecard/kuttl/affinity -type f)
  echo "Change storage test to set storageclass to managed-nfs-storage"
  sed -i.bak "s,rook-cephfs,managed-nfs-storage," $(find ./bundle/tests/scorecard/kuttl/storage -type f)
  # These will need changing if a different image is used
  echo "Change image-stream tests to the correct digest for correct architecture"
  sed -i.bak "s,sha256:928559729352bfc852388b0b0db6c99593c9964c67f63ee5081fef27a4eeaa74,sha256:357b350302f5199477982eb5e6928397220f99dc551c9a95c284f3d8c5c5695d," $(find ./bundle/tests/scorecard/kuttl/image-stream -type f)
  sed -i.bak "s,sha256:3d8bfaf38927e0feb81357de701b500df129547304594d54944e75c7b15930a9,sha256:ed90adf813ee3e6f7429a226914827ee805e1b394c545b2e2ae4e640c9545944," $(find ./bundle/tests/scorecard/kuttl/image-stream -type f)
  else
    echo "${ARCHITECTURE} is an invalid architecture type"
    exit 1
  fi
}

revert_tests() {
  echo "Reverting test changes back to amd64"
  find ./bundle/tests/scorecard/kuttl/* -name "*.bak" -exec sh -c 'mv -f $0 ${0%.bak}' {} \;
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
    --install-mode)
      shift
      readonly INSTALL_MODE="${1}"
      ;;
    --architecture)
      shift
      readonly ARCHITECTURE="${1}"
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
