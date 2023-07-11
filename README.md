# Introduction

This is the software development kit (SDK) for Replicated.

# Development

### Go Binary / API

Build the binary:
```shell
make build
```

Setup a `config.yaml` file with the below values
```yaml
license: |
  << license.yaml >>
licenseFields:
  << license-fields.yaml >>
appName: "APP_NAME"
channelID: "CHANNEL_ID"
channelName: "CHANNEL_NAME"
channelSequence: "CHANNEL_SEQUENCE"
releaseSequence: "CHANNEL_SEQUENCE"
releaseCreatedAt: "RELEASE_CREATED_AT"
releaseNotes: "RELEASE_NOTES"
versionLabel: "VERSION_LABEL"
replicatedAppEndpoint: "REPLICATED_APP_ENDPOINT"
```

Run the Replicated API:
```shell
./bin/replicated api \
    --config-file=[path/to/config.yaml]
    --namespace=[NAMESPACE]
```

Example:
**config.yaml**
```yaml
license: |
  apiVersion: kots.io/v1beta1
  kind: License
  metadata:
    name: replicated-sdk-test
  spec:
    appSlug: replicated-sdk-test
    channelID: channeID
    channelName: Stable
    customerEmail: repl@replicated.com
    customerName: replicated-sdk-test
    endpoint: https://replicated.app
    entitlements:
      expires_at:
        description: License Expiration
        signature: {}
        title: Expiration
        value: ""
        valueType: String
    licenseID: licenseID
    licenseSequence: 2
    licenseType: dev
    signature: eyJsa9PSJ9
licenseFields:
  expires_at:
    description: License Expiration
    name: expires_at
    signature:
      v1: eZrWHA==
    title: Expiration
    value: ""
    valueType: String
appName: "replicated-sdk-test"
channelID: "channelID"
channelName: "Stable"
channelSequence: 1
releaseSequence: 1
releaseCreatedAt: "2023-07-03T12:48:26Z"
releaseNotes: "replicated-sdk-test"
versionLabel: "0.0.1"
replicatedAppEndpoint: ""
```

```shell
./bin/replicated api \
    --config-file=config.yaml
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
    --set releaseCreatedAt=[VERSION_LABEL] \
    --set releaseNotes=[RELEASE_NOTES] \
    --set versionLabel=[VERSION_LABEL] \
    --set parentChartURL=[PARENT_CHART_URL] \
    --set replicatedAppEndpoint=[REPLICATED_APP_ENDPOINT]
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
    --set releaseCreatedAt="2023-05-09T16:41:35.000Z" \
    --set releaseNotes="my release notes" \
    --set versionLabel="v1.0.0" \
    --set parentChartURL="oci://registry.replicated.com/my-app/my-channel/my-parent-chart" \
    --set replicatedAppEndpoint="https://enterprise.slackernews.app"
```

**Note**: you can set the above values in the `values.yaml` file instead of using the `--set` flag for each field.

## Enabling Replicated SDK "integration" mode
When using a `Development` license, the Replicated SDK will initiate in integration mode. If you are performing a Helm install/upgrade using the replicated Helm chart, you can utilize the following values in the chart YAML for the Replicated SDK's integration mode:
```yaml
integration:
  licenseID: "development-license-id"
  mock:
    enabled: true
    data: |
      helmChartURL: oci://registry.replicated.com/dev-app/dev-channel/dev-parent-chart
      currentRelease:
        versionLabel: 0.1.7
        releaseNotes: "test"
        createdAt: "2012-09-09"
        helmReleaseName: dev-parent-chart
        helmReleaseRevision: 2
        helmReleaseNamespace: default   
```

To enable the Replicated SDK's `integration` mode, you can use the following values in the chart YAML:
- `licenseID`: This should be set to the development license ID obtained from the vendor portal.
- `mock.enabled`: When this field is set to `true`, the SDK will return mocked data.
- `mock.data`: This field allows you to override the default mock data the Replicated SDK returns when `mock.enabled` is set to `true`.

Below is an example demonstrating all the supported values for the `mock.data` field:
```yaml
helmChartURL: oci://registry.replicated.com/dev-app/dev-channel/dev-parent-chart
currentRelease:
  versionLabel: 0.1.3
  releaseNotes: "release notes 0.1.3"
  createdAt: 2023-05-23T20:58:07Z
  deployedAt: 2023-05-23T21:58:07Z
  helmReleaseName: dev-parent-chart
  helmReleaseRevision: 3
  helmReleaseNamespace: default
deployedReleases:
- versionLabel: 0.1.1
  releaseNotes: "release notes 0.1.1"
  createdAt: 2023-05-21T20:58:07Z
  deployedAt: 2023-05-21T21:58:07Z
  helmReleaseName: dev-parent-chart
  helmReleaseRevision: 1
  helmReleaseNamespace: default
- versionLabel: 0.1.2
  releaseNotes: "release notes 0.1.2"
  createdAt: 2023-05-22T20:58:07Z
  deployedAt: 2023-05-22T21:58:07Z
  helmReleaseName: dev-parent-chart
  helmReleaseRevision: 2
  helmReleaseNamespace: default
- versionLabel: 0.1.3
  releaseNotes: "release notes 0.1.3"
  createdAt: 2023-05-23T20:58:07Z
  deployedAt: 2023-05-23T21:58:07Z
  helmReleaseName: dev-parent-chart
  helmReleaseRevision: 3
  helmReleaseNamespace: default
availableReleases:
- versionLabel: 0.1.4
  releaseNotes: "release notes 0.1.4"
  createdAt: 2023-05-24T20:58:07Z
  deployedAt: 2023-05-24T21:58:07Z
- versionLabel: 0.1.5
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

**Note** The endpoint *POST* `/api/v1/mock-data` exclusively supports full data posts, meaning that if any updates are required for the mock data, the entire dataset must be sent to the endpoint via the `POST` method.

### Replicated SDK "integration" mode for staging/okteto environments
**Note**: Please don't document this in customer facing docs.

Replicated SDK supports `replicatedAppEndpoint` helm value which can be used to provide a custom replicated app endpoint.
This helm value will be handy for replicants when working with staging/okteto environment deployments.

In order to use staging/okteto license IDs in `integration` mode, you would have to provide the replicated app endpoint.
You can do so by providing the `replicatedAppEndpoint` value with the staging/okteto replicated app url.
e.g.: for staging licenses you can set the replicated app endpoint as below in `values.yaml`:
```yaml
replicatedAppEndpoint: "https://staging.replicated.app"
integration:
  licenseID: "development-license-id"
  mock:
    enabled: true
```

## Release process
1. Compare the commits between the previous tag and the current commit on the main branch.
2. Share the details of the commit differences by posting a note on the Slack channels [#production-system](https://replicated.slack.com/archives/C0HFCF4JE) and [#wg-builders-plan](https://replicated.slack.com/archives/C0522NKK988).
3. Generate a new tag for the commits and proceed to push the tag to the repository using the following commands:
eg:
```bash
  SDK_TAG="v0.0.1-alpha.16"
  git checkout main && git pull
  git tag $SDK_TAG
  git push -u origin $SDK_TAG
```
4. Ensure that the GitHub actions associated with the newly created tag are executed, and verify that the updated Helm charts are successfully published to both the staging and production replicated registry.
5. Make sure to update the [Replicated SDK Documentation](https://docs.google.com/document/d/1KJo9MfwS9sfu13p-Lyf3W4KRDBxNcAQPuOOYoHnQVis/edit#heading=h.pmlj79z5v9zk) by replacing all instances of the Replicated SDK version with the latest tag.

