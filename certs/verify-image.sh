#!/bin/bash

#####################################################################
# verify-image.sh
#
# Description:
#   This script verifies the authenticity and integrity of the Replicated SDK
#   container images by performing three key security checks:
#   1. SLSA Provenance verification - Ensures build chain integrity
#   2. Image signature verification - Validates image authenticity
#   3. SBOM attestation verification - Checks software bill of materials
#
# Usage:
#   ./verify-image.sh [OPTIONS]
#
# Required Arguments:
#   -e, --env ENV      Environment to verify (dev|stage|prod)
#   -v, --version VER  Version to verify
#   -d, --digest DIG   Image digest to verify
#
# Optional Arguments:
#   --show-sbom       Show full SBOM content
#   -h, --help        Show this help message
#
# Environment-specific Behavior:
#   - dev: Uses ttl.sh registry with dev signing keys
#   - stage: Uses staging registry with staging signing keys
#   - prod: Uses production registry with keyless verification
#
# Examples:
#   ./verify-image.sh --env dev \
#     --version 1.5.3-beta.3 \
#     --digest sha256:5b064832df6bfb934c081fa0263134bc9845525211f09a752d5684306310f3c5
#
#   ./verify-image.sh --env prod \
#     --version 1.5.3 \
#     --digest sha256:7cb8e0c8e0fba8e4a7157b4fcef9e7345538f7543a4e5925bb8b30c9c1375400
#
# Exit Codes:
#   0 - All verifications passed
#   1 - Verification failed or invalid arguments
#
# Author: Replicated, Inc.
# License: Apache-2.0
#####################################################################

# Help function to display usage information and examples
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

# Parse command line arguments using a while loop for flexibility
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

# Validate required arguments to ensure script can proceed
if [ -z "$ENV" ] || [ -z "$TEST_VERSION" ] || [ -z "$DIGEST" ]; then
    echo "Error: Environment (-e), version (-v), and digest (-d) are required"
    show_help
    exit 1
fi

# Validate environment value to prevent invalid configurations
if [[ ! "$ENV" =~ ^(dev|stage|prod)$ ]]; then
    echo "Error: Environment (-e, --env) must be one of: dev, stage, prod"
    exit 1
fi

# Set environment-specific variables for registry paths and image names
case $ENV in
    dev)
        # Development environment uses ttl.sh for temporary image storage
        REGISTRY="ttl.sh"
        IMAGE="${REGISTRY}/replicated/replicated-sdk"
        ;;
    stage)
        # Staging environment uses staging registry for pre-release testing
        REGISTRY="registry.staging.replicated.com"
        IMAGE="${REGISTRY}/library/replicated-sdk-image"
        ;;
    prod)
        # Production environment uses main registry for released images
        REGISTRY="registry.replicated.com"
        IMAGE="${REGISTRY}/library/replicated-sdk-image"
        ;;
esac

# Construct full image reference with digest for unique identification
IMAGE_WITH_DIGEST="${IMAGE}@${DIGEST}"

# Define source repository for SLSA verification
SOURCE_REPO=github.com/replicatedhq/replicated-sdk

echo "==============================================="
echo "Starting verification for ${IMAGE_WITH_DIGEST}"
echo "==============================================="

# Step 1: SLSA Provenance Verification
# This step ensures the image was built through our secure build pipeline
echo -e "\n📋 Step 1: Verifying SLSA provenance..."
if [ "$ENV" != "dev" ]; then
    # SLSA verification is skipped for dev environment as it uses different build process
    if COSIGN_REPOSITORY=${REGISTRY}/library/replicated-sdk-image slsa-verifier verify-image "${IMAGE_WITH_DIGEST}" \
      --source-uri ${SOURCE_REPO} \
      --source-tag ${TEST_VERSION} \
      --print-provenance | tee /tmp/slsa_output.json | jq -r '
        # Format and display relevant build information from SLSA attestation
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

# Step 2: Image Signature Verification
# Validates the image signature using environment-specific keys
echo -e "\n🔏 Step 2: Verifying image signature..."
if [ "$ENV" = "dev" ]; then
    # Development environment uses a development signing key for verification
    if VERIFICATION_OUTPUT=$(cosign verify-attestation \
      --key ./cosign-dev.pub \
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
    # Staging environment uses staging-specific signing key
    if VERIFICATION_OUTPUT=$(cosign verify-attestation \
      --key ./cosign-stage.pub \
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
    # Production environment uses production-specific signing key
    if VERIFICATION_OUTPUT=$(cosign verify-attestation \
      --key ./cosign-prod.pub \
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
fi

# Step 3: SBOM Attestation Verification
# Verifies and displays the Software Bill of Materials attached to the image
echo -e "\n📦 Step 3: Verifying SBOM attestation..."

# Try both SPDX predicate types for compatibility with different SBOM formats
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

# Ensure the attestation is not empty
if [ -z "$RAW_ATTESTATION" ]; then
    echo "❌ Empty SPDX attestation found"
    echo "This may indicate an issue during the build process"
    exit 1
fi

echo "✅ SBOM verification successful"
DECODED_PAYLOAD=$(echo "$RAW_ATTESTATION" | jq -r '.payload' | base64 -d)

# Display formatted SBOM information focusing on key details
echo "SBOM details:"
echo "$DECODED_PAYLOAD" | jq -r '
  "  • Document Name: \(.predicate.name // "N/A")",
  "  • Created: \(.predicate.creationInfo.created // "N/A")",
  "  • Created By: \(.predicate.creationInfo.creators | map(select(startswith("Organization: Replicated"))) | .[0] // "N/A")",
  "  • Tool: \(.predicate.creationInfo.creators | map(select(startswith("Tool:"))) | .[0] // "N/A")",
  "  • Total Packages: \(.predicate.packages | length) packages"
'

# Optionally display full SBOM content if requested
if [ "$SHOW_SBOM" = "true" ]; then
    echo -e "\nFull SBOM Content:"
    echo "$DECODED_PAYLOAD" | jq '.'
fi

echo "ℹ️  Use '--show-sbom' flag to view full SBOM contents"

echo -e "\n✨ All verifications completed successfully!"
echo "==============================================="
