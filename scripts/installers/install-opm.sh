#!/bin/bash

set -o errexit
set -o nounset

main() {
  if [[ -x "$(command -v opm)" ]]; then
    opm version
    exit 0
  fi

  readonly DEFAULT_RELEASE_VERSION=latest-4.10
  readonly RELEASE_VERSION=${1:-$DEFAULT_RELEASE_VERSION}
  readonly base_url="https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/${RELEASE_VERSION}"

  if [[ "$(uname)" = "Darwin" ]]; then
    binary_url="${base_url}/opm-mac.tar.gz"
  elif [[ "$(uname)" = "Linux" ]]; then
    binary_url="${base_url}/opm-linux.tar.gz"
  else
    echo "****** opm installer only supports Linux and macOS systems. Skipping..."
    exit 0
  fi

  echo "****** Installing opm version ${RELEASE_VERSION} on $(uname)"
  curl -L "${binary_url}" | tar xvz
  chmod +x opm
  sudo mv opm /usr/local/bin/opm

  opm version
}

main "$@"
