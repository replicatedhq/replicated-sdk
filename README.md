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
    --set versionLabel=[VERSION_LABEL] \
    --set parentChartURL=[PARENT_CHART_URL]
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
    --set versionLabel="v1.0.0" \
    --set parentChartURL="oci://registry.replicated.com/my-app/my-channel/my-parent-chart"
```

**Note**: you can set the above values in the `values.yaml` file instead of using the `--set` flag for each field.

## Enabling Replicated SDK "dev" mode
The Replicated SDK will start in `dev` mode when `"Development"` license is used.
The `dev` mode will return mock responses for SDK APIs when mock data is provided else SDK will return actual data.
Mock data can be provided to the dev mode by setting `--set-file dev.mockData=mock_data.yaml`.
The mock data accepts a yaml format of `helmChartURL`, `currentRelease`, `deployedReleases` and `availableReleases`
An example of mock data is shown below:
```yaml
helmChartURL: oci://registry.replicated.com/dev-app/dev-channel/dev-parent-chart
currentRelease:
  versionLabel: 0.1.3
  isRequired: false
  releaseNotes: "release notes 0.1.3"
  createdAt: 2023-05-23T20:58:07Z
  deployedAt: 2023-05-23T21:58:07Z
  helmReleaseName: dev-parent-chart
  helmReleaseRevision: 3
  helmReleaseNamespace: default
deployedReleases:
- versionLabel: 0.1.1
  isRequired: false
  releaseNotes: "release notes 0.1.1"
  createdAt: 2023-05-21T20:58:07Z
  deployedAt: 2023-05-21T21:58:07Z
  helmReleaseName: dev-parent-chart
  helmReleaseRevision: 1
  helmReleaseNamespace: default
- versionLabel: 0.1.2
  isRequired: false
  releaseNotes: "release notes 0.1.2"
  createdAt: 2023-05-22T20:58:07Z
  deployedAt: 2023-05-22T21:58:07Z
  helmReleaseName: dev-parent-chart
  helmReleaseRevision: 2
  helmReleaseNamespace: default
- versionLabel: 0.1.3
  isRequired: false
  releaseNotes: "release notes 0.1.3"
  createdAt: 2023-05-23T20:58:07Z
  deployedAt: 2023-05-23T21:58:07Z
  helmReleaseName: dev-parent-chart
  helmReleaseRevision: 3
  helmReleaseNamespace: default
availableReleases:
- versionLabel: 0.1.4
  isRequired: true
  releaseNotes: "release notes 0.1.4"
  createdAt: 2023-05-24T20:58:07Z
  deployedAt: 2023-05-24T21:58:07Z
- versionLabel: 0.1.5
  isRequired: false
  releaseNotes: "release notes 0.1.5"
  createdAt: 2023-06-01T20:58:07Z
  deployedAt: 2023-06-01T21:58:07Z
```

When the above mock data is configured:
- *GET* `/api/v1/app/info` will retrieve the application details along with the information about the `currentRelease`.
- *GET* `/api/v1/app/updates` will provide a list of `availableReleases`.
- *GET* `/api/v1/app/history` will provide a list of `deployedReleases`.

While running a Helm install/upgrade with `replicated` as a subchart, the following values can be used in the chart YAML:
```yaml
dev:
  mockData: |
    helmChartURL: oci://registry.replicated.com/dev-app/dev-channel/dev-parent-chart
    currentRelease:
      versionLabel: 0.1.7
      isRequired: false
      releaseNotes: "test"
      createdAt: "2012-09-09"
      helmReleaseName: dev-parent-chart
      helmReleaseRevision: 2
      helmReleaseNamespace: default
```
### mock data endpoints
The mock data endpoints provide functionality to manage mock data. The following endpoints are available:
- *POST* `/api/v1/mock-data` endpoint accepts a JSON request body to set the mock data.
- *GET* `/api/v1/mock-data` endpoint returns the entire mock data.
- *DELETE* `/api/v1/mock-data` endpoint deletes the mock data.

**Note** The endpoint *POST* `/api/v1/mock-data` exclusively supports full data posts, meaning that if any updates are required for the mock data, the entire dataset must be sent to the endpoint via the `POST` method.

**Note**: `dev` mode can be enabled for `"Development"` license type only.
