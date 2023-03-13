# Intro

This helm chart allows installing the Replicated KOTS SDK using helm.

# Usage

## Installation

```shell
helm upgrade --install [RELEASE_NAME] . \
    --namespace [NAMESPACE] \
    --create-namespace \
    --set-file license=[path/to/license.yaml] \
    --set channelID=[CHANNEL_ID] \
    --set channelName=[CHANNEL_NAME] \
    --set channelSequence=[CHANNEL_SEQUENCE] \
    --set releaseSequence=[RELEASE_SEQUENCE] \
    --set statusInformers=[STATUS_INFORMERS]
```

Example:
```shell
helm upgrade --install kots-sdk oci://ttl.sh/salah/kots-sdk \
    --namespace default \
    --set-file license=license.yaml \
    --set channelID=1YGSYsmJEjIj2XlyK1vqjCwuyb1 \
    --set channelName=Beta \
    --set channelSequence=1 \
    --set releaseSequence=1 \
    --set statusInformers="{default/deployment/nginx,default/statefulset/rqlite}"
```

## Local dev

To build a chart that uses ttl images, run the build-ttl.sh script located in the scripts directory.
```
./scripts/build-ttl.sh
```
