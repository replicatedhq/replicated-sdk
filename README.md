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
When using a `Development` license, the Replicated SDK will initiate in dev mode. If you are performing a Helm install/upgrade using the replicated helm chart, you can utilize the following values in the chart YAML for the Replicated SDK's dev mode:
```yaml
dev:
  licenseID: "development-license-id"
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

To enable the Replicated SDK's `dev` mode, you can use the following values in the chart YAML:
- `licenseID`: This should be set to the development license ID obtained from the vendor portal.
- `mockData`: This field allows you to provide the necessary data for the Replicated SDK to return mock responses.

The `dev` mode will return mock responses for SDK APIs when mock data is provided else SDK will return actual data. The mock data accepts a yaml format of `helmChartURL`, `currentRelease`, `deployedReleases` and `availableReleases`.

Below is an example demonstrating all the supported values for the `mockData` field:
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

### mock data endpoints
The mock data endpoints provide functionality to manage mock data. The following endpoints are available:
- *POST* `/api/v1/mock-data` endpoint accepts a JSON request body to set the mock data.
- *GET* `/api/v1/mock-data` endpoint returns the entire mock data.
- *DELETE* `/api/v1/mock-data` endpoint deletes the mock data.

**Note** The endpoint *POST* `/api/v1/mock-data` exclusively supports full data posts, meaning that if any updates are required for the mock data, the entire dataset must be sent to the endpoint via the `POST` method.

**Note**: [_when we start supporting custom domains for replicated app endpoint_] When using custom domains for replicated app, the first license pull with license ID would be through replicated app endpoint. Once the license information is available and Replicated SDK is running, subsequent api calls to replicated app would be via custom domain url of replicated app.

### Replicated SDK "dev" mode for staging/okteto environments
**Note**: Please don't document this in customer facing docs.

Replicated SDK supports `dev`.`replicatedEndpoint` helm value which can be used to provide alternate replicated app endpoints.
This helm value will be handy for replicants when working with staging/okteto environment deployments

In order to use staging/okteto license IDs in `dev` mode, you would have to provide the replicated app endpoint.
You can do so by providing the `replicatedEndpoint` value with the staging/okteto replicated app url
eg: for staging licenses you can set the replicated app endpoint as below in `values.yaml`:
```yaml
dev:
  licenseID: "development-license-id"
  replicatedEndpoint: "staging.replicated.app"
  mockData: ""
```
and then add the below content to replicated-secret.yaml
```yaml
  {{- if .Values.dev.replicatedEndpoint }}
  REPLICATED_ENDPOINT: {{ .Values.dev.replicatedEndpoint }}
  {{- end }}
```

