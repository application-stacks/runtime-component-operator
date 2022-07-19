#!/usr/bin/env bash

function evaluate() {
  name=$1
  expected_status=$2
  actual_status=$3
  task_name=$4
  skip_task=$5

  if [[ $skip_task == 'true' ]]; then
    echo "Task '${name}' has been skipped."
    echo "Actual: '${actual_status}' | Expected: '${expected_status}'."
  elif [ "$expected_status" != "$actual_status" ]; then
    echo "Task '${name}' has failed"
    echo "The actual result value did not match expected value."
    echo "Actual: '${actual_status}' | Expected: '${expected_status}'."
    export PIPELINE_EXIT+=1
  else
    export PIPELINE_EXIT+=0
  fi  
}  

printf "\n\nEvaluating Pipeline Task results \n\n" >&2

evaluate \
  "detect-secrets"                                   `# name` \
  "success"                                          `# expected_status` \
  "$(get_env DETECT_SECRETS_STATUS)"                 `# actual_status` \
  "$(get_env DETECT_SECRET_TASK_NAME)"               `# task_name` 

evaluate \
  "unit-tests"                                       `# name` \
  "success"                                          `# expected_status` \
  "$(get_env STAGE_TEST_STATUS)"                     `# actual_status` \
  "$(get_env UNIT_TESTS_TASK_NAME)"                  `# task_name` 

enable_sonar=$(get_env opt-in-sonar "")
enable_gosec=$(get_env opt-in-gosec "")
if [[ -n $enable_sonar || -n $enable_gosec ]]; then
  evaluate \
    "static-scan"                                    `# name` \
    "success"                                        `# expected_status` \
    "$(get_env STAGE_STATIC_SCAN_STATUS)"            `# actual_status` \
    "$(get_env STATIC_SCAN_TASK_NAME)"               `# task_name` 
fi

evaluate \
  "vulnerability-scan"                               `# name` \
  "success"                                          `# expected_status` \
  "$(get_env CRA_VULNERABILITY_RESULTS_STATUS)"      `# actual_status` \
  "$(get_env CRA_VULNERABILITY_TASK_NAME)"           `# task_name` 

evaluate \
  "cis-check"                                        `# name` \
  "success"                                          `# expected_status` \
  "$(get_env CIS_CHECK_VULNERABILITY_RESULTS_STATUS)"`# actual_status` \
  "$(get_env CIS_CHECK_TASK_NAME)"                   `# task_name`                

evaluate \
  "bom-check"                                        `# name` \
  "success"                                          `# expected_status` \
  "$(get_env CRA_BOM_CHECK_RESULTS_STATUS)"          `# actual_status` \
  "$(get_env BOM_CHECK_TASK_NAME)"                   `# task_name` 

evaluate \
  "branch-protection"                                `# name` \
  "success"                                          `# expected_status` \
  "$(get_env BRANCH_PROTECTION_STATUS)"              `# actual_status` \
  "$(get_env BRANCH_PROTECTION_TASK_NAME)"           `# task_name` \
  "$(get_env SKIP_BRANCH_PROTECTION 'false')"           

evaluate \
  "vulnerability-advisor"                            `# name` \
  "success"                                          `# expected_status` \
  "$(get_env STAGE_SCAN_ARTIFACT_STATUS)"            `# actual_status` \
  "$(get_env VULNERABILITY_ADVISOR_TASK_NAME)"       `# task_name` \
  "$(get_env SKIP_VA 'false')"      

evaluate \
  "acceptance-tests"                                 `# name` \
  "success"                                          `# expected_status` \
  "$(get_env STAGE_ACCEPTANCE_TEST_STATUS)"          `# actual_status` \
  "$(get_env ACCEPTANCE_TESTS_TASK_NAME)"            `# task_name` 

if [[ "$PIPELINE_EXIT" == *"1"* ]]; then
  exit 1
else
  echo "Every Task result passed the check!"
  exit 0
fi
