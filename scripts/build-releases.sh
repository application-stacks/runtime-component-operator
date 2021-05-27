#!/bin/bash

#########################################################################################
#
#
#           Script to build and push the multi arch images for operator
#           Note: Assumed to run under <operator root>/scripts
#
#
#########################################################################################

set -Eeo pipefail

readonly usage="Usage: build-release.sh -u <docker-username> -p <docker-password> --image repository/image"

main() {
  parse_args $@

  if [[ -z "${USER}" || -z "${PASS}" ]]; then
    echo "****** Missing docker authentication information, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${IMAGE}" ]]; then
    echo "****** Missing target image for operator build, see usage"
    echo "${usage}"
    exit 1
  fi

  ## Define current arc variable
  case "$(uname -p)" in
  "ppc64le")
    readonly arch="ppc64le"
    ;;
  "s390x")
    readonly arch="s390x"
    ;;
  *)
    readonly arch="amd64"
    ;;
  esac

  ## login to docker
  echo "${PASS}" | docker login -u "${USER}" --password-stdin

  ## build latest master branch
  echo "****** Building release: daily"
  build_release "daily"
  echo "****** Pushing release: daily"
  push_release "daily"

  ## loop through tagged releases and build
  local tags=$(git tag -l)
  while read -r tag; do
    if [[ -z "${tag}" ]]; then
      break
    fi

    git checkout -q "${tag}"

    ## Remove potential leading 'v' from tags
    local dockerTag="${tag#*v}"
    echo "****** Building release: ${dockerTag}"
    build_release "${dockerTag}"
    echo "****** Pushing release: ${dockerTag}"
    push_release "${dockerTag}"
  done <<< "${tags}"
}

build_release() {
  local release="$1"
  local full_image="${IMAGE}:${release}-${arch}"
  echo "*** Building ${full_image} for ${arch}"
  docker build -t "${full_image}"
  return $?

}

push_release() {
  local release="$1"

  if [[ "${TRAVIS}" = "true" && "${TRAVIS_PULL_REQUEST}" = "false" && "${TRAVIS_BRANCH}" = "master" ]]; then
    echo "****** Pushing image: ${IMAGE}:${release}-${arch}"
    docker push "${IMAGE}:${release}-${arch}"
  else
    echo "****** Skipping push for branch ${TRAVIS_BRANCH}"
  fi
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
