#!/bin/bash

#########################################################################################
#
#
#           Build manifest list for all releases of operator repository/image
#           Note: Assumed to run under <operator root>/scripts
#
#
#########################################################################################

set -Eeo pipefail

readonly usage="Usage: build-manifest.sh -u <docker-username> -p <docker-password> --image repository/image"

main() {
  parse_args $@

  if [[ -z "${USER}" || -z "${PASS}" ]]; then
    echo "****** Missing docker authentication information, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${IMAGE}" ]]; then
    echo "****** Missing target image for operator manifest lists, see usage"
    echo "${usage}"
    exit 1
  fi

  echo "${PASS}" | docker login -u "${USER}" --password-stdin

  echo "****** Building manifest for: daily"
  build_manifest "daily"

  local tags=$(git tag -l)
  while read -r tag; do
    if [[ -z "${tag}" ]]; then
      break
    fi

    echo "****** Building manifest list for: ${tag}"
    build_manifest "${tag}"
  done <<< "${tags}"
}

build_manifest() {
  local tag="$1"

  ## try to build manifest but allow failure
  ## this allows new release builds
  local target="${IMAGE}:${tag}"
  manifest-tool push from-args \
    --platforms "linux/amd64,linux/s390x,linux/ppc64le" \
    --template "${target}-ARCH" \
    --target "${target}" \
    || echo "*** WARN: Target archs not available"
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

main $@
