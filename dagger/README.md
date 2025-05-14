# Dagger Pipeline Usage

## Validation

To validate the codebase without publishing:
```bash
dagger call validate --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT
```

## Publishing

The publish pipeline supports different environments and configurations. Here are the common usage patterns:

### Development Release (ttl.sh)

For local testing using ttl.sh registry:
```bash
dagger call publish --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT \
    --dev=true \
    --staging=false \
    --version=1.6.0-dev.1 \
    --slsa=false
```

### Staging Release

For publishing to staging environment:
```bash
dagger call publish --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT \
    --op-service-account-production=env:OP_SERVICE_ACCOUNT_PRODUCTION \
    --dev=false \
    --staging=true \
    --version=1.6.0-beta.5 \
    --slsa=true \
    --github-token=env:GITHUB_TOKEN
```

### Production Release

For publishing to production:
```bash
dagger call publish --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT \
    --op-service-account-production=env:OP_SERVICE_ACCOUNT_PRODUCTION \
    --dev=false \
    --staging=false \
    --version=1.6.0 \
    --slsa=true \
    --github-token=env:GITHUB_TOKEN
```

## Available Flags

- `--progress=plain`: Show detailed progress output
- `--op-service-account`: 1Password service account token for staging access
- `--op-service-account-production`: 1Password service account token for production access
- `--dev`: (boolean) Use development environment (ttl.sh)
- `--staging`: (boolean) Use staging environment
- `--version`: Version string for the release
- `--slsa`: (boolean) Enable SLSA provenance generation
- `--github-token`: GitHub token for SLSA workflow access

Note: SLSA provenance generation requires both `--slsa=true` and a valid `--github-token`.

## SBOM Generation

Software Bill of Materials (SBOM) is automatically generated during the build process:

1. **Generation Method**: 
   - Uses Chainguard's melange and apko to create SPDX-formatted SBOMs
   - SBOMs are generated for both the base image and application layers
   - Includes all direct and transitive dependencies

2. **SBOM Format**:
   - Format: SPDX JSON
   - Contains:
     - Package information
     - License details
     - Dependency relationships
     - Creation metadata

3. **Verification**:
   - SBOMs are signed using cosign
   - Attestations are attached to the container image
   - Use the verification script in `certs/verify-image.sh` to verify:
     - SLSA provenance
     - Image signatures
     - SBOM attestations
   ```bash
   ./certs/verify-image.sh --env <dev|stage|prod> --version <version> --digest <image-digest>
   ```

4. **Viewing SBOM**:
   - Full SBOM content can be viewed using the `--show-sbom` flag with verify-image.sh:
     ```bash
     ./certs/verify-image.sh --env <dev|stage|prod> --version <version> --digest <image-digest> --show-sbom
     ```

## Image Attestation

- Images pushed to Dockerhub are attested using cosign
- This provides an additional layer of verification for public images
- The attestation includes:
  - SBOM data
  - Build provenance
  - Image signatures
