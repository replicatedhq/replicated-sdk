package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"strings"
)

func buildChainguardImage(
	ctx context.Context,
	source *dagger.Directory,
	version string,
) (*dagger.ApkoImage, error) {
	// build the melange.yaml with the correct version
	melangeYaml, err := source.File("deploy/melange.yaml").Contents(ctx)
	if err != nil {
		return nil, err
	}
	melangeYaml = strings.Replace(melangeYaml, "version: 1.0.0", fmt.Sprintf("version: %s", version), 1)
	source = source.WithNewFile("deploy/melange.yaml", melangeYaml)

	// Initialize melange module for AMD64
	amdPackages := dag.Melange().Build(
		source.File("deploy/melange.yaml"),
		dagger.MelangeBuildOpts{
			SourceDir: source,
			Arch:      "x86_64",
		},
	)

	// Get the signing key from melange build
	melangeKey := amdPackages.File("melange.rsa.pub")

	// Initialize melange module for ARM64
	armPackages := dag.Melange().Build(
		source.File("deploy/melange.yaml"),
		dagger.MelangeBuildOpts{
			SourceDir: source,
			Arch:      "aarch64",
		},
	)

	// build the apko.yaml with the correct version
	apkoYaml, err := source.File("deploy/apko.yaml").Contents(ctx)
	if err != nil {
		return nil, err
	}
	apkoYaml = strings.Replace(apkoYaml, "VERSION: 1.0.0", fmt.Sprintf("VERSION: %s", version), 1)
	source = source.WithNewFile("deploy/apko.yaml", apkoYaml)

	// Create source with packages and signing key for both architectures
	multiArchSource := source.
		WithDirectory("packages/x86_64", amdPackages).
		WithDirectory("packages/aarch64", armPackages).
		WithFile("melange.rsa.pub", melangeKey)

	// Build and publish multi-arch image
	image := dag.Apko().Publish(
		multiArchSource,
		source.File("deploy/apko.yaml"),
		[]string{fmt.Sprintf("ttl.sh/replicated-sdk-%s:1h", version)},
		dagger.ApkoPublishOpts{}, // No arch specified for multi-arch build
	)

	return image, nil
}
