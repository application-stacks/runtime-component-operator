#!/bin/bash

#########################################################################################
#
#
#           Script to bundle the multi arch images for operator
#           To skip pushing the image to the container registry, provide the `--skip-push` flag.
#           Note: Assumed to run under <operator root>/scripts
#
#
#########################################################################################

set -Eeo pipefail

readonly usage="Usage: bundle-release.sh -u <docker-username> -p <docker-password> --image repository/image --prod-image prod-repository/image --release <release> [--skip-push]"

main() {
  parse_args "$@"

  if [[ -z "${IMAGE}" ]]; then
    echo "****** Missing target image for operator build, see usage"
    echo "${usage}"
    exit 1
  fi

  if [[ -z "${PROD_IMAGE}" ]]; then
    echo "****** Missing production image reference for bundle, see usage"
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

  # Remove 'v' prefix from any releases matching version regex `\d+\.\d+\.\d+.*`
  if [[ "${RELEASE}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
    readonly release_tag="${RELEASE#*v}"
  else
    readonly release_tag="${RELEASE}"
  fi

  readonly digest="$(skopeo inspect docker://$IMAGE:${release_tag}-${arch} | grep Digest | grep -o 'sha[^\"]*')"
  readonly full_image="${PROD_IMAGE}@${digest}"
  readonly bundle_image="${IMAGE}-bundle:${release_tag}"

  ## login to docker
  if [[ -z "${REGISTRY}" ]]; then 
    echo "${DOCKER_PASSWORD}" | docker login -u "${DOCKER_USERNAME}" --password-stdin
  else
    echo "${DOCKER_PASSWORD}" | docker login "${REGISTRY}" -u "${DOCKER_USERNAME}" --password-stdin
  fi       

  echo "****** Bundling release: ${RELEASE}"
  bundle_release "${RELEASE}"

  if [[ "${SKIP_PUSH}" != true ]]; then
    echo "****** Pushing bundle: ${RELEASE}"
    push_bundle
  else
    echo "****** Skipping push for bundle ${RELEASE}"
  fi
}

bundle_release() {
  echo "*** Bundling ${full_image} for ${arch}.  Bundle location will be ${bundle_image}."

  make bundle bundle-build IMG="${full_image}" BUNDLE_IMG="${bundle_image}"

  return $?
}

push_bundle() {
  echo "****** Pushing bundle: ${bundle_image}"
  make bundle-push IMG="${full_image}" BUNDLE_IMG="${bundle_image}"
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
    --prod-image)
      shift
      readonly PROD_IMAGE="${1}"
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