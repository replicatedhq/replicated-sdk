#!/bin/bash

# Help function
show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo "Verify SLSA attestation and SBOM for Replicated SDK"
    echo
    echo "Options:"
    echo "  -e, --env ENV     Environment to verify (dev|stage|prod) [Required]"
    echo "  -v, --version VER Version to verify [Required]"
    echo "  -d, --digest DIG  Image digest to verify [Required]"
    echo "  --show-sbom        Show full SBOM content"
    echo "  -h, --help        Show this help message"
    echo
    echo "Examples:"
    echo "  $0 --env dev --version 1.5.3-beta.3 --digest sha256:5b064832df6bfb934c081fa0263134bc9845525211f09a752d5684306310f3c5"
    echo "  $0 --env stage --version 1.5.3-beta.3 --digest sha256:7cb8e0c8e0fba8e4a7157b4fcef9e7345538f7543a4e5925bb8b30c9c1375400"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--env)
            ENV="$2"
            shift 2
            ;;
        -v|--version)
            TEST_VERSION="$2"
            shift 2
            ;;
        -d|--digest)
            DIGEST="$2"
            shift 2
            ;;
        --show-sbom)
            SHOW_SBOM=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Validate required arguments
if [ -z "$ENV" ] || [ -z "$TEST_VERSION" ] || [ -z "$DIGEST" ]; then
    echo "Error: Environment (-e), version (-v), and digest (-d) are required"
    show_help
    exit 1
fi

# Validate environment
if [[ ! "$ENV" =~ ^(dev|stage|prod)$ ]]; then
    echo "Error: Environment (-e, --env) must be one of: dev, stage, prod"
    exit 1
fi

# Set environment-specific variables
case $ENV in
    dev)
        REGISTRY="ttl.sh"
        IMAGE="${REGISTRY}/replicated/replicated-sdk"
        ;;
    stage)
        REGISTRY="registry.staging.replicated.com"
        IMAGE="${REGISTRY}/library/replicated-sdk-image"
        ;;
    prod)
        REGISTRY="registry.replicated.com"
        IMAGE="${REGISTRY}/library/replicated-sdk-image"
        ;;
esac

# Set the full image reference with digest
IMAGE_WITH_DIGEST="${IMAGE}@${DIGEST}"

SOURCE_REPO=github.com/replicatedhq/replicated-sdk

echo "==============================================="
echo "Starting verification for ${IMAGE_WITH_DIGEST}"
echo "==============================================="

echo -e "\n📋 Step 1: Verifying SLSA provenance..."
if [ "$ENV" != "dev" ]; then
    # Only run SLSA verification for staging and production
    if COSIGN_REPOSITORY=${REGISTRY}/library/replicated-sdk-image slsa-verifier verify-image "${IMAGE_WITH_DIGEST}" \
      --source-uri ${SOURCE_REPO} \
      --source-tag ${TEST_VERSION} \
      --print-provenance | tee /tmp/slsa_output.json | jq -r '
        "✅ SLSA Verification: SUCCESS",
        "Build Details:",
        "  • Builder: \(.predicate.builder.id | split("@")[0])",
        "  • Organization: \(.predicate.invocation.environment.github_event_payload.organization.login)",
        "  • Last Commit Author: \(.predicate.invocation.environment.github_event_payload.head_commit.author.name)",
        "  • Last Commit Email: \(.predicate.invocation.environment.github_event_payload.head_commit.author.email)",
        "  • Release Created By: \(.predicate.invocation.environment.github_event_payload.pusher.name)",
        "  • Source: \(.predicate.invocation.configSource.uri | split("@")[0])",
        "  • Commit: \(.predicate.invocation.configSource.digest.sha1)",
        "  • Built From: \(.predicate.invocation.environment.github_ref)",
        "  • Image Digest: \(.subject[0].digest.sha256 // "N/A")",
        "  • Commit Timestamp: \(.predicate.invocation.environment.github_event_payload.head_commit.timestamp)"
      '; then
        echo "✅ SLSA verification successful"
    else
        echo "❌ SLSA verification failed"
        exit 1
    fi
else
    echo "ℹ️  SLSA verification skipped in dev mode"
fi

echo -e "\n🔏 Step 2: Verifying image signature..."
if [ "$ENV" = "dev" ]; then
    # Use cosign-dev.pub for dev mode
    if VERIFICATION_OUTPUT=$(cosign verify-attestation \
      --key certs/cosign-dev.pub \
      --type spdxjson \
      ${IMAGE_WITH_DIGEST} 2>/dev/null); then
        echo "✅ Image signature verification successful"
        echo "Signature details:"
        echo "$VERIFICATION_OUTPUT" | jq -r '
          "  • Attestation type: \(.payloadType)",
          "  • Signature timestamp: \(.optional.sig.timestamp // "N/A")"
        '
    else
        echo "❌ Image signature verification failed"
        exit 1
    fi
elif [ "$ENV" = "stage" ]; then
    # Use cosign-stage.pub for staging mode
    if VERIFICATION_OUTPUT=$(cosign verify-attestation \
      --key certs/cosign-stage.pub \
      --type spdxjson \
      ${IMAGE_WITH_DIGEST} 2>/dev/null); then
        echo "✅ Image signature verification successful"
        echo "Signature details:"
        echo "$VERIFICATION_OUTPUT" | jq -r '
          "  • Attestation type: \(.payloadType)",
          "  • Signature timestamp: \(.optional.sig.timestamp // "N/A")"
        '
    else
        echo "❌ Image signature verification failed"
        exit 1
    fi
else
    # Use keyless verification for prod
    if VERIFICATION_OUTPUT=$(cosign verify-attestation \
      --type spdxjson \
      --certificate-identity "https://github.com/replicated/replicated-sdk/.github/workflows/release.yml@refs/heads/main" \
      --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
      ${IMAGE_WITH_DIGEST} 2>/dev/null); then
        echo "✅ Image signature verification successful"
        echo "Signature details:"
        echo "$VERIFICATION_OUTPUT" | jq -r '
          "  • Certificate: \(.optional.bundle.Payload.certificate)",
          "  • Signature timestamp: \(.optional.sig.timestamp // "N/A")"
        '
    else
        echo "❌ Image signature verification failed"
        exit 1
    fi
fi

echo -e "\n📦 Step 3: Verifying SBOM attestation..."

# Try both predicate types
if ! RAW_ATTESTATION=$(cosign download attestation \
  --predicate-type spdxjson \
  ${IMAGE_WITH_DIGEST} 2>/dev/null) && \
   ! RAW_ATTESTATION=$(cosign download attestation \
  --predicate-type https://spdx.dev/Document \
  ${IMAGE_WITH_DIGEST} 2>/dev/null); then
    echo "❌ No SPDX attestation found on image"
    echo "This may indicate the SBOM wasn't properly attached during build"
    exit 1
fi

if [ -z "$RAW_ATTESTATION" ]; then
    echo "❌ Empty SPDX attestation found"
    echo "This may indicate an issue during the build process"
    exit 1
fi

echo "✅ SBOM verification successful"
DECODED_PAYLOAD=$(echo "$RAW_ATTESTATION" | jq -r '.payload' | base64 -d)

echo "SBOM details:"
echo "$DECODED_PAYLOAD" | jq -r '
  "  • Document Name: \(.predicate.name // "N/A")",
  "  • Created: \(.predicate.creationInfo.created // "N/A")",
  "  • Created By: \(.predicate.creationInfo.creators | map(select(startswith("Organization: Replicated"))) | .[0] // "N/A")",
  "  • Tool: \(.predicate.creationInfo.creators | map(select(startswith("Tool:"))) | .[0] // "N/A")",
  "  • Total Packages: \(.predicate.packages | length) packages",
  "\nKey Packages:",
  (.predicate.packages[] | select(.name | contains("replicated") or contains("curl") or contains("openssl")) | 
    "  • \(.name)@\(.versionInfo)\n    License: \(.licenseDeclared // "N/A")\n    Supplier: \(.supplier // "N/A")"
  )'

if [ "$SHOW_SBOM" = "true" ]; then
    echo -e "\nFull SBOM Content:"
    echo "$DECODED_PAYLOAD" | jq '.'
fi

echo "ℹ️  Use '--show-sbom' flag to view full SBOM contents"

echo -e "\n✨ All verifications completed successfully!"
echo "==============================================="
