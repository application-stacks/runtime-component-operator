#!/bin/bash

#########################################################################################
#
#           Script to run e2e tests for specified target.
#
#########################################################################################

set -Eeo pipefail

readonly usage="Usage: $0 -u <docker-username> -p <docker-password> --cluster-url <url> --cluster-token <token> --registry-name <name> --registry-namespace <namespace> --target <daily|releases|release-tag>"
readonly script_dir="$(dirname "$0")"
readonly release_blocklist="${script_dir}/release-blocklist.txt"

main() {
  parse_args "$@"

  if [[ -z "${TARGET}" ]]; then
    echo "****** Missing target release for bundle build, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${RELEASE}" ]]; then
    echo "****** Missing docker authentication information, see usage"
  fi

  if [[ -z "${DOCKER_USERNAME}" || -z "${DOCKER_PASSWORD}" ]]; then
    echo "****** Missing docker authentication information, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${CLUSTER_URL}" ]] || [[ -z "${CLUSTER_TOKEN}" ]]; then
    echo "****** Missing OCP URL or token, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${REGISTRY_NAME}" ]] || [[ -z "${REGISTRY_NAMESPACE}" ]]; then
    echo "****** Missing OCP registry name or registry namespace, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${TEST_TAG}" ]]; then
    echo "****** Missing test tag, see usage"
    echo "${usage}"
    exit 1
  fi

  # Bundle target release(s)
  if [[ "${TARGET}" != "releases" ]]; then
    run_e2e "${TARGET}"
  else
    test_releases
  fi
}

run_e2e() {
  local tag="${1}"
  local release_tag="${tag//\./}"
  "${script_dir}/e2e.sh" -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}" \
                         --cluster-url "${CLUSTER_URL}" --cluster-token "${CLUSTER_TOKEN}" \
                         --registry-name "${REGISTRY_NAME}" --registry-namespace "${REGISTRY_NAMESPACE}" \
                         --test-tag "${release_tag}-${TEST_TAG}" --release "${tag}"
}

test_releases() {
  tags="$(git tag -l)"
  while read -r tag; do
    if [[ -z "${tag}" ]]; then
      break
    fi

    # Skip any releases listed in the release blocklist
    if grep -q "^${tag}$" "${release_blocklist}"; then
      echo "Release ${tag} found in blocklist. Skipping..."
      continue
    fi

    run_e2e "${tag}"
  done <<< "${tags}"
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
    -u)
      shift
      readonly DOCKER_USERNAME="${1}"
      ;;
    -p)
      shift
      readonly DOCKER_PASSWORD="${1}"
      ;;
    --cluster-url)
      shift
      readonly CLUSTER_URL="${1}"
      ;;
    --cluster-token)
      shift
      readonly CLUSTER_TOKEN="${1}"
      ;;
    --registry-name)
      shift
      readonly REGISTRY_NAME="${1}"
      ;;
    --registry-namespace)
      shift
      readonly REGISTRY_NAMESPACE="${1}"
      ;;
    --target)
      shift
      readonly TARGET="${1}"
      ;;
    --test-tag)
      shift
      readonly TEST_TAG="${1}"
      ;;
    *)
      echo "Error: Invalid argument - $1"
      echo "$usage"
      exit 1
      ;;
    esac
    shift
  done
}

main "$@"
