#!/bin/bash

set -e

export CHART_VERSION=0.0.0
export REPLICATED_SDK_TAG=24h
export REPLICATED_SDK_REGISTRY=ttl.sh/$USER

envsubst < Chart.yaml.tmpl > Chart.yaml
envsubst < values.yaml.tmpl > values.yaml

export CHART_NAME=
CHART_NAME=$(helm package . | rev | cut -d/ -f1 | rev)
helm push "$CHART_NAME" oci://ttl.sh/"$USER"

rm -f Chart.yaml values.yaml
