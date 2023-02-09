#!/bin/bash

OPM_TOOL="opm"
CONTAINER_TOOL="docker"

## Variables used for determing which older versions to include in catalog
declare -a arrVersions
declare -a arrExcludeVersions
excludeVersionsFile=catalogVersionExclude.json

function main() {
    parse_arguments "$@"
    determineAndPullOlderBundles
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

function create_empty_db() {
    mkdir -p "${TMP_DIR}/manifests"
    echo "------------ creating an empty bundles.db ---------------"
    ${CONTAINER_TOOL} run --rm -v "${TMP_DIR}":/tmp --entrypoint "/bin/initializer" "${BASE_INDEX_IMG}:${OPM_VERSION}" -m /tmp/manifests -o /tmp/bundles.db
}

function add_historical_versions_to_db(){
    for imageTag in "${arrVersions[@]}"
    do
        local digest="$(docker pull $PROD_IMAGE:$imageTag | grep Digest | grep -o 'sha[^\"]*')"
        local img_digest="${PROD_IMAGE}@${digest}"
        echo "------------ adding bundle image ${img_digest} to ${TMP_DIR}/bundles.db ------------"
        "${OPM_TOOL}" registry add -b "${img_digest}" -d "${TMP_DIR}/bundles.db" -c "${CONTAINER_TOOL}" --permissive
    done
}

function add_current_version_to_db(){
    local stg_img=$1
    local prod_img=$2
    local digest="$(docker pull $stg_img | grep Digest | grep -o 'sha[^\"]*')"
    local img_digest="${prod_img}@${digest}"
    echo "------------ adding bundle image ${img_digest} to ${TMP_DIR}/bundles.db ------------"
    "${OPM_TOOL}" registry add -b "${img_digest}" -d "${TMP_DIR}/bundles.db" -c "${CONTAINER_TOOL}" --permissive
}

function createExcludeVersionsArray() {
    excludeCount=$(jq '.ExcludeTags | length' $1)
    for (( excludeIdx=0; excludeIdx<excludeCount; excludeIdx++ ));
    do
        jqQuery=".ExcludeTags[${excludeIdx}]"
        temp=$(cat $1 | jq $jqQuery)
        temp="${temp%\"}"
        excludeVersion="${temp#\"}"
        arrExcludeVersions[${#arrExcludeVersions[@]}]=$excludeVersion
    done

    ## Add current version to excludes array as well.  We do this because it is possible the version
    ## being built is already in production.  If we don't exclude the current version, the production
    ## version will be added to the catalog prior to the current build's bundle.  That will exclude the
    ## current build from being included into the catalog.  So, instead, we omit the production version
    ## so that the current build is added to the catalog.
    arrExcludeVersions[${#arrExcludeVersions[@]}]=$CURRENT_VERSION
}

function determineAndPullOlderBundles() {
    createExcludeVersionsArray $excludeVersionsFile
    imageInspection=$(skopeo list-tags docker://$PROD_IMAGE)
    echo "Image Inspection Results:"
    echo $imageInspection

    ## Count version tags in image inspection file
    versionCount=$(echo $imageInspection | jq '.Tags | length')
    echo "Found ${versionCount} image tags"

    ## Iterate image tags
    for (( versionIdx=0; versionIdx<versionCount; versionIdx++ ));
    do
        jqQuery=".Tags[${versionIdx}]"
        temp=$(echo $imageInspection | jq $jqQuery)
        temp="${temp%\"}"
        version="${temp#\"}"

        ## Check to see if we need to exclude this version
        index=-1
        for i in "${!arrExcludeVersions[@]}";
        do
            if [[ "${arrExcludeVersions[$i]}" = "${version}" ]];
            then
                index=$i
                break
            fi
        done
    
        ## If version is not in excludes list, add it to the versions array
        if [ $index -lt 0 ]; then
            echo "Including tag '${version}'"
            arrVersions[${#arrVersions[@]}]=$version
        else
            echo "Excluding tag '${version}'"
        fi
    done
}


function build_catalog() {
    echo "------------ Start of catalog-build ----------------"

    mkdir -p "${TMP_DIR}"
    chmod 777 "${TMP_DIR}"
    
    ##################################################################################
    ## The catalog index build will eventually require building a bundles.db file that
    ## includes all previous versions of the operator.  For now, that is not a requirement.
    ## When the time comes that another version is released and there is a need to include
    ## multiple versions of this operator, changes will be needed in this script.  See
    ## https://github.ibm.com/websphere/automation-operator/blob/main/ci/build-operator.sh 
    ## for an example on how this is done.
    ##################################################################################

    echo "Building Catalog Index Database..."

    create_empty_db
    add_historical_versions_to_db
    add_current_version_to_db "${BUNDLE_IMAGE}" "${PROD_IMAGE}"

    # Copy bundles.db local prior to building the image
    cp "${TMP_DIR}/bundles.db" .

    # Build catalog image 
    "${CONTAINER_TOOL}" build -t "${CATALOG_IMAGE}" -f index.Dockerfile . 
}


# --- Run ---

main $*