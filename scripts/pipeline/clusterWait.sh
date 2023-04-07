#!/usr/bin/env bash
arch=$1


rco_demand_id=$(get_env RCO_DEMAND_ID_$arch)
export demandId=$rco_demand_id
echo "calling ebc_waitForDemand.sh for $arch"
cd ebc-gateway-http
   
export ebcEnvironment=prod

json=$(./ebc_waitForDemand.sh)
rc=$?
echo "return from ebc_waitForDemand.sh for $arch"

cd ..

if [[ "$rc" == 0 ]]; then
    echo "EBC create of id: $rco_demand_id cluster successful"
else
    echo "EBC create of id: $rco_demand_id cluster failed, ask #was-ebc slack channel for help mentioning your demand id: $rco_demand_id"
    exit 1
fi

status=$(jq -c '.status' <<< $json)
ip=$(jq -c '.machineAddresses.ocpinf' <<< $json)
ip=$(echo "$ip" | tr -d '"')

PRIVATE_KEY="$(get_env private_key "")"
echo -n "${PRIVATE_KEY}" | base64 -d > id_rsa

echo "value of private key: "
cat id_rsa

chmod 600 id_rsa
pwd
ls -l id_rsa

echo "oc version:"
oc version

token=$(ssh -o StrictHostKeyChecking=no -i id_rsa root@$ip "cat ~/auth/kubeadmin-password")

echo "json=$json"
echo "status=$status"
echo "token=$token"
echo $ip

