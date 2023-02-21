#!/bin/bash

# Script to build & install the runtime component operator to a private registry of an OCP cluster

# -----------------------------------------------------
# Prereqs to running this script
# -----------------------------------------------------
# 1. Have "podman" or "docker" and "oc" installed & on the path
# 2. Run "oc login .." 
# 3. Run "oc registry login --skip-check"

#------------------------------------------------------------------------
# Usage
#------------------------------------------------------------------------
# dev.sh [command] [parameters]

#   Available commands:

#   all       - Run mininum setup/install targets (init, build, catalog, subscribe)
#   init      - Initialize new OCP cluster by patching registry settings and logging in
#   build     - Build and push all images
#   catalog   - Apply CatalogSource (install operator into operator hub)
#   subscribe - Apply OperatorGroup & Subscription (install operator onto cluster)
#   login     - Login to registry
#   deploy    - Run make deploy
#   e2e       - Setup cluster for e2e scorecard tests
#   scorecard - Run scorecard tests 

set -Eeo pipefail


readonly USAGE="Usage: dev.sh all | init | login| build | catalog | subscribe | deploy | e2e | scorecard [ -host <ocp registry hostname url> -version <operator verion to build> -image <image name> -bundle <bundle image> -catalog <catalog image> -name <operator name> -namespace <namespace> -tempdir <temp dir> ]"

warn() {
  echo -e "${yel}Warning:${end} $1"
}

main() {

  parse_args "$@"

  if [[ -z "${COMMAND}" ]]; then
    echo
    echo "${USAGE}"
    echo
    exit 1
  fi

  oc status > /dev/null 2>&1 && true
  if [[ $? -ne 0 ]]; then
    echo
    echo "Run 'oc login' to log into a cluster before running dev.sh"
    echo
    exit 1
  fi

  # Favor docker if installed. Fall back to podman. 
  # Override by setting CONTAINER_COMMAND
  docker -v > /dev/null 2>&1 && true
  if [[ $? -eq 0 ]]; then
    CONTAINER_COMMAND=${CONTAINER_COMMAND:="docker"}
    TLS_VERIFY=""
  else
    CONTAINER_COMMAND=${CONTAINER_COMMAND:="podman"}
    TLS_VERIFY=${TLS_VERIFY:="--tls-verify=false"}
  fi

  SCRIPT_DIR="$(dirname "$0")"

  # Set defaults unless overridden. 
  NAMESPACE=${NAMESPACE:="runtime-component-operator"}
  
  OCP_REGISTRY_URL=${OCP_REGISTRY_URL:=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')}  > /dev/null 2>&1 && true

  if [[ -z "${OCP_REGISTRY_URL}" ]]; then
     init_cluster
  fi

  # Create project if needed
  oc project $NAMESPACE > /dev/null 2>&1 && true
  if [[ $? -ne 0 ]]; then
     oc new-project $NAMESPACE 
  fi

  VERSION=${VERSION:="latest"}
  if [[ "$VERSION" == "latest" ]]; then
      VVERSION=$VERSION
  else
      VVERSION=${VVERSION:=v$VERSION}
  fi
  OPERATOR_NAME=${OPERATOR_NAME:="operator"}
  IMAGE_TAG_BASE=${IMAGE_TAG_BASE:=$OCP_REGISTRY_URL/$NAMESPACE/$OPERATOR_NAME:$VVERSION}
  IMG=${IMG:=$OCP_REGISTRY_URL/$NAMESPACE/$OPERATOR_NAME:$VVERSION}
  BUNDLE_IMG=${BUNDLE_IMG:=$OCP_REGISTRY_URL/$NAMESPACE/$OPERATOR_NAME-bundle:$VVERSION}
  CATALOG_IMG=${CATALOG_IMG:=$OCP_REGISTRY_URL/$NAMESPACE/$OPERATOR_NAME-catalog:$VVERSION}
  MAKEFILE_DIR=${MAKEFILE_DIR:=$SCRIPT_DIR/..}
  TEMP_DIR=${TEMP_DIR:=/tmp}
  
  if [[ "$COMMAND" == "all" ]]; then
     login_registry
     build
     bundle
     catalog
     apply_catalog
     apply_og
     apply_subscribe
  elif [[ "$COMMAND" == "init" ]]; then
     init_cluster
     login_registry
  elif [[ "$COMMAND" == "build" ]]; then
     build
     bundle
     catalog
  elif [[ "$COMMAND" == "catalog" ]]; then
     apply_catalog
  elif [[ "$COMMAND" == "subscribe" ]]; then
     apply_og
     apply_subscribe
  elif [[ "$COMMAND" == "login" ]]; then
     login_registry
  elif [[ "$COMMAND" == "e2e" ]]; then
     add_rbac
     install_rook
     install_serverless
     setup_knative_serving
     install_cert_manager
     add_affinity_label_to_node
     create_image_content_source_policy
  elif [[ "$COMMAND" == "scorecard" ]]; then
     run_scorecard
  elif [[ "$COMMAND" == "deploy" ]]; then
     deploy
  else 
    echo
    echo "Command $COMMAND unrecognized."
    echo
    echo "${USAGE}"
    exit 1
  fi

}

#############################################################################
# Setup an OCP cluster to use the private registry, insecurely (testing only)
#############################################################################
init_cluster() {
    oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
    OCP_REGISTRY_URL=${OCP_REGISTRY_URL:=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')}
    oc patch image.config.openshift.io/cluster  --patch '{"spec":{"registrySources":{"insecureRegistries":["'$OCP_REGISTRY_URL'"]}}}' --type=merge
}

login_registry() {
    echo  $CONTAINER_COMMAND login $TLS_VERIFY -u kubeadmin -p $(oc whoami -t) $OCP_REGISTRY_URL
    $CONTAINER_COMMAND login $TLS_VERIFY -u kubeadmin -p $(oc whoami -t) $OCP_REGISTRY_URL
    oc registry login --skip-check   
}

apply_catalog() {
    CATALOG_FILE=/$TEMP_DIR/catalog.yaml    
    
cat << EOF > $CATALOG_FILE
    apiVersion: operators.coreos.com/v1alpha1
    kind: CatalogSource
    metadata:
      name: runtime-component-operator-catalog
      namespace: $NAMESPACE
    spec:
      sourceType: grpc
      image: $CATALOG_IMG
      imagePullPolicy: Always
      displayName: Runtime Component Catalog
      updateStrategy:
        registryPoll:
          interval: 1m
EOF

    oc apply -f $CATALOG_FILE
    rm  $CATALOG_FILE
}

apply_subscribe() {
    SUBCRIPTION_FILE=/$TEMP_DIR/subscription.yaml    
    
cat << EOF > $SUBCRIPTION_FILE
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: runtime-component-operator-operator-subscription
  namespace: $NAMESPACE
spec:
  channel:  beta2
  name: runtime-component-operator
  source: runtime-component-operator-catalog
  sourceNamespace: $NAMESPACE
  installPlanApproval: Automatic
EOF

    oc apply -f $SUBCRIPTION_FILE
    rm $SUBCRIPTION_FILE          
}

apply_og() {
    OG_FILE=/$TEMP_DIR/og.yaml    
    
cat << EOF > $OG_FILE
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: rco-operator-group
  namespace: $NAMESPACE
spec:
  targetNamespaces:
    - $NAMESPACE  
EOF

    oc apply -f $OG_FILE
    rm $OG_FILE          
}

###################################
# Make deploy
###################################
deploy() {
    echo "------------"
    echo "deploy"
    echo "------------"
    make -C  $MAKEFILE_DIR install VERSION=$VVERSION IMG=$IMG IMAGE_TAG_BASE=$IMAGE_TAG_BASE BUNDLE_IMG=$BUNDLE_IMG CATALOG_IMG=$CATALOG_IMG TLS_VERIFY=false
    make -C  $MAKEFILE_DIR deploy VERSION=$VVERSION IMG=$IMG IMAGE_TAG_BASE=$IMAGE_TAG_BASE BUNDLE_IMG=$BUNDLE_IMG CATALOG_IMG=$CATALOG_IMG TLS_VERIFY=false
}

###################################
# Build and push the operator image
###################################
build() {
    echo "------------"
    echo "docker-build"
    echo "------------"
    cd $MAKEFILE_DIR
    go mod vendor
    cd -
    make -C  $MAKEFILE_DIR docker-build VERSION=$VVERSION IMG=$IMG IMAGE_TAG_BASE=$IMAGE_TAG_BASE BUNDLE_IMG=$BUNDLE_IMG CATALOG_IMG=$CATALOG_IMG TLS_VERIFY=false
    echo "------------"
    echo "docker-push"
    echo "------------"
    make -C  $MAKEFILE_DIR docker-push VERSION=$VVERSION IMG=$IMG IMAGE_TAG_BASE=$IMAGE_TAG_BASE BUNDLE_IMG=$BUNDLE_IMG CATALOG_IMG=$CATALOG_IMG TLS_VERIFY=false
}

###################################
# Build and push the bundle image
###################################
bundle() {
    echo "------------"
    echo "bundle"
    echo "------------"
    # Special case, Makefile make bundle only handles semantic versioning
    if [[ "$VERSION" == "latest" ]]; then
        make -C  $MAKEFILE_DIR bundle IMG=$IMG VERSION=9.9.9
        sed -i 's/$OCP_REGISTRY_URL\/$NAMESPACE\/$OPERATOR_NAME:v9.9.9/$OCP_REGISTRY_URL\/$NAMESPACE\/$OPERATOR_NAME:latest/g' $MAKEFILE_DIR/bundle/manifests/runtime-component.clusterserviceversion.yaml
    else
        make -C  $MAKEFILE_DIR bundle IMG=$IMG VERSION=$VERSION
    fi
   
    echo "------------"
    echo "bundle-build"
    echo "------------"
    make -C  $MAKEFILE_DIR bundle-build VERSION=$VVERSION IMG=$IMG IMAGE_TAG_BASE=$IMAGE_TAG_BASE BUNDLE_IMG=$BUNDLE_IMG CATALOG_IMG=$CATALOG_IMG TLS_VERIFY=false 
    echo "------------"
    echo "bundle-push"
    echo "------------"
    make -C  $MAKEFILE_DIR bundle-push VERSION=$VVERSION IMG=$IMG IMAGE_TAG_BASE=$IMAGE_TAG_BASE BUNDLE_IMG=$BUNDLE_IMG CATALOG_IMG=$CATALOG_IMG TLS_VERIFY=false
}

###################################
# Build and push the bundle image
###################################
catalog() {
    echo "------------"
    echo "catalog-build"
    echo "------------"
    make -C  $MAKEFILE_DIR catalog-build VERSION=$VVERSION IMG=$IMG IMAGE_TAG_BASE=$IMAGE_TAG_BASE BUNDLE_IMG=$BUNDLE_IMG CATALOG_IMG=$CATALOG_IMG TLS_VERIFY=false
    echo "------------"
    echo "catalog-push"
    echo "------------"
    make -C  $MAKEFILE_DIR catalog-push VERSION=$VVERSION IMG=$IMG IMAGE_TAG_BASE=$IMAGE_TAG_BASE BUNDLE_IMG=$BUNDLE_IMG CATALOG_IMG=$CATALOG_IMG TLS_VERIFY=false
}

parse_args() {
    readonly COMMAND="$1"

    while [ $# -gt 0 ]; do
    case "$1" in
    -host)
      shift
      OCP_REGISTRY_URL="${1}"
      ;;
    -namespace)
      shift
      NAMESPACE="${1}"
      ;;
    -version)
      shift
      VERSION="${1}"
      ;;
    -image)
       IMG="${1}"
      ;;
    -catalog)
       CATALOG_IMG="${1}"
      ;;
    -bundle)
       BUNDLE_IMG="${1}"
      ;;   
    -tempdir)
      shift
      TEMP_DIR="${1}"
      ;;         
    esac
    shift
  done
}


add_rbac() {
     oc apply -f $MAKEFILE_DIR/config/rbac/kuttl-rbac.yaml
}

run_scorecard() {
     operator-sdk scorecard --verbose --selector=suite=kuttlsuite --namespace $NAMESPACE --service-account scorecard-kuttl --wait-time 60m $MAKEFILE_DIR/bundle
}

install_rook() {
    if ! oc get storageclass | grep rook-ceph >/dev/null; then
    echo "Installing Rook storage orchestrator..."

    cur_dir="$(pwd)"
    tmp_dir=$(mktemp -d -t ceph-XXXXXXXXXX)
    cd "$tmp_dir"

    git clone --single-branch --branch master https://github.com/rook/rook.git
    cd rook/deploy/examples

    oc create -f crds.yaml
    oc create -f common.yaml
    oc create -f operator-openshift.yaml
    oc create -f cluster.yaml
    oc create -f ./csi/rbd/storageclass.yaml
    oc create -f ./csi/rbd/pvc.yaml
    oc create -f ./csi/rbd/snapshotclass.yaml
    oc create -f filesystem.yaml
    oc create -f ./csi/cephfs/storageclass.yaml
    oc create -f ./csi/cephfs/pvc.yaml
    oc create -f ./csi/cephfs/snapshotclass.yaml
    oc create -f toolbox.yaml

    oc patch storageclass rook-cephfs -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
    cd "$cur_dir"
    echo
  else
    echo "Rook storage orchestrator is already installed."
  fi
}

install_serverless() {
  if ! oc get subs -n openshift-operators | grep serverless-operator >/dev/null; then
    echo "Installing Red Hat Openshift Serverless operator..."
    name="serverless-operator"
    packageManifest="$(oc get packagemanifests $name -n openshift-marketplace -o jsonpath="{.status.catalogSource},{.status.catalogSourceNamespace},{.status.channels[?(@.name=='stable')].currentCSV}")"
    catalogSource="$(echo $packageManifest | cut -d, -f1)"
    catalogSourceNamespace="$(echo $packageManifest | cut -d, -f2)"
    currentCSV="$(echo $packageManifest | cut -d, -f3)"
    cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: $name
  namespace: openshift-operators
  generateName: serverless-operator-
spec:
  channel: stable
  installPlanApproval: Automatic
  name: $name
  source: $catalogSource
  sourceNamespace: $catalogSourceNamespace
  startingCSV: $currentCSV
EOF
    echo
  else
    echo "Red Hat Openshift Serverless operator is already installed."
  fi
}

setup_knative_serving() {
  if ! oc get knativeserving.operator.knative.dev/knative-serving -n knative-serving >/dev/null; then
    echo "Waiting 30 seconds for Serverless operator to finish being set up..."
    sleep 30
    wait_count=0
    KNS_FILE=/$TEMP_DIR/kns.yaml  
    cat << EOF > $KNS_FILE
apiVersion: operator.knative.dev/v1alpha1
kind: KnativeServing
metadata:
    name: knative-serving
    namespace: knative-serving
EOF
    while [ $wait_count -le 20 ]
    do
      echo "Creating Knative Serving instance..."
      oc apply -f $KNS_FILE > /dev/null 2>&1 && true
      [[ $? == 0 ]] && break
      warn "Knative Serving configuration failed (probably because the Serverless operator isn't done being set up). Trying again in 15 seconds."
      ((wait_count++))
      sleep 15
    done
    rm $KNS_FILE
    echo
  else
    echo "Knative Serving instance is already created."
  fi
}

install_cert_manager() {
  if ! oc get deployments -n cert-manager 2>/dev/null | grep cert-manager >/dev/null; then
    echo "Installing cert-manager..."
    oc apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml
    echo
  else
    echo "cert-manager is already installed."
  fi
}

add_affinity_label_to_node() {
  affinity_label="kuttlTest=test1"
  labeled_node="$(oc get nodes -l "$affinity_label" -o name)"
  if [[ -z "$labeled_node" ]]; then
    first_worker="$(oc get nodes | grep worker | head -1 | awk '{print $1}')"
    echo "Adding affinity label ($affinity_label) to worker node $first_worker..."
    oc label --overwrite node "$first_worker" "$affinity_label"
    echo
  else
    echo "Affinity label ($affinity_label) already exists on worker node $labeled_node."
  fi
}

create_image_content_source_policy() {
  if ! oc get imagecontentsourcepolicy | grep mirror-config >/dev/null; then
    echo "Adding ImageContentSourcePolicy to mirror to staging repository..."
    cat <<EOF | oc apply -f -
apiVersion: operator.openshift.io/v1alpha1
kind: ImageContentSourcePolicy
metadata:
    name: mirror-config
spec:
    repositoryDigestMirrors:
    - mirrors:
      - cp.stg.icr.io/cp
      source: cp.icr.io/cp
    - mirrors:
      - cp.stg.icr.io/cp
      source: icr.io/cpopen
EOF
    echo
  else
    echo "ImageContentSourcePolicy to mirror to staging repository already exists."
  fi
}

add_stg_registry_pull_secret() {
  dockerconfigjson="$(oc extract secret/pull-secret -n openshift-config --to=-)"
  if [[ "$(echo $dockerconfigjson | jq '.auths["cp.stg.icr.io"]')" == "null" ]]; then
    echo "Adding a pull secret for cp.stg.icr.io to .dockerconfigjson..."
    auth="$(echo "iamapikey:${REGISTRY_KEY}" | base64)"
    echo $dockerconfigjson | jq --arg auth "$auth" '.auths["cp.stg.icr.io"]={"email":"unused","auth":$auth}' > /tmp/.dockerconfigjson
    oc set data secret/pull-secret -n openshift-config --from-file=/tmp/.dockerconfigjson
    rm /tmp/.dockerconfigjson
  else
    echo ".dockerconfigjson already contains a pull secret for cp.stg.icr.io."
  fi
}

add_cluster_admin() {
  NEW_ADMIN_USER="$(echo "${NEW_ADMIN_CREDS}" | cut -d: -f1)"
  NEW_ADMIN_PASS="$(echo "${NEW_ADMIN_CREDS}" | cut -d: -f2)"

  cur_dir="$(pwd)"
  tmp_dir=$(mktemp -d -t htpasswd-XXXXXXXXXX)
  cd "$tmp_dir"

  if ! oc get user "${NEW_ADMIN_USER}" >/dev/null 2>&1; then
    if oc get secret htpass-secret >/dev/null 2>&1; then
      echo "Adding new user \"${NEW_ADMIN_USER}\" to cluster's HTPasswd secret..."
      oc get secret htpass-secret -ojsonpath={.data.htpasswd} -n openshift-config | base64 --decode > users.htpasswd
      htpasswd -bB users.htpasswd "${NEW_ADMIN_USER}" "${NEW_ADMIN_PASS}"

      echo "Updating HTPasswd secret on cluster..."
      oc create secret generic htpass-secret --from-file=htpasswd=users.htpasswd --dry-run=client -o yaml -n openshift-config | oc replace -f -
    else
      echo "Creating HTPasswd file for new user \"${NEW_ADMIN_USER}\"..."
      htpasswd -c -B -b users.htpasswd "${NEW_ADMIN_USER}" "${NEW_ADMIN_PASS}"

      echo "Adding HTPasswd secret to cluster..."
      oc create secret generic htpass-secret --from-file=htpasswd=users.htpasswd -n openshift-config
    fi

    cd "$cur_dir"

    echo "Add HTPasswd identity provider to cluster..."
    cat <<EOF | oc apply -f -
apiVersion: config.openshift.io/v1
kind: OAuth
metadata:
  name: cluster
spec:
  identityProviders:
  - name: htpasswd-provider
    mappingMethod: claim
    type: HTPasswd
    htpasswd:
      fileData:
        name: htpass-secret
EOF
        
    wait_count=0
    while [ $wait_count -le 20 ]
    do
      echo "Adding cluster-admin role to \"${NEW_ADMIN_USER}\" user..."
      oc adm policy add-cluster-role-to-user cluster-admin "${NEW_ADMIN_USER}"
      [[ $? == 0 ]] && break
      warn "Adding cluster-admin role to \"${NEW_ADMIN_USER}\" user failed (probably because the identity provider isn't done being set up). Trying again in 15 seconds."
      ((wait_count++))
      sleep 15
    done
    echo
  else
    echo "The user \"${NEW_ADMIN_USER}\" already exists on this cluster."
  fi
}


main "$@"
