# Legacy Image Verification (Releases Before 1.19.9)

This directory contains the public keys and verification script used by the
legacy release pipeline. Use this procedure only for Replicated SDK releases
prior to `1.19.9`.

Starting with `1.19.9`, stable releases are signed for production by SecureBuild
using Sigstore keyless signing. Follow the current inline verification commands
in [`RELEASE.md`](../RELEASE.md#verification) for those releases. The public
keys and script in this directory do not verify SecureBuild signatures.

## Contents

### Verification Script

- `verify-image.sh` - Verifies legacy SLSA attestations, image signatures, and
  SBOMs

### Public Keys

- `cosign-dev.pub` - Legacy development signing key
- `cosign-stage.pub` - Legacy staging signing key
- `cosign-prod.pub` - Legacy production signing key

## Usage

Run the script from this directory so it can find the legacy public keys:

```bash
cd certs
./verify-image.sh --env <environment> --version <version> --digest <image-digest>
```

To display the decoded SBOM, add `--show-sbom`. For detailed usage instructions
and examples, run:

```bash
./verify-image.sh --help
```

## Legacy Environment-Specific Verification

- **Development**: Uses `cosign-dev.pub` for signature verification
- **Staging**: Uses `cosign-stage.pub` for signature verification
- **Production**: Uses `cosign-prod.pub` for signature verification

These environments describe how images were signed by the legacy pipeline; they
do not apply to SecureBuild releases starting with `1.19.9`.

## Security Notes

- The public keys in this directory are used only for verification.
- Always verify an immutable digest rather than a mutable image tag.
