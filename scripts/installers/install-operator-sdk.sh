#!/bin/bash

set -o errexit
set -o nounset

main() {
  DEFAULT_RELEASE_VERSION=v1.24.0
  RELEASE_VERSION=${1:-$DEFAULT_RELEASE_VERSION}

  if [[ -x "$(command -v operator-sdk)" ]]; then
    if operator-sdk version | grep -q "$RELEASE_VERSION"; then
      operator-sdk version
      exit 0
    else
      echo "****** Another operator-sdk version detected"
    fi
  fi

  ## doesn't support zLinux yet
  if [[ $(uname -p) = "s390x" ]]; then
    echo "****** zLinux build detected, skipping operator-sdk install"
    exit 0
  fi

  if [[ "$(uname)" = "Darwin" ]]; then
    binary_url="https://github.com/operator-framework/operator-sdk/releases/download/$RELEASE_VERSION/operator-sdk_darwin_amd64"
  elif [[ "$(uname -p)" = "ppc64le" ]]; then
    binary_url="https://github.com/operator-framework/operator-sdk/releases/download/$RELEASE_VERSION/operator-sdk_linux_ppc64le"
  elif [[ "$(uname -p)" = "s390x" ]]; then
    binary_url="https://github.com/operator-framework/operator-sdk/releases/download/$RELEASE_VERSION/operator-sdk_linux_s390x"
  else
    binary_url="https://github.com/operator-framework/operator-sdk/releases/download/$RELEASE_VERSION/operator-sdk_linux_amd64"
  fi

  echo "****** Installing operator-sdk version $RELEASE_VERSION on $(uname)"
  curl -L -o operator-sdk "${binary_url}"
  chmod +x operator-sdk
  sudo mv operator-sdk /usr/local/bin/operator-sdk

  operator-sdk version
}

main "$@"
