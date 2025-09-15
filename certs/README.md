# Certificates and Verification Tools

> **⚠️ DEPRECATION NOTICE**: This directory is transitioning away from manual image verification tools as SecureBuild now automatically handles SLSA attestations, SBOM generation, and image signing for staging and production builds. See migration notes below.

This directory contains legacy certificates and verification tools that were used for validating the authenticity and integrity of Replicated SDK container images.

## 🔄 Migration to SecureBuild

**As of September 2025**, the Replicated SDK build pipeline uses [SecureBuild](https://securebuild.com) for staging and production image builds, which provides:

- **Automatic SLSA Attestation**: Built-in provenance generation without manual workflows
- **Automatic SBOM Generation**: Comprehensive software bill of materials with CVE remediation data
- **Automatic Image Signing**: Cryptographic signatures managed by SecureBuild
- **Multi-Registry Publishing**: Images published to Docker Hub, Replicated registries, and CVE0.io

### Current Build Architecture

- **Development Builds**: May still use traditional pipeline with manual verification (optional SecureBuild)
- **Staging/Production Builds**: Use SecureBuild with automatic security attestations
- **Environment Flag**: Controlled by `USE_SECUREBUILD=true` in GitHub Actions

## 📋 Legacy Contents (For Reference)

### Verification Script
- `verify-image.sh` - **Legacy script** for manual SLSA attestation, image signatures, and SBOM verification

### Public Keys
- `cosign-dev.pub` - **Legacy**: Development environment public key for image signature verification
- `cosign-stage.pub` - **Legacy**: Staging environment public key for image signature verification  
- `cosign-prod.pub` - **Legacy**: Production environment public key for image signature verification

## 🔧 Verification Methods

### For SecureBuild Images (Recommended)
SecureBuild images include built-in attestations and can be verified using SecureBuild's verification tools. Refer to the [SecureBuild documentation](https://securebuild.com) for current verification methods.

### For Legacy Images (Development/Fallback)
If using traditional pipeline builds, the legacy verification script can still be used:

```bash
./verify-image.sh --env <environment> --version <version> --digest <image-digest>
```

For detailed usage instructions:
```bash
./verify-image.sh --help
```

## ⚠️ Important Notes

1. **Production Images**: All production images now use SecureBuild attestations - legacy verification may not apply
2. **Staging Images**: All staging images now use SecureBuild attestations - legacy verification may not apply  
3. **Development Images**: May use either SecureBuild or legacy pipeline depending on `USE_SECUREBUILD` flag
4. **Future Plans**: This directory may be removed in future releases once SecureBuild migration is complete

## 📚 Documentation References

- **Current Build Process**: See `/dagger/README.md` for comprehensive SecureBuild integration guide
- **SecureBuild Benefits**: Zero-CVE images, enhanced security, compliance-ready attestations
- **Migration Guide**: All GitHub Actions workflows automatically use SecureBuild for staging/production
