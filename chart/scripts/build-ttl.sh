#!/bin/bash

set -e

export REPLICATED_CHART_NAME=replicated
export REPLICATED_CHART_VERSION=0.0.0
export REPLICATED_TAG=24h
export REPLICATED_REGISTRY=ttl.sh/$USER

envsubst < Chart.yaml.tmpl > Chart.yaml
envsubst < values.yaml.tmpl > values.yaml

export CHART_NAME=
CHART_NAME=$(helm package . | rev | cut -d/ -f1 | rev)
helm push "$CHART_NAME" oci://ttl.sh/"$USER"

rm -f Chart.yaml values.yaml
