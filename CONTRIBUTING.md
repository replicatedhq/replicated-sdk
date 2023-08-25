## Development

Build and push the replicated Docker image and Helm chart to ttl.sh:

```shell
make build-ttl.sh
```

**Note**: The above command will also output the .tgz Helm chart under `chart/replicated-0.0.0.tgz`.

You can then install the Helm chart using one of the following options:

**Option 1 (Integration mode)**:
This is the quickest way to get replicated up and running out of the box.

Notes:
- When replicated runs in integration mode, it returns mocked data instead of real data except for `/license` endpoints.
- Only a `Development` license ID is required to run replicated in integration mode.
- Integration mode is enabled by default when using a `Development` license.
- Integration mode is only supported for `Development` licenses.
- When using a license ID from a staging/okteto environment, the `replicatedAppEndpoint` field must be set accordingly. For example: `--set replicatedAppEndpoint=https://staging.replicated.app`.

To install replicated in integration mode, run the following command:
```bash
helm install replicated oci://ttl.sh/[USER]/replicated --version 0.0.0 --set integration.licenseID=[DEV_LICENSE_ID]
```

**Option 2 (Production mode)**:

Use the following command to install replicated in production mode:

**Note**: if using a `Development` license, disable integration mode by passing `--set integration.enabled=false` as well.

```shell
helm install replicated oci://ttl.sh/[USER]/replicated \
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
    --set replicatedAppEndpoint=[REPLICATED_APP_ENDPOINT] \
    --set statusInformers=[STATUS_INFORMERS]
```

Example:
```shell
helm install replicated oci://ttl.sh/salah/replicated \
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
    --set replicatedAppEndpoint="https://enterprise.slackernews.app" \
    --set statusInformers="{default/deployment/nginx,default/statefulset/rqlite}"
```

**Note**: you can set the above values in the `values.yaml` file instead of using the `--set` flag for each field.

## Testing

Tests are automatically run in GitHub Actions after opening or updating a pull request.

Unit and Pact tests can be run locally using the `make test` command.

Pact tests live in the `pact/` directory at the root of the repository. The Pact standalone command line executable must be installed to run Pact tests locally. It can be downloaded from the releases page in the following repository: https://github.com/pact-foundation/pact-ruby-standalone.

## Release process
1. Compare the commits between the previous tag and the current commit on the main branch.
2. Share the details of the commit differences by posting a note on the Slack channels [#production-system](https://replicated.slack.com/archives/C0HFCF4JE) and [#wg-builders-plan](https://replicated.slack.com/archives/C0522NKK988).
3. Generate a new tag for the commits and proceed to push the tag to the repository using the following commands:
eg:
```bash
  SDK_TAG="v0.0.1-beta.1"
  git checkout main && git pull
  git tag $SDK_TAG
  git push -u origin $SDK_TAG
```
4. Ensure that the GitHub actions associated with the newly created tag are executed, and verify that the updated Helm charts are successfully published to both the staging and production replicated registry.
5. Make sure to update the [Replicated SDK Documentation](https://docs.replicated.com/vendor/replicated-sdk-overview) by replacing all instances of the Replicated SDK version with the latest tag.
