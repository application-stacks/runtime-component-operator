#!/bin/bash

GH_API_ROOT="https://api.github.com"
GH_BRANCH="main"
GH_REPOSITORY="websphere-liberty-operator"
GH_ORG="WASdev"
CI_TRIGGER="wlodocker"
CI_CONFIG_FILE=".ci-orchestrator/websphere-liberty-operator-build.yml"
pipelineName="Websphere Liberty Operator Docker Build"
command="make build-pipeline-releases"


function main() {
    parse_arguments "$@"
    request_ciorchestrator
}

function print_usage() {
    script_name=`basename ${0}`
    echo "Usage: ${script_name} [OPTIONS]"
    echo ""
    echo "Kick off of CI Orchestrator job"
    echo ""
    echo "Options:"
    echo "   -u, --user       string  IntranetId to use to authenticate to CI Orchestrator"
    echo "   --password       string  Intranet Password to use to authenticate to CI Orchestrator"
    echo "   -b, --branch     string  Github Repository branch"
    echo "   -r, --repository string  GitHub Repository to use"
    echo "   --org            string  Github Organisation containing repository"
    echo "   --trigger        string  Name of trigger within CI Orchestrator config file"
    echo "   --configFile     string  Location of CI Orchestrator config file"
    echo "   --command        string  Command to execute on remote machine"
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
        -u | --user)
            shift
            USER=$1
            ;;
        --password)
            shift
            PASSWORD=$1
            ;;
        -b | --branch)
            shift
            GH_BRANCH=$1
            ;;
        -r | --repository)
            shift
            GH_REPOSITORY=$1
            ;;
        --org)
            shift
            GH_ORG=$1
            ;;
        --trigger)
            shift
            CI_TRIGGER=$1
            ;;
        --configFile)
            shift
            CI_CONFIG_FILE=$1
            ;;
        --command)
            shift
            COMMAND=$1
            ;;                         
        -h | --help)
            print_usage
            exit 1
            ;;
        esac
        shift
    done
}


function request_ciorchestrator() {
    pipelineId=OnePipeline_${PIPELINE_RUN_ID}_${RANDOM}
    cat >ciorchestrator-submit.json <<EOL
    {
        "type": "PipelineTriggered",
        "ecosystemRouting": "prod",
        "pipelineId": "${pipelineId}",
        "pipelineName": "${pipelineName}",
        "triggerName": "${CI_TRIGGER}",
        "triggerType": "manual",
        "requestor": "${USER}",
        "properties": {
            "RELEASE_TARGET": "${GH_BRANCH}",
            "DISABLE_ARTIFACTORY": "${DISABLE_ARTIFACTORY}",
            "ARTIFACTORY_REPO_URL": "${ARTIFACTORY_REPO_URL}",
            "PIPELINE_OPERATOR_IMAGE": "${PIPELINE_OPERATOR_IMAGE}",
            "OPM_VERSION": "${OPM_VERSION}",
            "PIPELINE_PRODUCTION_IMAGE": "${PIPELINE_PRODUCTION_IMAGE}",
            "REDHAT_BASE_IMAGE": "${REDHAT_BASE_IMAGE}",
            "REDHAT_REGISTRY": "${REDHAT_REGISTRY}",
            "PIPELINE_REGISTRY": "${PIPELINE_REGISTRY}",
            "scriptOrg": "${GH_ORG}",
            "command": "${COMMAND}"
        },
        "configMetadata": {
            "apiRoot": "${GH_API_ROOT}",
            "org": "${GH_ORG}",
            "repo": "${GH_REPOSITORY}",
            "branch": "${GH_BRANCH}",
            "filePath": "${CI_CONFIG_FILE}"
        }
    }
EOL

    echo "${pipelineId}" >ciorchestrator-submit.id
    # add retry logic for Fyre networking issues
    echo "Sending Pipeline Request to CI Orchestrator pipelineId: ${pipelineId} as ${USER}"
    echo "command to run: $COMMAND"
    count=0
    tryAgain=true
    while $tryAgain;  do
        curl --fail --insecure -v -X POST \
            -H "Content-Type: application/json"  \
            -d @ciorchestrator-submit.json \
            -u "${USER}:${PASSWORD}" \
            https://libh-proxy1.fyre.ibm.com/eventPublish/rawCIData/${pipelineId}
            rc=$?
        if [[ $rc -eq 0 ]]; then
            echo "Successfully sent CI orchestrator Request"
            tryAgain=false
        elif [[ $count -gt 600 ]]; then
            #Bail after 10 mins
            echo "Problem sending CI orchestrator Request after 10 mins of trying, giving up.  Curl returned $rc"
            exit 1;
        else
            sleep 10
            count=$((count+10))
        fi
    done
}


# --- Run ---

main "$@"