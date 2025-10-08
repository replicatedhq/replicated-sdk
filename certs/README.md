# Certificates and Verification Tools

This directory contains certificates and verification tools used for validating the authenticity and integrity of Replicated SDK container images.

## Contents

### Verification Script
- `verify-image.sh` - Script to verify SLSA attestation, image signatures, and SBOM for SDK images

### Public Keys
- `cosign-dev.pub` - Development environment public key for image signature verification
- `cosign-stage.pub` - Staging environment public key for image signature verification
- `cosign-prod.pub` - Production environment public key for image signature verification

## Usage

The verification script supports three environments (dev, stage, prod) and can be run as follows:

```bash
./verify-image.sh --env <environment> --version <version> --digest <image-digest>
```

For detailed usage instructions and examples, run:
```bash
./verify-image.sh --help
```

## Environment-Specific Verification

- **Development**: Uses `cosign-dev.pub` for signature verification
- **Staging**: Uses `cosign-stage.pub` for signature verification
- **Production**: Uses `cosign-prod.pub` for signature verification

## Security Notes

- The public keys in this directory are used only for verification
