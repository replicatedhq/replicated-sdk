package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
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

	pactToken := mustGetSecret(ctx, opServiceAccount, "Pactflow read-only token", "credential")

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
