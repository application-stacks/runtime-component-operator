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

  echo "${DOCKER_PASSWORD}" | docker login -u "${DOCKER_USERNAME}" --password-stdin

  # Bundle target release(s)
  if [[ "${TARGET}" != "releases" ]]; then
    bundle_release "${TARGET}"
  else
    bundle_releases
  fi
}

bundle_release() {
  local tag="${1}"
  local release_tag="${tag#*v}"
  local operator_ref="${IMAGE}:${tag}"

  # Switch to release tag
  if [[ "${tag}" != "daily" ]]; then
    git checkout -q "${tag}"
  fi

  # Build the bundle
  local bundle_ref="${IMAGE}:bundle-${release_tag}"
  make bundle-build-podman bundle-push-podman IMG="${operator_ref}" BUNDLE_IMG="${bundle_ref}"

  # Build the catalog
  local catalog_ref="${IMAGE}:catalog-${release_tag}"
  make build-catalog push-catalog IMG="${operator_ref}" BUNDLE_IMG="${bundle_ref}" CATALOG_IMG="${catalog_ref}"
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
