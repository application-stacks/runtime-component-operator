#!/bin/bash

#########################################################################################
#
#
#           Script to build the multi arch images for operator
#           To skip pushing the image to the container registry, provide the `--skip-push` flag.
#           Note: Assumed to run under <operator root>/scripts
#
#
#########################################################################################

set -Eeo pipefail

readonly usage="Usage: build-release.sh -u <docker-username> -p <docker-password> --image repository/image --release <release> [--skip-push]"

main() {
  parse_args "$@"

  if [[ -z "${IMAGE}" ]]; then
    echo "****** Missing target image for operator build, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${RELEASE}" ]]; then
    echo "****** Missing release for operator build, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${DOCKER_USERNAME}" || -z "${DOCKER_PASSWORD}" ]]; then
    echo "****** Missing docker authentication information, see usage"
    echo "${usage}"
    exit 1
  fi

  ## Define current arch variable
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

  readonly full_image="${IMAGE}:${RELEASE}-${arch}"

  ## login to docker
  echo "${DOCKER_PASSWORD}" | docker login -u "${DOCKER_USERNAME}" --password-stdin

  ## build or push latest master branch
  echo "****** Building release: ${RELEASE}"
  build_release "${RELEASE}"

  if [[ "${SKIP_PUSH}" != true ]]; then
    echo "****** Pushing release: ${RELEASE}"
    push_release
  else
    echo "****** Skipping push for release ${RELEASE}"
  fi
}

build_release() {
  local release="$1"
  echo "*** Building ${full_image} for ${arch}"

  if [[ "${release}" != "daily" ]]; then
    git checkout -q "v${release}"
  fi

  docker build -t "${full_image}" .
  return $?
}

push_release() {
  if [[ "${TRAVIS}" = "true" && "${TRAVIS_PULL_REQUEST}" = "false" && "${TRAVIS_BRANCH}" = "master" ]]; then
    echo "****** Pushing image: ${full_image}"
    docker push "${full_image}"
  else
    echo "****** Skipping push for branch ${TRAVIS_BRANCH}"
  fi
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
    --skip-push)
      readonly SKIP_PUSH=true
      ;;
    --release)
      shift
      readonly RELEASE="${1}"
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
