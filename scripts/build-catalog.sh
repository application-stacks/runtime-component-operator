#!/bin/bash

#########################################################################################
#
#
#           Script to build the multi arch images for catalog
#           Note: Assumed to run under <operator root>/scripts
#
#
#########################################################################################

set -Eeo pipefail

OPM_TOOL="opm"
CONTAINER_TOOL="docker"

main() {
  parse_arguments "$@"
  build_catalog
}

usage() {
    script_name=`basename ${0}`
    echo "Usage: ${script_name} [OPTIONS]"
    echo "  -n, --opm-version        [REQUIRED] Version of opm (e.g. v4.5)"
    echo "  -b, --base-image         [REQUIRED] The base image that the index will be built upon (e.g. registry.redhat.io/openshift4/ose-operator-registry)"
    echo "  -t, --output             [REQUIRED] The location where the database should be output"
    echo "  -i, --image-name         [REQUIRED] The bundle image name"
    echo "  -p, --prod-image         [REQUIRED] The name of the production image the bundle should point to"
    echo "  -a, --catalog-image-name [REQUIRED] the catalog image name"
    echo "  -r, --registry           Registry to push the image to"
    echo "  -c, --container-tool     Tool to build image [docker, podman] (default 'docker')"
    echo "  -o, --opm-tool           Name of the opm tool (default 'opm')"
    echo "  -h, --help               Display this help and exit"
    echo "  -v, --current-version    Identifies the current version of this operator"
    exit 0
}


function parse_arguments() {
    if [[ "$#" == 0 ]]; then
        usage
        exit 1
    fi

    # process options
    while [[ "$1" != "" ]]; do
        case "$1" in
        -c | --container-tool)
            shift
            CONTAINER_TOOL=$1
            ;;
        -o | --opm-tool)
            shift
            OPM_TOOL=$1
            ;;   
        -n | --opm-version)
            shift
            OPM_VERSION=$1
            ;;
        -b | --base-image)
            shift
            BASE_INDEX_IMG=$1
            ;;
        -d | --directory)
            shift
            BASE_MANIFESTS_DIR=$1
            ;;
        -r | --registry)
            shift
            REGISTRY=$1
            echo "$REGISTRY"
            ;;
        -i | --image-name)
            shift
            BUNDLE_IMAGE=$1
            ;;
        -p | --prod-image)
            shift
            PROD_IMAGE=$1
            ;;
        -a | --catalog-image-name)
            shift
            CATALOG_IMAGE=$1
            ;;
        -h | --help)
            usage
            exit 1
            ;;
        -t | --output)
            shift
            TMP_DIR=$1
            ;;
        -v | --current-version)
            shift
            CURRENT_VERSION=$1
            ;;
        esac
        shift
    done
}

function build_catalog() {
    echo "------------ Start of catalog-build ----------------"
    
    ##################################################################################
    ## The catalog index build will eventually require building a bundles.db file that
    ## includes all previous versions of the operator.  For now, that is not a requirement.
    ## When the time comes that another version is released and there is a need to include
    ## multiple versions of this operator, changes will be needed in this script.  See
    ## https://github.ibm.com/websphere/automation-operator/blob/main/ci/build-operator.sh 
    ## for an example on how this is done.
    ##################################################################################

    ## Define current arch variable
    case "$(uname -p)" in
    "ppc64le")
        readonly arch="ppc64le"
        ;;
    "s390x")
        readonly arch="s390x"
        ;;
    *)
        readonly arch="amd64"
        ;;
    esac

    CATALOG_IMAGE_ARCH="${REGISTRY}/${CATALOG_IMAGE}-$arch"

    # Build catalog image
    echo "*** Building $CATALOG_IMAGE_ARCH"
  	${OPM_TOOL} index add --bundles "${REGISTRY}/${BUNDLE_IMAGE}" --tag "${CATALOG_IMAGE_ARCH}" -c docker
    if [ "$?" != "0" ]; then
        echo "Error building catalog image: $CATALOG_IMAGE_ARCH"
        exit 1
    fi 

    # Push catalog image
    make catalog-push CATALOG_IMG="${CATALOG_IMAGE_ARCH}"
    if [ "$?" != "0" ]; then
        echo "Error pushing catalog image: $CATALOG_IMAGE_ARCH"
        exit 1
    fi 

}

# --- Run ---

main $*
