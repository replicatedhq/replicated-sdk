# Release Process

This document outlines the process for creating and publishing new releases of the Replicated SDK.

## Overview

The release process is automated through GitHub Actions. When a new version tag is pushed to the repository, it triggers a workflow that:
1. Builds and tests the code
2. Creates a Docker image using Chainguard's melange and apko
3. Builds and publishes Helm charts
4. Generates release notes
5. Creates security attestations

## Creating a New Release

### Required Credentials

To create releases, you'll need:

1. **1Password Service Account Tokens**:
   - For staging environment access
   - For production environment access

2. **GitHub Token**:
   - With permissions to trigger the SLSA GitHub workflow

### Version Tag Format

Supported version tag formats (without 'v' prefix):
- Release: `X.Y.Z` (e.g., `1.2.3`)
- Beta: `X.Y.Z-beta` or `X.Y.Z-beta.N` (e.g., `1.2.3-beta` or `1.2.3-beta.1`)
- Alpha: `X.Y.Z-alpha` or `X.Y.Z-alpha.N` (e.g., `1.2.3-alpha` or `1.2.3-alpha.1`)

Note: While older beta releases used a 'v' prefix (e.g., v1.0.0-beta.28), current releases do not use this prefix.

### Steps to Release

1. Create and push a new tag:
   ```bash
   git tag X.Y.Z
   git push origin X.Y.Z
   ```

2. The GitHub Actions workflow will automatically:
   - Run all tests
   - Build the Go binaries
   - Create and push Docker images
   - Build and publish Helm charts
   - Generate release notes
   - Create a GitHub release

## Release Artifacts and Process

### Build and Publish Process

The release workflow uses the specs in `securebuild/package` and `securebuild/image` as the single source of truth:

1. **Stable image releases** call `.github/workflows/publish-securebuild.yml`. SecureBuild builds the `replicated-sdk` package and the `replicated-sdk` and `replicated-sdk-fips` images.
2. **Prerelease images** are built locally by Dagger, which adapts those same specs in memory. This preserves the existing prerelease registry credentials while following the KOTS stable-service/prerelease-local split.
3. **Development and E2E images** use that same Dagger adapter; there are no separate Melange or APKO specs under `deploy/`.
4. **Registry destinations** previously used by the release pipeline were:
   - Public Docker Hub: `index.docker.io/replicated/replicated-sdk:${VERSION}`
   - Staging Replicated registry: `registry.staging.replicated.com/library/replicated-sdk-image:${VERSION}`
   - Production Replicated registry: `registry.replicated.com/library/replicated-sdk-image:${VERSION}`

   SecureBuild also publishes the FIPS variants using the same repositories with a `${VERSION}-fips` tag.

   Configure the SecureBuild `replicated-sdk` image to publish to all three destinations. Configure an additional destination for `replicated-sdk-fips` wherever that new image should be distributed; the legacy pipeline did not publish a FIPS image.

5. **Helm Chart Publishing**:
   The process builds and publishes Helm charts to both staging and production registries:
   - Updates values.yaml with correct version and registry information
   - Publishes to:
     - Staging: `registry.staging.replicated.com/library`
     - Production: `registry.replicated.com/library`
   ```go
   // Example of chart publishing process
   ctr := dag.Container().From("alpine/helm:latest").
       WithMountedDirectory("/source", source).
       WithWorkdir("/source/chart").
       WithNewFile("/source/chart/values.yaml", valuesYaml).
       WithEnvVariable("HELM_USERNAME", username).
       WithSecretVariable("HELM_PASSWORD", password).
       WithExec([]string{"helm", "dependency", "update"}).
       WithExec([]string{"helm", "package", "--version", version, "--app-version", version, "."}).
       WithExec([]string{"helm", "registry", "login", "registry.replicated.com", "--username", username, "--password", password}).
       WithExec([]string{"helm", "push", helmChartFilename, "oci://registry.replicated.com/library"})
   ```

6. **Image provenance** is produced by SecureBuild for stable release images. The legacy Dagger SLSA dispatch is bypassed when `--skip-image=true` because Dagger no longer owns those image digests.

### Security and Attestations

Each release includes:
- SLSA provenance attestation for all container images
- Daily security scans using Grype
- Automated vulnerability reporting

## Verification

After a release is published, verify:

1. Docker Image:
   ```bash
   docker pull registry.replicated.com/library/replicated-sdk-image:X.Y.Z
   ```

2. Helm Charts:
   ```bash
   # List chart versions
   helm registry login registry.replicated.com
   helm search repo replicated --versions
   
   # Pull chart
   helm pull oci://registry.replicated.com/library/replicated --version X.Y.Z
   ```

3. Image signatures and attestations:

   Releases starting with `1.19.9` are signed for production by SecureBuild
   using Sigstore keyless signing. A tag can be used to verify the image it
   currently references:

   ```bash
   cosign verify \
     --certificate-identity='sb-attestor@cve0-issuer.iam.gserviceaccount.com' \
     --certificate-oidc-issuer='https://accounts.google.com' \
     registry.replicated.com/library/replicated-sdk-image:X.Y.Z
   ```

   For immutable verification, use the image digest instead:

   ```bash
   cosign verify \
     --certificate-identity='sb-attestor@cve0-issuer.iam.gserviceaccount.com' \
     --certificate-oidc-issuer='https://accounts.google.com' \
     registry.replicated.com/library/replicated-sdk-image@sha256:DIGEST
   ```

   Append `-fips` to the version tag to verify the FIPS image. For the
   attestation commands below, replace `IMAGE_REFERENCE` with either the tagged
   or digest-qualified image reference.

   Verify the SLSA provenance attestation:

   ```bash
   cosign verify-attestation \
     --certificate-identity='sb-attestor@cve0-issuer.iam.gserviceaccount.com' \
     --certificate-oidc-issuer='https://accounts.google.com' \
     --type='https://slsa.dev/provenance/v1' \
     IMAGE_REFERENCE
   ```

   Verify the SPDX SBOM attestation:

   ```bash
   cosign verify-attestation \
     --certificate-identity='sb-attestor@cve0-issuer.iam.gserviceaccount.com' \
     --certificate-oidc-issuer='https://accounts.google.com' \
     --type='https://spdx.dev/Document' \
     IMAGE_REFERENCE
   ```

   Cosign uses the Sigstore Public Good trust root by default. No public key
   file is required for these releases.

   For releases prior to `1.19.9`, use the legacy verification procedure in
   [`certs/README.md`](certs/README.md).

## Troubleshooting

If the release workflow fails:

1. Check the GitHub Actions logs for errors
2. Common issues:
   - Failed tests
   - Docker registry authentication issues
   - Helm chart validation failures
   - Version format issues with melange/apko builds
   - Missing or invalid credentials
   - Insufficient GitHub token permissions

## Post-Release

After a successful release:

1. Verify the GitHub release is created
2. Check the documentation PR in replicated-docs
3. Monitor for any reported issues
4. Update the changelog if necessary

## Support

If you encounter issues with the release process:
1. Check the GitHub Actions logs
2. Review the workflow error messages
3. Contact the maintainers team

## Rolling Back

If issues are discovered after release:

1. Tag and push a new patch release with fixes

Note: Always prefer forward fixes over rollbacks when possible.
