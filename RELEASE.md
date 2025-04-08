# Release Process

This document outlines the process for creating and publishing new releases of the Replicated SDK.

## Overview

The release process is automated through GitHub Actions. When a new version tag is pushed to the repository, it triggers a workflow that:
1. Builds and tests the code
2. Creates a Docker image
3. Builds and publishes Helm charts
4. Generates release notes
5. Creates security attestations

## Creating a New Release

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
   - Create and push a Docker image to DockerHub
   - Build and publish Helm charts
   - Generate release notes
   - Create a GitHub release

## Release Artifacts

### Docker Image

The Docker image is published to multiple registries:

1. DockerHub:
   - Repository: `index.docker.io/replicated/replicated-sdk`
   - Tag: matches the git tag (e.g., `1.2.3`)
   - Includes SLSA provenance attestation and SBOM

2. Staging Registry:
   - Repository: `registry.staging.replicated.com/library/replicated-sdk`
   - Tag: matches the git tag (e.g., `1.2.3`)

3. Production Registry:
   - Repository: `registry.replicated.com/library/replicated-sdk`
   - Tag: matches the git tag (e.g., `1.2.3`)

Note: SLSA provenance attestation and SBOM are only generated for the DockerHub image.

### Helm Charts

The Helm charts are published to two registries:

1. Staging Registry:
   ```
   registry.staging.replicated.com/library
   ```

2. Production Registry:
   ```
   registry.replicated.com/library
   ```


## Verification

After a release is published, verify:

1. Docker Image:
   ```bash
   docker pull replicated/replicated-sdk:X.Y.Z
   ```

2. Helm Charts:
   ```bash
   # List chart versions
   helm registry login registry.replicated.com
   helm search repo replicated --versions
   
   # Pull chart
   helm pull oci://registry.replicated.com/library/replicated --version X.Y.Z
   ```

## Security

Each release includes:
- SLSA provenance attestation for all container images (DockerHub, staging, and production)
- Daily security scans using Grype
- Automated vulnerability reporting

## Troubleshooting

If the release workflow fails:

1. Check the GitHub Actions logs for errors
2. Common issues:
   - Failed tests
   - Docker registry authentication issues
   - Helm chart validation failures

## Release Notes

Release notes are automatically generated and include:
- Changes since the previous release
- Breaking changes (if any)
- New features
- Bug fixes
- Dependencies updates

A PR for updating the documentation is automatically created in the replicated-docs repository.

## Post-Release

After a successful release:

1. Verify the GitHub release is created
2. Check the documentation PR in replicated-docs
3. Monitor for any reported issues
4. Update the changelog if necessary

## Support

If you encounter issues with the release process:
1. Check the GitHub Actions logs
2. Review the error messages
3. Contact the maintainers team

## Rolling Back

If issues are discovered after release:

1. Tag and push a new patch release with fixes
2. In emergencies, you can revert to the previous version:
   ```bash
   # For Docker
   docker pull replicated/replicated-sdk:PREVIOUS_VERSION
   
   # For Helm
   helm rollback RELEASE_NAME REVISION_NUMBER
   ```

Note: Always prefer forward fixes over rollbacks when possible.
