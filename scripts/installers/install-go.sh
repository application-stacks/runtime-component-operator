#!/bin/bash

set -o errexit
set -o nounset

main() {
    DEFAULT_RELEASE_VERSION="$(grep '^go [0-9]\+.[0-9]\+' go.mod | cut -d ' ' -f 2)"
    RELEASE_VERSION=${1:-$DEFAULT_RELEASE_VERSION}

    if [[ -x "$(command -v go)" ]]; then
        if go version | grep -q "$RELEASE_VERSION"; then
            go version
            exit 0
        else
            echo "****** Another go version detected"
        fi
    fi

    if [[ "$(uname)" = "Darwin" ]]; then
        binary_url="https://golang.org/dl/go${RELEASE_VERSION}.darwin-amd64.tar.gz"
    elif [[ "$(uname -p)" = "s390x" ]]; then
        binary_url="https://golang.org/dl/go${RELEASE_VERSION}.linux-s390x.tar.gz"
    elif [[ "$(uname -p)" = "ppc64le" ]]; then
        binary_url="https://golang.org/dl/go${RELEASE_VERSION}.linux-ppc64le.tar.gz"
    else
        binary_url="https://golang.org/dl/go${RELEASE_VERSION}.linux-amd64.tar.gz"
    fi

    echo "****** Installing Go version $RELEASE_VERSION on $(uname)"

    rm -rf /usr/local/go && wget --no-verbose --header "Accept: application/octet-stream" "${binary_url}" -O - | tar -xz -C /usr/local/
    export PATH=$PATH:/usr/local/go/bin

    go version
}

main "$@"