#!/usr/bin/env bash
arch=$1
timestamp=$(date +%s)
echo $timestamp
wlo_demand_id="wlo_$timestamp"_"$arch"
echo "wlo_demand_id=$wlo_demand_id"
    
#git clone https://$(get_env git-token)@github.ibm.com/elastic-build-cloud/ebc-gateway-http.git
cd ebc-gateway-http

echo "PRE_RELEASE=$PRE_RELEASE"
echo "arch=$arch"

export intranetId_USR=$(get_env ebc_id)
export intranetId_PSW=$(get_env ebc_pw)

export demandId=$wlo_demand_id
set_env WLO_DEMAND_ID_$arch "$wlo_demand_id"
echo "wlo_demand_id=$wlo_demand_id"

PRE_RELEASE=$(get_env pre-release)
PRE_RELEASE="$(echo "$PRE_RELEASE" | tr '[:upper:]' '[:lower:]')"
if [[ ! -z "$PRE_RELEASE" && "$PRE_RELEASE" != "false" && "$PRE_RELEASE" != "no"  ]]; then
    echo "<<1>>"
    rhcos_level=$(get_env pre-release-rhcos-url)
    ocp_level=$(get_env pre-release-ocp-url)
    echo "this is a pre-release OCP cluster build"
    echo "ocp level: $ocp_level"
    echo "core os level: $rhcos_level"
    export ebc_fyre_install_url=${ocp_level}/openshift-install-linux.tar.gz
    export ebc_fyre_client_url=${ocp_level}/openshift-client-linux.tar.gz
    if [[ "$arch" == "X" ]]; then
        # X values
         echo "<<1a>>"
        export ebc_plan=svl-onepipeline-ocpplus_x_custom.yml
        export ebc_fyre_kernel_url=${rhcos_level}/rhcos-live-kernel-x86_64
        export ebc_fyre_initramfs_url=${rhcos_level}/rhcos-live-initramfs.x86_64.img
        export ebc_fyre_metal_url=${rhcos_level}/rhcos-metal.x86_64.raw.gz
        export ebc_fyre_rootfs_url=${rhcos_level}/rhcos-live-rootfs.x86_64.img
    fi
    if [[ "$arch" == "Z" ]]; then
        # Z values
        export ebc_plan=svl-onepipeline-ocpplus_z_custom.yml
        export ebc_fyre_kernel_url=${rhcos_level_z}/rhcos-live-kernel-s390x
        export ebc_fyre_initramfs_url=${rhcos_level_z}/rhcos-live-initramfs.s390x.img
        export ebc_fyre_metal_url=${rhcos_level_z}/rhcos-metal.s390x.raw.gz
        export ebc_fyre_rootfs_url=${rhcos_level_z}/rhcos-live-rootfs.s390x.img
    fi
    if [[ "$arch" == "P" ]]; then
        # P
        export ebc_plan=svl-onepipeline-ocpplus_p_custom.yml
        export ebc_fyre_kernel_url=${rhcos_level_p}/rhcos-live-kernel-ppc64le
        export ebc_fyre_initramfs_url=${rhcos_level_p}/rhcos-live-initramfs.ppc64le.img
        export ebc_fyre_metal_url=${rhcos_level_p}/rhcos-metal.ppc64le.raw.gz
        export ebc_fyre_rootfs_url=${rhcos_level_p}/rhcos-live-rootfs.ppc64le.img
    fi
else
    if [[ "$arch" == "X" ]]; then
        export ebc_plan=svl-onepipeline-ocpplus_x.yml
        echo "setting ebc plan for X: $ebc_plan"
    fi
    if [[ "$arch" == "Z" ]]; then
       export ebc_plan=svl-onepipeline-ocpplus_z.yml
    fi
    if [[ "$arch" == "P" ]]; then
        export ebc_plan=svl-onepipeline-ocpplus_p.yml
    fi
    export ebc_ocp_version=$(get_env ocp_version)
fi
# prod or dev, start out with dev
export ebcEnvironment=prod
# priority is 30 to start, prod priority may be 100
export ebc_priority=30
export ebc_autoCompleteAfterXHours=$(get_env ebc_autocomplete_hours "6")
# gather pipeline URL and place in following env var
reason="https://cloud.ibm.com/devops/pipelines/tekton/${PIPELINE_ID}/runs/${PIPELINE_RUN_ID}"
export ebc_reasonForEnvironment=$reason
    
./ebc_demand.sh
rc=$?
if [[ "$rc" == 0 ]]; then
    echo "cluster requested"
else
    echo "Outage impacting demand of cluster, try again later"
    exit 1
fi