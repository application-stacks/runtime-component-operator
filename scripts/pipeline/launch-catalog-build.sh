#!/bin/bash

TRAVIS_TOKEN=
LAUNCH_TRAVIS=
MONITOR_TRAVIS=yes
GH_REPO=
GH_COMMIT_ID=

function main() {
    parse_arguments "$@"
    launch_travis
}

function print_usage() {
    script_name=`basename ${0}`
    echo "Usage: ${script_name} [OPTIONS]"
    echo ""
    echo "Kick off or check status of Travis job"
    echo ""
    echo "Options:"
    echo "   -t, --token      string  Travis API token"
    echo "   -b, --branch     string  Github Repository branch"
    echo "   -l, --launch             Launch Travis job"
    echo "   -m, --monitor            Monitor Travis job"
    echo "   -r, --repository string  GitHub Repository to use"
    echo "   -c, --commit     string  GH head commit ID"
    echo "   -h, --help               Print usage information"
    echo ""
}


function parse_arguments() {
    if [[ "$#" == 0 ]]; then
        print_usage
        exit 1
    fi

    # process options
    while [[ "$1" != "" ]]; do
        case "$1" in
        -t | --token)
            shift
            TRAVIS_TOKEN=$1
            ;;
        -b | --branch)
            shift
            BRANCH=$1
            ;;    
        -l | --launch)
            LAUNCH_TRAVIS=yes
            ;;
        -m | --monitor)
            MONITOR_TRAVIS=yes
            ;;
        -r | --repository)
            shift
            GH_REPO=$1
            ;;
        -c | --commit)
            shift
            GH_COMMIT_ID=$1
            ;;
        -h | --help)
            print_usage
            exit 1
            ;;
        esac
        shift
    done
}

function launch_travis() {


    echo "Going to work with GH repository: ${GH_REPO} ..."

    # for Travis API call, the repository shall be provided without the full URL
    GH_REPO=$( echo $GH_REPO | sed -e 's/.*github.com\///g' )

    # for Travis API call, the GH repo needs to be encoded using URL encoding
    # FIXME proper URL encoding, not only handle backslashes
    GH_REPO=$( echo $GH_REPO | sed -e 's/\//%2F/g' )


    if [[ ! -z ${LAUNCH_TRAVIS} ]]; then

        body="{
            \"request\": {
            \"branch\":\"$BRANCH\",
            \"merge_mode\":\"replace\",
            \"config\": {
                \"dist\": \"focal\",
                \"language\": \"go\",
                \"go\": [
                    \"1.19.x\"
                ],
                \"go_import_path\": \"github.com/application-stacks/runtime-component-operator\",
                \"services\": [
                    \"docker\"
                ],    
                \"before_install\": [
                    \"sudo apt-get update\"
                ],    
                \"stages\": [
                    {
                        \"name\": \"build\"
                    }
                ],    
                \"jobs\": {
                    \"include\": [
                        {
                            \"stage\": \"build\",
                            \"name\": \"Build bundle on amd64\",
                            \"os\": \"linux\",
                            \"arch\": \"amd64\",
                            \"before_install\": [
                                \"sudo apt-get install -qq -y software-properties-common uidmap\",
                                \"make install-podman\",
                                \"make install-opm\"
                            ],    
                            \"script\": [
                                \"make bundle-pipeline-releases RELEASE_TARGET=$BRANCH\"
                            ]
                        }
                    ]
                }
            },
            \"sha\": \"$GH_COMMIT_ID\",
            \"message\": \"Run bundle builds\"
        }}"

        echo $body

    echo "Requesting Travis build for GH repository: ${GH_REPO}..."

    curl -s -X POST \
        -H "Content-Type: application/json" \
        -H "Accept: application/json" \
        -H "Travis-API-Version: 3" \
        -H "Authorization: token ${TRAVIS_TOKEN}" \
        -d "$body" \
        "https://api.travis-ci.com/repo/${GH_REPO}/requests" > travis-request.json

    fi

    REQUEST_NUMBER=$(jq -r '.request.id' travis-request.json)
    echo "Travis build request number: $REQUEST_NUMBER"

    echo "Checking Travis build (${REQUEST_NUMBER}) status...."

    # TODO read these in as env properties?
    retries=300
    sleep_time=30
    total_time_mins=$(( sleep_time * retries / 60))

    while true; do

        if [[ ${retries} -eq 0 ]]; then
            echo "Timeout after ${total_time_mins} minutes waiting for Travis request ${REQUEST_NUMBER} to complete."
        fi

        curl -s -X GET \
            -H "Accept: application/json" \
            -H "Travis-API-Version: 3" \
            -H "Authorization: token ${TRAVIS_TOKEN}" \
            "https://api.travis-ci.com/repo/${GH_REPO}/request/${REQUEST_NUMBER}" > travis-status-1.json

        REQUEST_STATUS=$(jq -r '.builds[].state' travis-status-1.json)
        echo "Travis request ${REQUEST_NUMBER} status: '${REQUEST_STATUS}' ..."

        if [[ "${REQUEST_STATUS}" != "failed" && "${REQUEST_STATUS}" != "passed" ]]; then # FIXME
        	retries=$(( retries - 1 ))
	        echo "Retrying waiting for Travis request ${REQUEST_NUMBER}... (${retries} left)"
	        sleep ${sleep_time}
        elif [[ "${REQUEST_STATUS}" == "failed" ]]; then
	        echo "Travis request ${REQUEST_NUMBER} failed, exiting."
            exit 1    
        else
	        echo "Travis request ${REQUEST_NUMBER} completed with status ${REQUEST_STATUS}."
	        break
        fi

    done

}


# --- Run ---

main $*
