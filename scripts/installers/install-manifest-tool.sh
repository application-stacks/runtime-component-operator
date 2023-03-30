#!/bin/bash

set -o errexit
set -o nounset

main() {
  if [[ -x "$(command -v manifest-tool)" ]]; then
    manifest-tool --version
    exit 0
  fi

  DEFAULT_RELEASE=v1.0.2
  RELEASE_VERSION=${1:-$DEFAULT_RELEASE}

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

  echo "****** Installing manifest-tool version ${RELEASE_VERSION} on $(uname)"
  wget "https://github.com/estesp/manifest-tool/releases/download/${RELEASE_VERSION}/manifest-tool-linux-${arch}" -O manifest-tool
  chmod +x manifest-tool
  sudo mv manifest-tool /usr/local/bin/manifest-tool
}

main $@
