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
    echo "****** Missing target release for bundle build, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${IMAGE}" ]]; then
    echo "****** Missing target image for bundle build, see usage"
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

  # Bundle target release(s)
  if [[ "${TARGET}" != "releases" ]]; then
    bundle_release "${TARGET}"
  else
    bundle_releases
  fi
}

bundle_release() {
  local tag="${1}"
  # Remove 'v' prefix from any releases matching version regex `\d+\.\d+\.\d+.*`
  if [[ "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
    local release_tag="${tag#*v}"
  else
    local release_tag="${tag}"
  fi
  local operator_ref="${IMAGE}:${release_tag}"

  # Switch to release tag
  if [[ "${tag}" != "daily" ]]; then
    git checkout -q "${tag}"
  fi

  # Build the bundle
  local bundle_ref="${IMAGE}:bundle-${release_tag}"
  make kustomize bundle bundle-build bundle-push IMG="${operator_ref}" BUNDLE_IMG="${bundle_ref}"

  # Build the catalog
  local catalog_ref="${IMAGE}:catalog-${release_tag}"
  if [[ -z "${REGISTRY}" ]]; then 
    make build-catalog push-catalog IMG="${operator_ref}" BUNDLE_IMG="${bundle_ref}" CATALOG_IMG="${catalog_ref}"
  else
    make build-catalog push-pipeline-catalog IMG="${operator_ref}" BUNDLE_IMG="${bundle_ref}" CATALOG_IMG="${catalog_ref}"    
  fi  
}

bundle_releases() {
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

    bundle_release "${tag}"
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
