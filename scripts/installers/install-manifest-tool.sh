#!/bin/bash

set -o errexit
set -o nounset

main() {
  if [[ -x "$(command -v manifest-tool)" ]]; then
    manifest-tool version
    exit 0
  fi

  DEFAULT_RELEASE=v0.9.0
  RELEASE_VERSION=${1:-$DEFAULT_RELEASE}

  echo "****** Installing manifest-tool version ${RELEASE_VERSION} on $(uname)"
  wget "https://github.com/estesp/manifest-tool/releases/download/${RELEASE_VERSION}/manifest-tool-linux-$(uname -p)" -O manifest-tool
  chmod +x manifest-tool
  sudo mv manifest-tool /usr/local/bin/manifest-tool
}

main $@
