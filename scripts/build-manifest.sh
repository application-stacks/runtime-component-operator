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

readonly usage="Usage: $0 -u <docker-username> -p <docker-password> --image [registry/]repository/image"
readonly script_dir="$(dirname "$0")"
readonly release_blocklist="${script_dir}/release-blocklist.txt"

main() {
  parse_args "$@"

  if [[ -z "${TARGET}" ]]; then
    echo "****** Missing target release for operator manifest lists, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${DOCKER_USERNAME}" || -z "${DOCKER_PASSWORD}" ]]; then
    echo "****** Missing docker authentication information, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${IMAGE}" ]]; then
    echo "****** Missing target image for operator manifest lists, see usage"
    echo "${usage}"
    exit 1
  fi

  echo "${DOCKER_PASSWORD}" | docker login -u "${DOCKER_USERNAME}" --password-stdin

  # Build manifest for target release(s)
  if [[ "${TARGET}" != "releases" ]]; then
    # Remove 'v' prefix from any releases matching version regex `\d+\.\d+\.\d+.*`
    if [[ "${TARGET}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
      readonly release_tag="${TARGET#*v}"
    else
      readonly release_tag="${TARGET}"
    fi
    build_manifest "${release_tag}"
  else
    build_manifests
  fi
}

build_manifest() {
  local tag="$1"
  echo "****** Building manifest for: ${tag}"

  ## try to build manifest but allow failure
  ## this allows new release builds
  local target="${IMAGE}:${tag}"
  manifest-tool push from-args \
    --platforms "linux/amd64,linux/s390x,linux/ppc64le" \
    --template "${target}-ARCH" \
    --target "${target}" \
    || echo "*** WARN: Target architectures not available"
}

# Build manifest for previous releases
build_manifests() {
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

    local release_tag="${tag#*v}"
    build_manifest "${release_tag}"
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
