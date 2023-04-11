#!/usr/bin/env bash
    type=$1
    namespace=${2:-default}

    MAX_RETRIES=99

    count=0

    echo "Waiting for ${type} ${namespace} to be ready..."
    kubectl get ${type} -n ${namespace} >/dev/null 

    while [ $? -ne 0 ]; do
        if [ $count -eq $MAX_RETRIES ]; then
            echo "Timeout and exit due to maximum retires reached."
            return 1
        fi

        count=$((count+1))

        echo "Unable to get ${type} ${namespace}: retry ${count} of ${MAX_RETRIES}."
        sleep 5s
        kubectl get ${type} -n ${namespace}
    done

    echo "The ${type} ${namespace} is ready."

    if [[ "${type}" == *"deploy"* ]]; then
        echo "Waiting for deployment ${name} pods to be ready..."
        count=0
        replicas="$(kubectl get deploy -n ${namespace} -o=jsonpath='{.items[*].status.readyReplicas}')"
        readyReplicas="$(kubectl get deploy -n ${namespace} -o=jsonpath='{.items[*].status.replicas}')"
        echo "replicas: $replicas,readyReplicas: $readyReplicas; Retry ${count} of ${MAX_RETRIES}."

        while true;
        do
            if [ "$replicas" = "$readyReplicas" ]; then
                echo "all deployments ready"
                exit 0
            fi
            if [ $count -eq $MAX_RETRIES ]; then
                echo "Timeout and exit due to maximum retires reached."
                exit 1
            fi

            count=$((count+1))

            echo "replicas: $replicas,readyReplicas: $readyReplicas; Retry ${count} of ${MAX_RETRIES}."
            sleep 5s
            replicas="$(kubectl get deploy -n ${namespace} -o=jsonpath='{.items[*].status.readyReplicas}')"
            readyReplicas="$(kubectl get deploy -n ${namespace} -o=jsonpath='{.items[*].status.replicas}')"
        done

        echo "All pods ready for deployment ${name}."
    fi
