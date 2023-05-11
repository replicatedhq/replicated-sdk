# Introduction

This is the software development kit (SDK) for Replicated.

# Development

### Go Binary / API

Build the binary:
```shell
make build
```

Run the Replicated API:
```shell
./bin/replicated api \
    --license-file=[path/to/license.yaml] \
    --license-fields-file=[path/to/license-fields.yaml] \
    --app-name=[APP_NAME] \
    --channel-id=[CHANNEL_ID] \
    --channel-name=[CHANNEL_NAME] \
    --channel-sequence=[CHANNEL_SEQUENCE] \
    --release-sequence=[RELEASE_SEQUENCE] \
    --release-is-required=[RELEASE_IS_REQUIRED] \
    --release-created-at=[RELEASE_CREATED_AT] \
    --release-notes=[RELEASE_NOTES] \
    --version-label=[VERSION_LABEL] \
    --namespace=[NAMESPACE]
```

Example:
```shell
./bin/replicated api \
    --license-file=license.yaml \
    --license-fields-file=license-fields.yaml \
    --app-name="My App" \
    --channel-id=1YGSYsmJEjIj2XlyK1vqjCwuyb1 \
    --channel-name=Beta \
    --channel-sequence=1 \
    --release-sequence=1 \
    --release-is-required=false \
    --release-created-at="2023-05-09T16:41:35.000Z" \
    --release-notes="my release notes" \
    --version-label="v1.0.0" \
    --namespace=default
```

### Helm Chart
Build and push the replicated Docker image and Helm chart to ttl.sh:

```shell
make build-ttl.sh
```

The above command will also output the .tgz Helm chart under `chart/replicated-0.0.0.tgz`.
You can either extract and include the produced .tgz Helm chart as a subchart in other applications, or you can run the following command to install the chart:

```shell
helm upgrade --install replicated oci://ttl.sh/salah/replicated \
    --namespace [NAMESPACE] \
    --set-file license=[path/to/license.yaml] \
    --set-file licenseFields=[path/to/license-fields.yaml] \
    --set appName=[APP_NAME] \
    --set channelID=[CHANNEL_ID] \
    --set channelName=[CHANNEL_NAME] \
    --set channelSequence=[CHANNEL_SEQUENCE] \
    --set releaseSequence=[RELEASE_SEQUENCE] \
    --set releaseIsRequired=[IS_REQUIRED] \
    --set releaseCreatedAt=[VERSION_LABEL] \
    --set releaseNotes=[RELEASE_NOTES] \
    --set versionLabel=[VERSION_LABEL]
```

Example:
```shell
helm upgrade --install replicated oci://ttl.sh/salah/replicated \
    --namespace default \
    --set-file license=license.yaml \
    --set-file licenseFields=license-fields.yaml \
    --set appName="My App" \
    --set channelID=1YGSYsmJEjIj2XlyK1vqjCwuyb1 \
    --set channelName=Beta \
    --set channelSequence=1 \
    --set releaseSequence=1 \
    --set releaseIsRequired=false \
    --set releaseCreatedAt="2023-05-09T16:41:35.000Z" \
    --set releaseNotes="my release notes" \
    --set versionLabel="v1.0.0"
```

**Note**: you can set the above values in the `values.yaml` file instead of using the `--set` flag for each field.
