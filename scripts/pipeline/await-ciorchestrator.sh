#!/bin/bash

function main() {
    parse_arguments "$@"
    await_ciorchestrator
}

function print_usage() {
    script_name=`basename ${0}`
    echo "Usage: ${script_name} [OPTIONS]"
    echo ""
    echo "Await Completion of CI Orchestrator job"
    echo ""
    echo "Options:"
    echo "   -u, --user       string  IntranetId to use to authenticate to CI Orchestrator"
    echo "   --password       string  Intranet Password to use to authenticate to CI Orchestrator"
    echo "   --pipelineId     string  pipelineId of the request that should be awaited"
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
        --pipelineId)
            shift
            pipelineId=$1
            ;;               
        -h | --help)
            print_usage
            exit 1
            ;;
        esac
        shift
    done
}

function await_ciorchestrator() {
    echo "Checking Pipeline Request in CI Orchestrator as ${USER}, pipelineId: ${pipelineId}"

    cat >ciorchestrator-query.json <<EOL
{
    "targetType": "PipelineSummarised", 
    "filters": [
        {
            "field": "pipelineId",
            "operation": "EQUALS",
            "value": "${pipelineId}"
        }
    ],
    "resultProps": [
        "progress",
        "health"
    ],
    "excludeHeaders": true
}
EOL

    while [[ true ]]
    do
        check_request
        cat ciorchestrator-query-output.csv | grep -E "COMPLETED|FATAL|CANCELLED" >/dev/null
        rc=$?
        if [ $rc -eq 0 ]; then
            echo "CIOrchestrator Pipeline finished"
            cat ciorchestrator-query-output.csv | grep -E "OK" >/dev/null
            ok=$?
            echo "Exiting $ok"
            exit $ok
        else
            sleep 1m
        fi
    done
}

function check_request(){
    curl -s -X POST \
       --insecure \
       -H "Content-Type: application/json"  \
       -d @ciorchestrator-query.json \
       -u "${USER}:${PASSWORD}" \
       -o ciorchestrator-query-output.csv \
       https://libh-proxy1.fyre.ibm.com/ci-pipeline-work-views-stateStore/query

    cat ciorchestrator-query-output.csv

}


# --- Run ---

main $*