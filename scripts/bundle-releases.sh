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

readonly usage="Usage: $0 -u <docker-username> -p <docker-password> --image [registry/]<repository>/<image>"
readonly script_dir="$(dirname "$0")"
readonly release_blocklist="${script_dir}/release-blocklist.txt"

main() {
  parse_args "$@"

  if [[ -z "${IMAGE}" ]]; then
    echo "****** Missing target image for bundle build, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${USER}" || -z "${PASS}" ]]; then
    echo "****** Missing docker authentication information, see usage"
    echo "${usage}"
    exit 1
  fi

  echo "${PASS}" | docker login -u "${USER}" --password-stdin

  # Bundle and create index for daily
  bundle_release "daily"

  # Bundle and create index for previous releases
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

bundle_release() {
  local tag="${1}"
  local release_tag="${tag#*v}"
  local operator_ref="${IMAGE}:${tag}"

  # Switch to release tag
  if [[ "${tag}" != "daily" ]]; then
    git switch -q "${tag}"
  fi

  # Build the bundle
  local bundle_ref="${IMAGE}:bundle-${release_tag}"
  make bundle-build-podman bundle-push-podman IMG="${operator_ref}" BUNDLE_IMG="${bundle_ref}"

  # Build the catalog
  local catalog_ref="${IMAGE}:catalog-${release_tag}"
  make build-catalog push-catalog IMG="${operator_ref}" BUNDLE_IMG="${bundle_ref}" CATALOG_IMG="${catalog_ref}"
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
    -u)
      shift
      readonly USER="${1}"
      ;;
    -p)
      shift
      readonly PASS="${1}"
      ;;
    --image)
      shift
      readonly IMAGE="${1}"
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
