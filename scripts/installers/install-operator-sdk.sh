#!/bin/bash

set -o errexit
set -o nounset

main() {
  if [[ -x "$(command -v operator-sdk)" ]]; then
    operator-sdk version
    exit 0
  fi

  ## doesn't support zLinux yet
  if [[ $(uname -p) = "s390x" ]]; then
    echo "****** zLinux build detected, skipping operator-sdk install"
    exit 0
  fi

  DEFAULT_RELEASE_VERSION=v0.15.2
  RELEASE_VERSION=${1:-$DEFAULT_RELEASE_VERSION}

  if [[ "$(uname)" = "Darwin" ]]; then
    binary_url="https://github.com/operator-framework/operator-sdk/releases/download/$RELEASE_VERSION/operator-sdk-$RELEASE_VERSION-x86_64-apple-darwin"
  elif [[ "$(uname -p)" = "ppc64le" ]]; then
    binary_url="https://github.com/operator-framework/operator-sdk/releases/download/$RELEASE_VERSION/operator-sdk-$RELEASE_VERSION-ppc64le-linux-gnu"
  else
    binary_url="https://github.com/operator-framework/operator-sdk/releases/download/$RELEASE_VERSION/operator-sdk-$RELEASE_VERSION-x86_64-linux-gnu"
  fi

  echo "****** Installing operator-sdk version $RELEASE_VERSION on $(uname)"
  curl -L -o operator-sdk $binary_url
  chmod +x operator-sdk
  sudo mv operator-sdk /usr/local/bin/operator-sdk

  operator-sdk version
}

main $@
