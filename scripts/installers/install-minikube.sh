#!/bin/bash

set -e

## Some minikube job specific ENV variables
export MINIKUBE_WANTUPDATENOTIFICATION=false
export MINIKUBE_HOME=$HOME
export KUBECONFIG=$HOME/.kube/config
export DOCKER_HOST=tcp://127.0.0.1:2376

function main () {
  install_minikube

  echo "****** Verifying installation..."
  kubectl cluster-info
  wait_for_kube
  
  echo "****** Minikube enabled job is running..."

}

function install_minikube() {
  sudo apt-get update -y
  sudo apt-get install -y lsb-release kubelet kubeadm
  sudo apt-get -qq -y install conntrack
  sudo apt-get install --reinstall -y systemd

  ## Download kubectl
  echo "****** Installing kubectl v1.19.4..."
  curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v1.19.4/bin/linux/amd64/kubectl \
  && chmod +x kubectl \
  && sudo mv kubectl /usr/local/bin/ 

  ## Download minikube
  echo "****** Installing Minikube v1.21.0..."
  curl -Lo minikube https://storage.googleapis.com/minikube/releases/v1.21.0/minikube-linux-amd64 \
  && chmod +x minikube \
  && sudo mv minikube /usr/local/bin/

  echo $DOCKER_HOST

  mkdir -p $HOME/.kube $HOME/.minikube
  touch $KUBECONFIG
  minikube start --profile=minikube --kubernetes-version=v1.19.4 --driver=docker --force
  minikube update-context --profile=minikube

  eval "$(minikube docker-env --profile=minikube)" && export DOCKER_CLI='docker'
}


function wait_for_kube() {
  local json_path='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
  echo "****** Waiting for kube-controller-manager to be available..."

  until kubectl -n kube-system get pods -l component=kube-controller-manager -o jsonpath="$json_path" 2>&1 | grep -q "Ready=True"; do
    sleep 5;
  done

  kubectl get pods --all-namespaces
}

main "$@"
