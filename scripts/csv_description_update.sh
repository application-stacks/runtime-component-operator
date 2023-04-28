#!/bin/bash -e

BASE_DIR="$(cd $(dirname $0) && pwd)"

function update_csv() {
    local FILE="$BASE_DIR/../bundle/manifests/runtime-component.clusterserviceversion.yaml"

    DESCRIPTION_FILE=/tmp/description.md
    echo "  description: |" > $DESCRIPTION_FILE
    cat "$BASE_DIR/../config/manifests/description.md" | sed 's/^/    /' >> $DESCRIPTION_FILE
    sed -i.bak '/^  displayName: Runtime Component/r /tmp/description.md' $FILE
    rm -f "${FILE}.bak"
    rm -f $DESCRIPTION_FILE
    }

if [ "$1" == "update_csv" ]; then
    update_csv
else
    echo "Usage: $0 update_csv"
    exit 1
fi
