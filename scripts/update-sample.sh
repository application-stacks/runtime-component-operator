#!/bin/sh

# This script fetches the sha256 for the latest version of the open liberty getting started container
# image, and then edits the sample deployment and the csv for the operator, and inserts the tag

if ! skopeo -v ; then
  echo "Skopeo is not installed. Sample sha will not be updated"
  exit
fi

echo "Editing sample tag"
SHA=$(skopeo inspect docker://icr.io/appcafe/open-liberty/samples/getting-started:latest | jq '.Digest'| sed -e 's/"//g')
if [ -z $SHA ]
then
  echo "Couldn't find latest SHA for sample image"
  exit
fi

echo "sha is $SHA"

files=" 
config/samples/rc.app.stacks_v1_runtimecomponent.yaml
config/manager/manager.yaml
internal/deploy/kustomize/daily/base/runtime-component-operator.yaml
"

for file in $files 
do
  sed -i.bak "s,getting-started@sha256:[a-zA-Z0-9]*,getting-started@$SHA," $file
  rm $file.bak
done

