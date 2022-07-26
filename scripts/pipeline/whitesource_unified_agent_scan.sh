#!/usr/bin/env bash

source "${ONE_PIPELINE_PATH}/internal/tools/logging"
SCRIPT_RC=0

#
# Get required properties from the environment properties
#
WS_APIKEY=$(get_env whitesource-org-token "")
WS_USERKEY=$(get_env whitesource-user-key "")
WS_PRODUCTNAME=$(get_env whitesource-product-name "")
WS_PRODUCTTOKEN=$(get_env whitesource-product-token "")
WS_PROJECTNAME=$(get_env whitesource-project-name "")

# Check that all required properties/keys/tokens are provided
if [ -z "$WS_APIKEY" ] || [ -z "$WS_USERKEY" ] || [ -z "$WS_PRODUCTNAME" ] || [ -z "$WS_PROJECTNAME" ]; then
  error "'whitesource-org-token', 'whitesource-user-key', 'whitesource-product-name', and 'whitesource-project-name' are required properties."
  SCRIPT_RC=1
fi

# get optional properties
WS_SERVER_URL=$(get_env whitesource-server-url "https://ibmets.whitesourcesoftware.com")
WS_PRINT_SCAN_RESULTS=$(get_env whitesource-print-scan-results "")
WS_JAR_URL=$(get_env wS_jar_url "https://unified-agent.s3.amazonaws.com/wss-unified-agent.jar")

# If user overrode the whitesource server property, make sure it isn't an empty string    
if [ -z "$WS_SERVER_URL" ]; then
  error "'whitesource-server-url' cannot be empty."
  SCRIPT_RC=1
fi

if ((SCRIPT_RC==0)); then
  # Download the whitesource unified agent jar we will use to execute the scan  
  curl -LJO "$WS_JAR_URL"

  # Export environment variables required by the scanner
  export WS_APIKEY
  export WS_USERKEY
  export WS_PRODUCTNAME
  export WS_PROJECTNAME
  export WS_SERVER_URL    
  export WS_WSS_URL=${WS_SERVER_URL}/agent

  # Create the base results directory relative to workspace
  WHITESOURCE_SCAN_RESULTS_DIR=${WORKSPACE}/whitesource
  mkdir -p "$WHITESOURCE_SCAN_RESULTS_DIR"

  # Set default scan status we will pass to collect-evidence 
  SCAN_STATUS="success"
fi 

if ((SCRIPT_RC==0)); then
  # Iterate over repos that were registered to the pipeline by the save_repo of the pipelinectl tool.
  while read -r REPO ; do
    REPO_PATH="$(load_repo "${REPO}" path)"
    REPO_URL="$(load_repo "${REPO}" url)"

    # WS_PROJECTTOKEN needs to be set AFTER the jar invocation, scan will fail if both project name and project token are set. 
    unset WS_PROJECTTOKEN 

    EVIDENCE_PARAMS=(
      --tool-type "whitesource" \
      --evidence-type "com.ibm.code_vulnerability_scan" \
      --asset-type "repo" \
      --asset-key "${REPO}"
    )

    collect-evidence ${EVIDENCE_PARAMS[@]} --status "pending"

    # Execute the scan
    banner "Executing Whitesource Unified Agent scan against $REPO ($REPO_URL)"
    WHITESOURCE_SCAN_LOG="${WHITESOURCE_SCAN_RESULTS_DIR}/$REPO_PATH-ws_scan_output.log"
    SCAN_START_TIME=$SECONDS
    java -jar wss-unified-agent.jar -d "$WORKSPACE/$REPO_PATH" > "$WHITESOURCE_SCAN_LOG" 
    SCAN_RC=$?
    ELAPSED_TIME=$(( SECONDS - SCAN_START_TIME ))
    debug "   scan completed in $ELAPSED_TIME seconds"

    if ((SCAN_RC==0)); then
      #
      # Get the project token programmatically via API calls for the project name. 
      # Only do this once; once we have the project token variable set, don't execute this loop again. 
      #
      PROJECTTOKEN=""
      if [ -z "$PROJECTTOKEN" ]; then
        body="{
              \"requestType\": \"getProductProjectTags\",
              \"userKey\": \"${WS_USERKEY}\",
              \"productToken\": \"${WS_PRODUCTTOKEN}\"
              }"

        PROJECT_QUERY_RESULTS="${WHITESOURCE_SCAN_RESULTS_DIR}/projects.json"
        PROJECT_QUERY_LOG="${WHITESOURCE_SCAN_RESULTS_DIR}/projects_query.log"
        curl -X POST -H "Content-Type: application/json" -d "$body" "${WS_SERVER_URL}/api/v1.3" -o "$PROJECT_QUERY_RESULTS" >> "$PROJECT_QUERY_LOG" 2>&1

        if [ -e "$PROJECT_QUERY_RESULTS" ]; then
          NUM_PROJECT_RESULTS=$(jq '.projectTags | length' "$PROJECT_QUERY_RESULTS")
          for (( RESULTS_ROW_NUM=0; RESULTS_ROW_NUM<NUM_PROJECT_RESULTS; RESULTS_ROW_NUM++ ))
          do
            RETURNED_PROJECT_NAME=$(jq -r '.projectTags['"$RESULTS_ROW_NUM"'].name' $PROJECT_QUERY_RESULTS)
            if [[ "$RETURNED_PROJECT_NAME" == "$WS_PROJECTNAME" ]]; then
              PROJECTTOKEN=$(jq -r '.projectTags['"$RESULTS_ROW_NUM"'].token' $PROJECT_QUERY_RESULTS)
              break
            fi
          done
          
          if [ -n "$PROJECTTOKEN" ]; then
            debug "Whitesource project token for project name [$WS_PROJECTNAME] = [$PROJECTTOKEN]"  
          else
            # fail if we could not get a project token for the project name - we won't be able to query scan results
            SCRIPT_RC=1
            error "   Whitesource project token for project name [$WS_PROJECTNAME] was not found in query results"
          fi 
        else
          # PROJECT_QUERY_RESULTS json file wasn't written, the query must have failed 
          SCRIPT_RC=1
          error "   Whitesource project token query failed"
        fi
      fi

      if ((SCRIPT_RC==0)); then
        #
        # Get scan results 
        #
        WS_PROJECTTOKEN="$PROJECTTOKEN"
        body="{
            \"requestType\": \"getProjectAlerts\",
            \"userKey\": \"${WS_USERKEY}\",
            \"projectToken\": \"${WS_PROJECTTOKEN}\"
        }"
        WHITESOURCE_SCAN_RESULTS="${WHITESOURCE_SCAN_RESULTS_DIR}/$REPO_PATH-ws_scan_results.json"
        curl -X POST -H "Content-Type: application/json" -d "$body" "${WS_SERVER_URL}/api/v1.3" -o "$WHITESOURCE_SCAN_RESULTS" >> "$WHITESOURCE_SCAN_LOG" 2>&1

        if [ -e "$WHITESOURCE_SCAN_RESULTS" ]; then

          if [ -n "$WS_PRINT_SCAN_RESULTS" ]; then
            banner "=== Scan results for $REPO ($REPO_URL) ==="
            cat "$WHITESOURCE_SCAN_RESULTS" | jq
          fi

          debug "   saved scan results file $WHITESOURCE_SCAN_RESULTS"
          EVIDENCE_PARAMS+=(--attachment "${WHITESOURCE_SCAN_RESULTS}")

        else 
          SCRIPT_RC=1
          error "   Whitesource Unified Agent scan results could not be fetched"
          banner "==================== SCAN LOG ===================="
          cat "$WHITESOURCE_SCAN_LOG" 
          EVIDENCE_PARAMS+=(--attachment "${WHITESOURCE_SCAN_LOG}")
        fi
      else  
        # we were not able to query the project token  
        banner "==================== PROJECT QUERY LOG ===================="
        cat "$PROJECT_QUERY_LOG"         
        banner "==================== SCAN LOG ===================="
        cat "$WHITESOURCE_SCAN_LOG" 
        EVIDENCE_PARAMS+=(--attachment "${WHITESOURCE_SCAN_LOG}")
        EVIDENCE_PARAMS+=(--attachment "${PROJECT_QUERY_LOG}")
      fi
    else  
      # scan returned a non-zero return code 
      SCRIPT_RC=$SCAN_RC
      error "   Whitesource Unified Agent scan returned exit code $SCAN_RC"
      banner "==================== SCAN LOG ===================="
      cat "$WHITESOURCE_SCAN_LOG" 
      EVIDENCE_PARAMS+=(--attachment "${WHITESOURCE_SCAN_LOG}")
    fi
    #
    # report evidence using `collect-evidence`
    #
    
    if ((SCRIPT_RC>0)); then
      SCAN_STATUS="failure"
    fi  

    EVIDENCE_PARAMS+=(
      --status "${SCAN_STATUS}"
    )
    collect-evidence "${EVIDENCE_PARAMS[@]}"

  done < <(list_repos)  
else
    EVIDENCE_PARAMS=(
      --tool-type "whitesource" \
      --evidence-type "com.ibm.code_vulnerability_scan" \
      --status "failure"
    )
    collect-evidence "${EVIDENCE_PARAMS[@]}"
fi

if ((SCRIPT_RC>0)); then
  exit $SCRIPT_RC
fi