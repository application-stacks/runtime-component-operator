#!/usr/bin/env bash

set -e -o pipefail

main() {
  install_kind
}

install_kind() {
  install_dependencies
  setup_env
  create_kind_cluster
  install_rook_ceph

  kubectl cluster-info --context kind-e2e-cluster
}

install_dependencies() {
  ## Install docker
  if ! command -v docker &> /dev/null; then
    echo "****** Installing Docker..."
    if test -f "/usr/share/keyrings/docker-archive-keyring.gpg"; then
      rm /usr/share/keyrings/docker-archive-keyring.gpg
    fi
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
    local release="$(cat /etc/os-release | grep UBUNTU_CODENAME | cut -d '=' -f 2)"
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
      ${release} stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update  -y &&  sudo apt-get install -y docker-ce docker-ce-cli containerd.io conntrack
  fi

  ## Install kubectl
  if ! command -v kubectl &> /dev/null; then
    echo "****** Installing kubectl v1.23.12..."
    curl -Lo /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.23.12/bin/linux/amd64/kubectl && chmod +x /usr/local/bin/kubectl
  fi

  # Install kind
  if ! command -v kind &> /dev/null; then
    echo "****** Installing kind..."
    curl -Lo /usr/local/bin/kind https://kind.sigs.k8s.io/dl/v0.16.0/kind-linux-amd64
    chmod +x /usr/local/bin/kind
  fi
}

setup_env() {
  if lvs | grep -q kind-lv; then
    return 0
  fi

  # Create LVM used for Ceph
  rm -rf /cephfs; mkdir /cephfs && dd if=/dev/zero of=/cephfs/vdisk.img bs=100M count=55 || {
    echo "Error: unable to create virtual disk for Ceph"
    exit 1
  }

  losetup -f /cephfs/vdisk.img || {
    echo "Error: unable to create loop device"
    exit 1
  }

  local loop_dev_name="$(losetup -a | grep '/cephfs/vdisk.img' | cut -d ':' -f 1)"

  pvcreate "${loop_dev_name}" && vgcreate kind-vg "${loop_dev_name}" && lvcreate -L 5G -n kind-lv kind-vg || {
    echo "Error: unable logical volume"
    exit 1
  }
}

create_kind_cluster() {
  # Create registry container unless it already exists
  local reg_name='kind-registry'
  local reg_port='5000'
  if [ "$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)" != 'true' ]; then
    docker run -d --restart=always -p "${reg_port}:5000" --name "${reg_name}" registry:2
  fi

  if [[ -z "$(kind get clusters | grep e2e-cluster)" ]]; then
    # Create a cluster with the local registry enabled in containerd
    cat << EOF | kind create cluster --name e2e-cluster --image kindest/node:v1.23.12 --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
- role: worker
- role: worker
networking:
  apiServerAddress: "$(dig +noall +answer +short $(hostname))"
  apiServerPort: 6443
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${reg_port}"]
    endpoint = ["http://${reg_name}:5000"]
EOF

  kubectl config set-context kind-e2e-cluster

  sleep 5

  kubectl wait --namespace kube-system \
      --for=condition=ready pod \
      --selector=component=kube-controller-manager \
      --timeout=300s || {
    echo "Error: Failed to deploy kind cluster"
    exit 1
  }

  # Connect the registry to the cluster network if not already connected
  if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${reg_name}")" = 'null' ]; then
    docker network connect "kind" "${reg_name}"
  fi

# Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${reg_port}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
  fi

  readonly test_namespace="rco-test"
  echo "****** Creating test namespace: ${test_namespace}"
  kubectl create namespace "${test_namespace}"
  kubectl config set-context kind-e2e-cluster --namespace="${test_namespace}"
}

install_rook_ceph() {
  if kubectl get storageclass | grep -q rook-ceph; then
    echo "Rook storage orchestrator is already installed"
    return 0
  fi

  cat << EOF | kubectl apply -f -
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: local-storage
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: kind-pv
spec:
  storageClassName: local-storage
  capacity:
    storage: 5G
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  volumeMode: Block
  local:
    path: /dev/dm-1
  nodeAffinity:
      required:
        nodeSelectorTerms:
          - matchExpressions:
              - key: kubernetes.io/hostname
                operator: In
                values:
                - e2e-cluster-worker
EOF

  rm -rf /root/rook
  git clone --single-branch --branch master https://github.com/rook/rook.git
  cd /root/rook/deploy/examples
  kubectl apply -f crds.yaml -f common.yaml -f operator.yaml

  wait_for rook-ceph rook-ceph-operator

  cat << EOF | kubectl apply -f -
kind: ConfigMap
apiVersion: v1
metadata:
  name: rook-config-override
  namespace: rook-ceph # namespace:cluster
data:
  config: |
    [global]
    osd_pool_default_size = 1
    mon_warn_on_pool_no_redundancy = false
    bdev_flock_retry = 20
    bluefs_buffered_io = false
---
apiVersion: ceph.rook.io/v1
kind: CephCluster
metadata:
  name: my-cluster
  namespace: rook-ceph # namespace:cluster
spec:
  dataDirHostPath: /var/lib/rook
  cephVersion:
    image: quay.io/ceph/ceph:v17
    allowUnsupported: true
  mon:
    count: 1
    allowMultiplePerNode: true
  mgr:
    count: 1
    allowMultiplePerNode: true
  dashboard:
    enabled: true
  crashCollector:
    disable: true
  storage:
    useAllNodes: false
    useAllDevices: false
    storageClassDeviceSets:
    - name: set1
      count: 1
      portable: false
      tuneDeviceClass: true
      tuneFastDeviceClass: false
      encrypted: false
      volumeClaimTemplates:
      - metadata:
          name: data
        spec:
          resources:
            requests:
              storage: 5000M
          storageClassName: local-storage
          volumeMode: Block
          accessModes:
            - ReadWriteOnce
  healthCheck:
    daemonHealth:
      mon:
        interval: 45s
        timeout: 600s
  priorityClassNames:
    all: system-node-critical
    mgr: system-cluster-critical
  disruptionManagement:
    managePodBudgets: true
---
apiVersion: ceph.rook.io/v1
kind: CephBlockPool
metadata:
  name: builtin-mgr
  namespace: rook-ceph # namespace:cluster
spec:
  name: .mgr
  replicated:
    size: 1
    requireSafeReplicaSize: false
EOF

  wait_for rook-ceph rook-ceph-osd-0

  kubectl apply -f ./csi/rbd/storageclass-test.yaml -f ./csi/rbd/pvc.yaml
  kubectl apply -f filesystem-test.yaml -f ./csi/cephfs/storageclass.yaml -f ./csi/cephfs/pvc.yaml
  kubectl patch storageclass standard -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'
  kubectl patch storageclass rook-cephfs -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}

wait_for() {
  local namespace="${1}"
  local deployment="${2}"

  while [[ $(kubectl get deploy -n "${namespace}" "${deployment}" -o jsonpath='{.status.readyReplicas}') -ne "1" ]]; do
    echo "****** Waiting for ${namespace}/${deployment} to be ready..."
    sleep 10
  done
  echo "****** ${namespace}/${deployment} is ready..."
}

main "$@"
