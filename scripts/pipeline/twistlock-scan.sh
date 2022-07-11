 #!/bin/bash -e

 function install_twistlock() {
    DEBIAN_FRONTEND=noninteractive apt-get -y update && \
    DEBIAN_FRONTEND=noninteractive apt-get -y install uuid-runtime file jq && \
    wget --no-check-certificate https://w3twistlock.sos.ibm.com/download/tt_latest.zip && \
    unzip -l tt_latest.zip | grep linux_x86_64/tt | awk '{print $4}' | xargs unzip -j tt_latest.zip -d /usr/local/bin
    chmod +x /usr/local/bin/tt
}

# Install Twistlock
install_twistlock

IBMCLOUD_API_KEY=$(get_env ibmcloud-api-key)

# loop through listed artifact images and scan each amd64 image
for artifact_image in $(list_artifacts); do
  IMAGE_LOCATION=$(load_artifact $artifact_image name)
  ARCH=$(load_artifact $artifact_image arch)

  echo "image from load_artifact:" $IMAGE_LOCATION 
  echo "arch:" $ARCH

  if [[ -z ${IMAGE_LOCATION} ]]; then 
    continue
  fi

  if [[ "$ARCH" != "amd64" ]]; then 
    echo $arch " images not supported by twistlock scanning, skipping"
    continue
  fi

  # The "pull" in "pull-and-scan" is a remote action. The image will be pulled and scanned on a remote server, and
  # the results will be dumped to file here.

  # twistlock command
tt images pull-and-scan ${IMAGE_LOCATION} --iam-api-key $IBMCLOUD_API_KEY -u "$(get_env twistlock-user-id):$(get_env twistlock-api-key)" -g "websphere"   

  # save the artifact
  for i in twistlock-scan-results*; do save_result scan-artifact ${i}; done
done
