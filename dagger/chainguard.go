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
) (*dagger.Container, *dagger.Container, error) {
	// build the melange.yaml with the correct version
	melangeYaml, err := source.File("deploy/melange.yaml").Contents(ctx)
	if err != nil {
		return nil, nil, err
	}
	melangeYaml = strings.Replace(melangeYaml, "version: 1.0.0", fmt.Sprintf("version: %s", version), 1)
	source = source.WithNewFile("deploy/melange.yaml", melangeYaml)

	// Initialize melange module for AMD64
	amdDirectory := dag.Melange().Build(
		source.File("deploy/melange.yaml"),
		dagger.MelangeBuildOpts{
			SourceDir: source,
			Arch:      "x86_64", // Use x86_64 instead of amd64 for Wolfi
		},
	)

	// Initialize melange module for ARM64
	armDirectory := dag.Melange().Build(
		source.File("deploy/melange.yaml"),
		dagger.MelangeBuildOpts{
			SourceDir: source,
			Arch:      "aarch64", // Use aarch64 instead of arm64 for Wolfi
		},
	)

	// build the apko.yaml with the correct version
	apkoYaml, err := source.File("deploy/apko.yaml").Contents(ctx)
	if err != nil {
		return nil, nil, err
	}
	apkoYaml = strings.Replace(apkoYaml, "VERSION: 1.0.0", fmt.Sprintf("VERSION: %s", version), 1)
	source = source.WithNewFile("deploy/apko.yaml", apkoYaml)

	// Build AMD64 image
	amdBuild := dag.Apko().Build(
		amdDirectory,
		source.File("deploy/apko.yaml"),
		"x86_64", // Use x86_64 instead of amd64 for Wolfi
		dagger.ApkoBuildOpts{
			Arch: "x86_64",
		},
	)

	// Build ARM64 image
	armBuild := dag.Apko().Build(
		armDirectory,
		source.File("deploy/apko.yaml"),
		"aarch64", // Use aarch64 instead of arm64 for Wolfi
		dagger.ApkoBuildOpts{
			Arch: "aarch64",
		},
	)

	fmt.Println("amdBuild", amdBuild)
	fmt.Println("armBuild", armBuild)

	return nil, nil, nil
}
