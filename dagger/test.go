package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"strings"
)

func testUnit(
	ctx context.Context,
	source *dagger.Directory,
) error {
	ctr := buildEnvSDK(ctx, source)

	ctr = ctr.WithExec([]string{"make", "test-unit"})

	out, err := ctr.Stdout(ctx)
	if err != nil {
		return err
	}

	fmt.Println(out)

	return nil
}

func testPact(
	ctx context.Context,
	source *dagger.Directory,
	opServiceAccount *dagger.Secret,
) error {
	ctr := buildEnvSDK(ctx, source)

	// Add Ruby and pact tools
	ctr = ctr.
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "ruby", "ruby-dev", "build-essential"}).
		WithExec([]string{"gem", "install", "pact-mock_service", "-v", ">=3.5.0"}).
		WithExec([]string{"gem", "install", "pact-provider-verifier", "-v", ">=1.36.1"}).
		WithExec([]string{"gem", "install", "pact_broker-client", "-v", ">=1.22.3"})

	// Copy pact executables from pact-cli container
	pactContainer := dag.Container().From("pactfoundation/pact-cli:latest")
	pactFiles := []string{
		"/usr/bin/pact",
		"/usr/bin/pact-mock-service",
		"/usr/bin/pact-stub-service",
		"/usr/bin/pact-broker",
		"/usr/bin/pact-provider-verifier",
		"/usr/bin/pactflow",
		"/usr/bin/pact-message",
		"/usr/bin/pact-provider-verifier.cmd",
	}

	for _, file := range pactFiles {
		ctr = ctr.WithFile(file, pactContainer.File(file))
	}

	// Create symbolic link for pact-broker
	ctr = ctr.WithExec([]string{"ln", "-sf", "/usr/bin/pact-broker-client", "/usr/local/bin/pact-broker"})

	commitHash, err := dag.GitInfo(source).CommitHash(ctx)
	if err != nil {
		return err
	}

	pactToken := mustGetSecret(ctx, opServiceAccount, "Pactflow read-only token", "credential", VaultDeveloperAutomation)

	ctr = ctr.
		WithEnvVariable("PACT_VERSION", commitHash).
		WithEnvVariable("PACT_BROKER_BASE_URL", "https://replicated.pactflow.io").
		WithSecretVariable("PACT_BROKER_TOKEN", pactToken).
		WithExec([]string{"make", "test-pact"})

	out, err := ctr.Stdout(ctx)
	if err != nil {
		return err
	}

	fmt.Println(out)

	return nil
}

func buildEnvSDK(ctx context.Context, source *dagger.Directory) *dagger.Container {
	ctr := dag.Container().From("golang:1.24").
		WithDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "download"})

	return ctr
}

func testSBOMGeneration(
	ctx context.Context,
	dag *dagger.Client,
	source *dagger.Directory,
	version string,
) error {
	// Build the image first
	amdPackages, armPackages, melangeKey, err := buildAndPublishChainguardImage(ctx, dag, source, version)
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}

	// Use a test registry (ttl.sh) to avoid authentication
	testRegistry := fmt.Sprintf("ttl.sh/replicated-sdk-sbom-test-%s", version)

	// Publish the image with SBOM enabled
	digest, err := publishChainguardImage(
		ctx,
		dag,
		source,
		amdPackages,
		armPackages,
		melangeKey,
		version,
		testRegistry,
		"",
		nil,
		nil,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to publish image: %w", err)
	}

	// Use crane to inspect the published image
	ctr := dag.Container().
		From("gcr.io/go-containerregistry/crane:latest").
		WithExec([]string{"manifest", fmt.Sprintf("%s:%s", testRegistry, version)})

	manifest, err := ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get image manifest: %w", err)
	}

	// Check for SBOM attestation in the manifest
	if !strings.Contains(manifest, "application/spdx+json") && !strings.Contains(manifest, "application/vnd.cyclonedx+json") {
		return fmt.Errorf("SBOM attestation not found in image manifest")
	}

	fmt.Printf("Successfully verified SBOM generation and attachment for image %s:%s with digest %s\n", testRegistry, version, digest)
	return nil
}
