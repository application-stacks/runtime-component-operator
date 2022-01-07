#!/bin/bash

#########################################################################################
#
#
#           Script to build images for all releases and daily.
#           Note: Assumed to run under <operator root>/scripts
#
#
#########################################################################################

set -Eeo pipefail

readonly usage="Usage: $0 -u <docker-username> -p <docker-password> --image [registry/]<repository>/<image> --target <daily|releases|release-tag>"
readonly script_dir="$(dirname "$0")"
readonly release_blocklist="${script_dir}/release-blocklist.txt"

main() {
  parse_args "$@"

  if [[ -z "${TARGET}" ]]; then
    echo "****** Missing target release for operator build, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${IMAGE}" ]]; then
    echo "****** Missing target image for operator build, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${DOCKER_USERNAME}" || -z "${DOCKER_PASSWORD}" ]]; then
    echo "****** Missing docker authentication information, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${REGISTRY}" ]]; then 
    echo "${DOCKER_PASSWORD}" | docker login -u "${DOCKER_USERNAME}" --password-stdin
  else
    echo "${DOCKER_PASSWORD}" | docker login "${REGISTRY}" -u "${DOCKER_USERNAME}" --password-stdin
  fi      

  # Build target release(s)
  if [[ "${TARGET}" != "releases" ]]; then
    if [[ -z "${REGISTRY}" ]]; then 
      "${script_dir}/build-release.sh" -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}" --release "${TARGET}" --image "${IMAGE}"
    else
      "${script_dir}/build-release.sh" -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}" --release "${TARGET}" --image "${IMAGE}" --registry "${REGISTRY}"
    fi  
  else
    build_releases
  fi
}

build_releases() {
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

    "${script_dir}/build-release.sh" -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}" --release "${tag}" --image "${IMAGE}"
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
    --registry)
      shift
      readonly REGISTRY="${1}"
      ;;  
    --image)
      shift
      readonly IMAGE="${1}"
      ;;
    --target)
      shift
      readonly TARGET="${1}"
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
