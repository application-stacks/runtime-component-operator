#!/usr/bin/env bash

set -e -o pipefail

usage() {
  echo "usage: $0 --cluster-name <cluster-name-prefix> --user <fyre-user> --key <fyre-key> --product-group-id <deployment-product-id>"
  echo
  echo "OPTIONAL ARGUMENTS"
  echo "  --os-name <os-name>                 Desired OS. Defaults to 'Ubuntu 22.04'"
  echo "  --size <s|m|l|x>                    Valid options: s (2CPU, 2GB, 250GB), m (2CPU, 4GB, 250GB), l (4CPU, 8GB, 250GB), or x (8CPU, 16GB, 250GB). Defaults to 's'"
  echo "  --count <node-count>                Desired number of nodes. Defaults to '1'"
  echo "  --init-script <init-script-path>    Init script to configure new node(s). Requires --pass <fyre-pass> to be set"
  echo "  --pass <fyre-pass>                  Fyre password to ssh onto VM"
  exit 1
}

# PRODUCT_GROUP_ID for 'WAS - ALL' -> 52

main() {
  parse_args "${@}"

  BUILD_DATA="""{
    \"type\" : \"simple\",
    \"cluster_prefix\" :\"${CLUSTER_NAME}\",
    \"instance_type\" : \"virtual_server\",
    \"size\" : \"${VM_SIZE}\",
    \"platform\": \"x\",
    \"os\" : \"${VM_OS_NAME}\",
    \"count\" : \"${VM_COUNT}\",
    \"product_group_id\": \"${PRODUCT_GROUP_ID}\"
  }"""

  BUILD_TIME_START=${SECONDS}
  CURL_OPTS="-s -k -u ${FYRE_USER}:${FYRE_KEY}"

  # Avoid creating cluster if it already exists
  BUILD_REQUEST_STATUS="$(curl ${CURL_OPTS} "https://api.fyre.ibm.com/rest/v1/?operation=query&request=showclusters" | jq -r --arg cluster_name "${CLUSTER_NAME}" '.clusters[] | select(.name == $cluster_name) | .status')"
  if [[ -z "${BUILD_REQUEST_STATUS}" ]]; then
    # Build the single-VM cluster
    echo "Sending build request to Fyre..."
    echo "${BUILD_DATA}"
    echo
    BUILD_REQUEST_URL="$(curl ${CURL_OPTS} -X POST 'https://api.fyre.ibm.com/rest/v1/?operation=build' --data "${BUILD_DATA}" | jq '.details' | sed 's/"//g')"
    BUILD_REQUEST_INFO="$(curl ${CURL_OPTS} ${BUILD_REQUEST_URL})"
    BUILD_REQUEST_STATUS="$(echo ${BUILD_REQUEST_INFO} | jq -r '.request[0].status')"
    if [[ "${BUILD_REQUEST_STATUS}" == "error" ]]; then
      echo "Build request failed:"
      echo "${BUILD_REQUEST_INFO}" | jq -r '.request[0].error_details'
      exit 1
    fi

    BUILD_COMPLETE_PERCENT="$(curl ${CURL_OPTS} "https://fyre.ibm.com/embers/checkbuild?cluster=${CLUSTER_NAME}")"

    until [ ${BUILD_COMPLETE_PERCENT} -eq 100 ]
    do
        BUILD_COMPLETE_PERCENT="$(curl ${CURL_OPTS} "https://fyre.ibm.com/embers/checkbuild?cluster=${CLUSTER_NAME}")"
        BUILD_REQUEST_STATUS="$(curl ${CURL_OPTS} "https://api.fyre.ibm.com/rest/v1/?operation=query&request=showclusters" | jq --arg cluster_name "${CLUSTER_NAME}" '.clusters[] | select(.name == $cluster_name) | .status' | sed 's/"//g')"
        echo "VM status is ${BUILD_REQUEST_STATUS} (${BUILD_COMPLETE_PERCENT}%)"
        sleep 5
    done
  fi

  if [[ -n "${INIT_SCRIPT}" ]]; then
    for i in $(seq 1 "${VM_COUNT}"); do
      local cluster_instance="${CLUSTER_NAME}${i}"
      sshpass -p "${FYRE_PASS}" ssh -o LogLevel=ERROR -o StrictHostKeyChecking=no "root@${cluster_instance}.fyre.ibm.com" 'bash -s' < "${INIT_SCRIPT}"
    done
  fi

  BUILD_TIME_END=${SECONDS}
  BUILD_TIME_DIFF=$((BUILD_TIME_END-STBUILD_TIME_STARTART))
  BUILD_TIME_MINUTES=$((BUILD_TIME_DIFF/60))
  BUILD_TIME_COUNTED_MINUTES=$((BUILD_TIME_MINUTES*60))
  BUILD_TIME_SECONDS=$((BUILD_TIME_DIFF-BUILD_TIME_COUNTED_MINUTES))

  echo "Your VM was built in ${BUILD_TIME_MINUTES} minutes, ${BUILD_TIME_SECONDS} seconds."
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "${1}" in
    -n|--cluster-name)
      shift
      readonly CLUSTER_NAME="${1}"
      ;;
    -u|--user)
      shift
      readonly FYRE_USER="${1}"
      ;;
    -k|--key)
      shift
      readonly FYRE_KEY="${1}"
      ;;
    -p|--pass)
      shift
      readonly FYRE_PASS="${1}"
      ;;
    -pgid|--product-group-id)
      shift
      readonly PRODUCT_GROUP_ID="${1}"
      ;;
    -os|--os-name)
      shift
      readonly VM_OS_NAME="${1}"
      ;;
    -s|--size)
      shift
      readonly VM_SIZE="${1}"
      ;;
    -c|--count)
      shift
      readonly VM_COUNT="${1}"
      ;;
    -i|--init-script)
      shift
      readonly INIT_SCRIPT="${1}"
      ;;
    *)
      echo "Error: Invalid argument - ${1}"
      echo "$usage"
      exit 1
      ;;
    esac
    shift
  done

  if [[ -z "${CLUSTER_NAME}" ]] ||
     [[ -z "${FYRE_USER}" ]] ||
     [[ -z "${FYRE_KEY}" ]] ||
     [[ -z "${PRODUCT_GROUP_ID}" ]]; then
    usage
  fi

  if [[ -n "${INIT_SCRIPT}" ]] && [[ -z "${FYRE_PASS}" ]]; then
    echo "Error: Fyre password required with --init-script"
    usage
  fi

  if [[ -n "${INIT_SCRIPT}" ]] && [[ ! -f "${INIT_SCRIPT}" ]]; then
    echo "Error: Init script does not exist"
    usage
  fi

  if [[ -z "${VM_OS_NAME}" ]]; then
    readonly VM_OS_NAME="Ubuntu 22.04"
  fi

  if [[ -z "${VM_SIZE}" ]]; then
    readonly VM_SIZE="m"
  elif [[ "${VM_SIZE}" != "s" ]] && [[ "${VM_SIZE}" != "m" ]] && [[ "${VM_SIZE}" != "l" ]] && [[ "${VM_SIZE}" != "x" ]]; then
    echo "Error: Expected size of 's', 'm', 'l', or 'x'; got '${VM_SIZE}'"
    usage
  fi

  if [[ -z "${VM_COUNT}" ]]; then
    readonly VM_COUNT="1"
  elif [[ "${VM_COUNT}" -lt 1 ]] || [[ "${VM_COUNT}" -gt 100 ]]; then
    echo "Error: VM count must be between [1, 100]; got ${VM_COUNT}"
    usage
  fi
}

main "${@}"
