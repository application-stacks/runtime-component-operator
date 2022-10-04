#!/usr/bin/env bash

set -e

usage() {
  echo "usage: $0 --cluster-name <cluster-name-prefix> --user <fyre-user> --key <fyre-key>"
  exit 1
}

# PRODUCT_GROUP_ID for 'WAS - ALL' -> 52

main() {
  parse_args "${@}"

  CURL_OPTS="-s -k -u ${FYRE_USER}:${FYRE_KEY}"

  # Build the single-VM cluster
  echo "Sending delete request to Fyre..."
  curl ${CURL_OPTS} -X POST 'https://api.fyre.ibm.com/rest/v1/?operation=delete' --data "{\"cluster_name\":\"${CLUSTER_NAME}\"}" > /dev/null
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
     [[ -z "${FYRE_KEY}" ]]; then
    usage
  fi
}

main "${@}"
