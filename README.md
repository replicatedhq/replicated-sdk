# Introduction

This is the software development kit (SDK) for Replicated KOTS.

# Development

## Go Binary / SDK API

Build the binary:
```shell
make build
```

Run the SDK API:
```shell
./bin/kots-sdk api \
    --license-file=[path/to/license.yaml] \
    --channel-id=[CHANNEL_ID] \
    --channel-name=[CHANNEL_NAME] \
    --channel-sequence=[CHANNEL_SEQUENCE] \
    --release-sequence=[RELEASE_SEQUENCE] \
    --status-informers=[STATUS_INFORMERS]
```

Example:
```shell
./bin/kots-sdk api \
    --license-file=license.yaml \
    --channel-id=1YGSYsmJEjIj2XlyK1vqjCwuyb1 \
    --channel-name=Beta \
    --channel-sequence=1 \
    --release-sequence=1 \
    --status-informers=default/deployment/nginx,default/statefulset/rqlite
```

## Helm Chart
Build and push the kots-sdk Docker image and Helm chart to ttl.sh:

```shell
make build-ttl.sh
```

The above command will also output the .tgz Helm chart under `chart/kots-sdk-0.0.0.tgz`.
You can either extract and include the produced .tgz Helm chart as a subchart in other applications, or you can run the following command to install the chart:

```shell
helm upgrade --install kots-sdk oci://ttl.sh/salah/kots-sdk \
    --namespace default \
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
    --namespace [NAMESPACE] \
    --create-namespace \
    --set-file license=license.yaml \
    --set channelID=1YGSYsmJEjIj2XlyK1vqjCwuyb1 \
    --set channelName=Beta \
    --set channelSequence=1 \
    --set releaseSequence=1 \
    --set statusInformers="{default/deployment/nginx,default/statefulset/rqlite}"
```

**Note**: you can set the above values in the `values.yaml` file instead of using the `--set` flag for each field.
