#!/bin/bash
arch=$1
source ./clusterWait.sh $arch
clusterurl="$ip:6443"

echo "in directory"
pwd

echo "running configure-cluster.sh"
git clone --single-branch --branch main https://$(get_env git-token)@github.ibm.com/websphere/operators.git
ls -l operators/scripts/configure-cluster/configure-cluster.sh
echo "**** issuing oc login"
oc login --insecure-skip-tls-verify $clusterurl -u kubeadmin -p $token
echo "Open Shift Console:"
console=$(oc whoami --show-console)
echo $console
echo "*** after issuing oc login"
operators/scripts/configure-cluster/configure-cluster.sh -p $token -k $(get_env ibmcloud-api-key-staging) --arch $arch -A


export GO_VERSION=$(get_env go-version)
make setup-go GO_RELEASE_VERSION=$GO_VERSION
export PATH=$PATH:/usr/local/go/bin
export INSTALL_MODE=$(get_env install-mode)
export ARCHITECTURE=$arch

# OCP test
export PIPELINE_USERNAME=$(get_env ibmcloud-api-user)
export PIPELINE_PASSWORD=$(get_env ibmcloud-api-key-staging)
export PIPELINE_REGISTRY=$(get_env pipeline-registry)
export PIPELINE_OPERATOR_IMAGE=$(get_env pipeline-operator-image)
export DOCKER_USERNAME=$(get_env docker-username)
export DOCKER_PASSWORD=$(get_env docker-password)
#export CLUSTER_URL=$(get_env test-cluster-url)
export CLUSTER_URL=$clusterurl
#export CLUSTER_USER=$(get_env test-cluster-user kubeadmin)
export CLUSTER_TOKEN=$token
export RELEASE_TARGET=$(get_env branch)
export DEBUG_FAILURE=$(get_env debug-failure)

# Kind test
export FYRE_USER=$(get_env fyre-user)
export FYRE_KEY=$(get_env fyre-key)
export FYRE_PASS=$(get_env fyre-pass)
export FYRE_PRODUCT_GROUP_ID=$(get_env fyre-product-group-id)

cd ../..
echo "directory before acceptance-test.sh"
pwd

scripts/acceptance-test.sh
rc=$?

echo "switching back to ebc-gateway-http directory"
cd scripts/pipeline/ebc-gateway-http

if [[ "$rc" == 0 ]]; then
    ./ebc_complete.sh
else
    hours=$(get_env ebc_autocomplete_hours "6")
    echo "Your acceptance test failed, the cluster will be retained for $hours hours."
    echo "debug of cluster may be required, issue @ebc debug $rco_demand_id in #was-ebc channel to keep cluster for debug"
    echo "issue @ebc debugcomplete $rco_demand_id when done debugging in #was-ebc channel "
    echo "access console at: $console"
    echo "credentials: kubeadmin/$token"
    slack_users=$(get_env slack_users)
    echo "slack_users=$slack_users"
    eval "arr=($slack_users)"
    for user in "${arr[@]}"; do 
    echo "user=$user"
    curl -X POST -H 'Content-type: application/json' --data '{"text":"<'$user'>  accceptance test failure see below "}' $(get_env slack_web_hook_url)
    echo " "
    done
    pipeline_url="https://cloud.ibm.com/devops/pipelines/tekton/${PIPELINE_ID}/runs/${PIPELINE_RUN_ID}"
    curl -X POST -H 'Content-type: application/json' --data '{"text":"Your acceptance test failed."}' $(get_env slack_web_hook_url) </dev/null
    curl -X POST -H 'Content-type: application/json' --data '{"text":"Failing pipeline: '$pipeline_url'"}' $(get_env slack_web_hook_url) </dev/null
    curl -X POST -H 'Content-type: application/json' --data '{"text":"The cluster will be retained for '$hours' hours.  If you need more time to debug ( 72 hours ):"}' $(get_env slack_web_hook_url) </dev/null
    curl -X POST -H 'Content-type: application/json' --data '{"text":"issue @ebc debug '$rco_demand_id' in #was-ebc channel to keep cluster for debug"}' $(get_env slack_web_hook_url) </dev/null
    curl -X POST -H 'Content-type: application/json' --data '{"text":"access console at: '$console'"}' $(get_env slack_web_hook_url) </dev/null
    curl -X POST -H 'Content-type: application/json' --data '{"text":"credentials: kubeadmin/'$token'"}' $(get_env slack_web_hook_url) </dev/null
fi

echo "Cleaning up after tests have be completed"
echo "switching back to scripts/pipeline directory"
cd ..
oc logout
export CLUSTER_URL=""