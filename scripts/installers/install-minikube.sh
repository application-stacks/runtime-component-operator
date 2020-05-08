#!/bin/bash

set -e

## Some minikube job specific ENV variables
export CHANGE_MINIKUBE_NONE_USER=true
export MINIKUBE_WANTUPDATENOTIFICATION=false
export MINIKUBE_HOME=$HOME
export CHANGE_MINIKUBE_NONE_USER=true
export KUBECONFIG=$HOME/.kube/config

function main () {
  echo "****** Installing minikube..."
  install_minikube

  echo "****** Verifying installation..."
  kubectl cluster-info
  wait_for_kube
  ## Run tests below
  echo "Minikube enabled job is running..."

}

function install_minikube() {
  sudo apt-get update -y
  sudo apt-get -qq -y install conntrack
  ## get kubectl
  curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v1.18.1/bin/linux/amd64/kubectl \
      && chmod +x kubectl \
      && sudo mv kubectl /usr/local/bin/
  ## Download minikube
  curl -Lo minikube https://storage.googleapis.com/minikube/releases/v1.8.1/minikube-linux-amd64 \
      && chmod +x minikube \
      && sudo mv minikube /usr/local/bin/

  mkdir -p $HOME/.kube $HOME/.minikube
  touch $KUBECONFIG
  sudo minikube start --profile=minikube --vm-driver=none --kubernetes-version=v1.18.1
  minikube update-context --profile=minikube

  eval "$(minikube docker-env --profile=minikube)" && export DOCKER_CLI='docker'
}

function wait_for_kube() {
  local json_path='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'

  until kubectl -n kube-system get pods -l k8s-app=kube-dns -o jsonpath="$json_path" 2>&1 | grep -q "Ready=True"; do
    sleep 5;echo "waiting for kube-dns to be available"
    kubectl get pods --all-namespaces
  done
}

main "$@"
