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

  ## build latest master branch
  echo "****** Building release: daily"
  build_release "daily"
  echo "****** Pushing release: daily"
  push_release "daily"

  ## loop through tagged releases and build
  local tags=$(git tag -l)
  while read -r tag; do
    git checkout -q "${tag}"

    ## Remove potential leading 'v' from tags
    local dockerTag="${tag#*v}"
    echo "****** Building release: ${dockerTag}"
    build_release "${dockerTag}"
    echo "****** Pushing release: ${dockerTag}"
    push_release "${dockerTag}"
  done
}

build_release() {
  local release="$1"
  local full_image="${image}:${release}-${arch}"

  if [[ ! "${arch}" = "s390x" ]]; then
    echo "*** Building ${full_image} for ${arch}"
    operator-sdk build "${full_image}"
    return $?
  else
    ## build manually on zLinux as operator-sdk doesn't support
    ## NOTE values below must be changed to build other operators
    echo "*** Building binary of operator project"
    go build -o "$(pwd)/build/_output/bin/operator" \
      -gcflags all=-trimpath=$(pwd)/.. \
      -asmflags all=-trimpath=$(pwd)/..\
      -mod=vendor "github.com/application-runtimes/operator"

    if [[ $? -ne 0 ]]; then
      echo "Failed to build binary for zLinux, exiting"
      return 1
    fi

    echo "*** Building image: ${full_image} for ${arch}"
    docker build -f build/Dockerfile -t "${full_image}" .
    return $?
  fi
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
