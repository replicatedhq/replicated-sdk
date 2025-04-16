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

The release process is handled by the `Publish` function in `dagger/publish.go`. Here's how it works:

1. **Package Building (Melange)**:
   - The process starts by building packages using Chainguard's melange
   - Version strings are sanitized for Alpine/Wolfi compatibility (e.g., `1.6.0-beta.5` becomes `1.6.0_beta.5-r0`)
   - Packages are built for both AMD64 and ARM64 architectures
   - The melange build process:
     ```go
     // Updates melange.yaml with correct version
     melangeYaml = strings.Replace(melangeYaml, "version: 1.0.0", fmt.Sprintf("version: %s", packageVersion), 1)
     
     // Builds packages for both architectures
     amdPackages := dag.Melange().Build(source.File("deploy/melange.yaml"), dagger.MelangeBuildOpts{
         SourceDir: source,
         Arch:      "x86_64",
     })
     armPackages := dag.Melange().Build(source.File("deploy/melange.yaml"), dagger.MelangeBuildOpts{
         SourceDir: source,
         Arch:      "aarch64",
     })
     ```

2. **Container Image Building (Apko)**:
   - Uses Chainguard's apko to build the final container image
   - The apko.yaml is updated with:
     - The correct package version constraint
     - Environment variables
   - Images are built and published to multiple registries:
     ```go
     // Updates apko.yaml with package version
     apkoYaml = strings.Replace(
         apkoYaml,
         "    - replicated\n",
         fmt.Sprintf("    - replicated=%s-r0\n", sanitizeVersionForMelange(version)),
         1,
     )
     ```

3. **Image Publishing**:
   The process publishes images to three different registries based on the build type:
   - Development (if enabled):
     ```
     ttl.sh/replicated/replicated-sdk:${VERSION}
     ```
   - Staging (if enabled):
     ```
     index.docker.io/replicated/replicated-sdk:${VERSION}
     registry.staging.replicated.com/library/replicated-sdk-image:${VERSION}
     ```
   - Production (if enabled):
     ```
     index.docker.io/replicated/replicated-sdk:${VERSION}
     registry.replicated.com/library/replicated-sdk-image:${VERSION}
     ```

4. **Helm Chart Publishing**:
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

5. **SLSA Provenance**:
   If SLSA provenance is enabled, the process triggers a GitHub workflow:
   ```go
   if slsa {
       ctr := dag.Gh().
           Run(fmt.Sprintf(`api /repos/replicatedhq/replicated-sdk/actions/workflows/slsa.yml/dispatches \
               -f ref=main \
               -f inputs[digest]=%s`, digest),
               dagger.GhRunOpts{
                   Token: githubToken,
               },
           )
   }
   ```

### Security and Attestations

Each release includes:
- SLSA provenance attestation for all container images
- Daily security scans using Grype
- Automated vulnerability reporting

## Verification

After a release is published, verify:

1. Docker Image:
   ```bash
   docker pull registry.replicated/replicated-sdk-image:X.Y.Z
   ```

2. Helm Charts:
   ```bash
   # List chart versions
   helm registry login registry.replicated.com
   helm search repo replicated --versions
   
   # Pull chart
   helm pull oci://registry.replicated.com/library/replicated --version X.Y.Z
   ```

## Troubleshooting

If the release workflow fails:

1. Check the GitHub Actions logs for errors
2. Common issues:
   - Failed tests
   - Docker registry authentication issues
   - Helm chart validation failures
   - Version format issues with melange/apko builds

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


Note: Always prefer forward fixes over rollbacks when possible.
