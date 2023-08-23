#!/bin/bash

set -o pipefail
set +e

serviceIP=$(kubectl get svc replicated-sdk -n nginx -o jsonpath='{.spec.clusterIP}')

inputLicenseFields="[{"name":"num_seats","value":"10"}]"

echo "$inputLicenseFields" | jq -r '.[] | @base64'

echo $?
