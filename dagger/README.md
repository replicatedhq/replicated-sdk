# Dagger Pipeline Usage

## Build Architecture Overview

This Dagger pipeline supports two build approaches:

### 🚀 **SecureBuild** (Default for staging/production)
- **What**: Zero-CVE container images built by SecureBuild.com
- **Benefits**: Enhanced security, compliance, automatic CVE remediation
- **Requirements**: SecureBuild account + 1Password access
- **Registries**: Docker Hub + Replicated registries + CVE0.io

### 🛠️ **Wolfi/TTL.sh** (Fallback for development)  
- **What**: Traditional Wolfi-based images for development
- **Benefits**: No credentials required
- **Requirements**: None (public repositories only)
- **Registries**: TTL.sh 

---

## 🔧 **Environment Configuration**

### SecureBuild Mode (Recommended)
```bash
export USE_SECUREBUILD=true
export OP_SERVICE_ACCOUNT="your-1password-service-account-token"
export OP_SERVICE_ACCOUNT_PRODUCTION="your-production-1password-token"  # for production builds
```

### Traditional Mode (Development Fallback)
```bash
# No environment variables needed - will use deploy/apko-wolfi.yaml
# Images published to ttl.sh with 24h expiration
```

---

## Validation

To validate the codebase without publishing:
```bash
dagger call validate --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT
```

## Publishing

The publish pipeline supports different environments and configurations:

> **Note**: All version numbers in examples below (e.g., `1.0.0-dev.1`, `1.0.0-beta.1`, `1.0.0`) are for illustration purposes only. Use your actual release version numbers when running commands.

### Development Release (ttl.sh)

#### Option 1: SecureBuild Development (with credentials)
In this scenario, the pipeline uses SecureBuild and will publish the image to ttl.sh
```bash
export USE_SECUREBUILD=true
dagger call publish --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT \
    --dev=true \
    --staging=false \
    --version=1.0.0-dev.1 \
    --slsa=false
```
**Result**: Uses `deploy/apko-securebuild.yaml` → publishes to TTL.sh + CVE0.io
In this scenario, the pipeline uses Wolfi and will publish to ttl.sh

#### Option 2: Traditional Wolfi Development (no credentials) 
```bash
# DO NOT set USE_SECUREBUILD
dagger call publish --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT \
    --dev=true \
    --staging=false \
    --version=1.0.0-dev.1 \
    --slsa=false
```  
**Result**: Uses `deploy/apko-wolfi.yaml` → publishes to TTL.sh only

### Staging Release (SecureBuild Required)

For publishing to staging environment:
```bash
export USE_SECUREBUILD=true  # Required for staging builds
dagger call publish --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT \
    --op-service-account-production=env:OP_SERVICE_ACCOUNT_PRODUCTION \
    --dev=false \
    --staging=true \
    --version=1.0.0-beta.1
```
**Result**: Uses SecureBuild staging → publishes to:
- `docker.io/replicated/replicated-sdk:1.0.0-beta.1`
- `registry.staging.replicated.com/library/replicated-sdk-image:1.0.0-beta.1`  
- `cve0.io/replicated-sdk:1.0.0-beta.1`

### Production Release (SecureBuild Required)

For publishing to production:
```bash
export USE_SECUREBUILD=true  # Required for production builds
dagger call publish --progress=plain \
    --op-service-account=env:OP_SERVICE_ACCOUNT \
    --op-service-account-production=env:OP_SERVICE_ACCOUNT_PRODUCTION \
    --dev=false \
    --staging=false \
    --version=1.0.0
```
**Result**: Uses SecureBuild production → publishes to:
- `docker.io/replicated/replicated-sdk:1.0.0`
- `registry.replicated.com/library/replicated-sdk-image:1.0.0`
- `cve0.io/replicated-sdk:1.0.0`

> **Note**: SecureBuild automatically handles SLSA attestations and SBOM generation, so `--slsa` and `--github-token` flags are not required (and are ignored) when using SecureBuild.

---

## 🔐 **SecureBuild Integration**

### Requirements for SecureBuild
1. **SecureBuild Account**: Active account at SecureBuild.com
2. **Team Configuration**:
   - Feature flags enabled for "Custom APKO Upload" and "Custom Melange Upload"
   - Team subscribed to the image/catalog item in SecureBuild
3. **1Password Access**: Service account tokens for dev/production vaults
4. **Credentials**: Proper 1Password items configured:
   - `SecureBuild-Dev-Token` (in developer automation vault)
   - `SecureBuild-Replicated-SDK-Prod-Token` (in production vault)

### SecureBuild Benefits
- **Zero CVE Images**: Automatically patched base images
- **Multi-Registry Publishing**: Single build → multiple registries  
- **Enhanced Security**: SLSA attestations + SBOM generation
- **Compliance Ready**: Meets enterprise security requirements

### Image Naming Architecture
SecureBuild handles different registry naming requirements:
- **Docker Hub**: Uses `replicated-sdk` image name
- **Replicated Registries**: Uses `replicated-sdk-image` image name  
- **CVE0.io**: Uses `replicated-sdk` image name

This ensures compatibility with existing Docker Hub users and Replicated Helm charts.

### Configuration Files
- **SecureBuild**: Uses `deploy/apko-securebuild.yaml`
- **Traditional**: Uses `deploy/apko-wolfi.yaml`
- **Selection**: Automatic based on `USE_SECUREBUILD` environment variable

### Current Limitations
**Package Dependency Requirement**: The SecureBuild pipeline dynamically modifies the APKO configuration:
- Version substitution: `VERSION: 1.0.0` → `VERSION: "your-version"`
- Package substitution: `- replicated` → `- replicated-your-version`

**Important**: The versioned package (e.g., `replicated-1.0.0-beta.1`) must already exist in the CVE0.io repository before the APKO build. The current pipeline does not automatically create these packages via melange submission.

### Troubleshooting SecureBuild
Common issues and solutions:

**403 Forbidden Errors**:
- Check 1Password service account has vault access
- Verify SecureBuild team has "Custom APKO Upload" feature flag
- Confirm API tokens are valid in 1Password items

**Missing Package Errors**:  
- SecureBuild uses CVE0.io repository for packages
- Package `replicated-{version}` must exist before APKO build
- **Current workaround**: Manually create packages in SecureBuild before running builds
- **Future enhancement**: Automatic melange.yaml submission for package generation

**Build Timeouts**:
- Monitor build status in SecureBuild dashboard
- Check Dagger traces for detailed timing

---

## Available Flags

- `--progress=plain`: Show detailed progress output
- `--op-service-account`: 1Password service account token for staging access
- `--op-service-account-production`: 1Password service account token for production access
- `--dev`: (boolean) Use development environment (ttl.sh)
- `--staging`: (boolean) Use staging environment
- `--version`: Version string for the release
- `--slsa`: (boolean) Enable SLSA provenance generation *(ignored when using SecureBuild)*
- `--github-token`: GitHub token for SLSA workflow access *(ignored when using SecureBuild)*

**Note**: When using SecureBuild (`USE_SECUREBUILD=true`), SLSA attestations and SBOM generation are automatic. The `--slsa` and `--github-token` flags are only used with the traditional Chainguard pipeline.

## SBOM Generation

Software Bill of Materials (SBOM) is automatically generated during the build process:

1. **Generation Method**: 
   - **SecureBuild**: Automatically generates comprehensive SBOMs with CVE remediation data
   - **Traditional**: Uses Chainguard's melange and apko to create SPDX-formatted SBOMs
   - SBOMs include all direct and transitive dependencies with license information

2. **SBOM Format**:
   - Format: SPDX JSON
   - Contains:
     - Package information
     - License details
     - Dependency relationships
     - Creation metadata

3. **Verification**:
   - Refer to the instructions at securebuild.com

4. **Viewing SBOM**:
   - Refer to the instructions at securebuild.com

5. **Image Attestation**
   - Refer to the instructions at securebuild.com

---

## 🔄 **Migration Notes**

### GitHub Actions Integration
As of September 2025, GitHub Actions workflows automatically use SecureBuild for staging and production builds:
- **Environment Variable**: `USE_SECUREBUILD=true` is set in `.github/workflows/publish.yml`
- **Backward Compatibility**: Wolfi based pipeline remains as fallback if flag is removed
- **No Breaking Changes**: All existing interfaces and outputs remain unchanged

### For Contributors  
- **Development builds**: Can still use TTL.sh without any SecureBuild credentials
- **No setup required**: Traditional Wolfi builds work without configuration
- **Optional SecureBuild**: Set `USE_SECUREBUILD=true` only if you have access
