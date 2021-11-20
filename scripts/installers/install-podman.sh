#!/bin/bash

set -o errexit
set -o nounset

main() {
  if [[ -x "$(command -v podman)" ]]; then
    podman version
    exit 0
  fi

  if [[ "$(lsb_release -i --short)" != "Ubuntu" ]]; then
    echo "Installer only works on Ubuntu"
    exit 1
  fi

  . /etc/os-release
  echo "deb https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_${VERSION_ID}/ /" | sudo tee /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
  curl -L "https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_${VERSION_ID}/Release.key" | sudo apt-key add -
  sudo apt-get update
  sudo apt-get -y install podman

  podman version
}

main "$@"
