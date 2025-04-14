package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"strings"
)

func buildChainguardImage(ctx context.Context, dag *dagger.Client, source *dagger.Directory, version string) (*dagger.Container, *dagger.Container, error) {
	// Update melange.yaml with correct version
	melangeYaml, err := source.File("deploy/melange.yaml").Contents(ctx)
	if err != nil {
		return nil, nil, err
	}
	melangeYaml = strings.Replace(melangeYaml, "version: 1.0.0", fmt.Sprintf("version: %s", version), 1)
	source = source.WithNewFile("deploy/melange.yaml", melangeYaml)

	// Build AMD64 package with melange
	amdPackages := dag.Melange().Build(source.File("deploy/melange.yaml"), dagger.MelangeBuildOpts{
		SourceDir: source,
		Arch:      "x86_64",
	})

	// Get the signing key from melange build
	melangeKey := amdPackages.File("melange.rsa.pub")

	// Build ARM64 package with melange
	armPackages := dag.Melange().Build(source.File("deploy/melange.yaml"), dagger.MelangeBuildOpts{
		SourceDir: source,
		Arch:      "aarch64",
	})

	// Update apko.yaml with correct version
	apkoYaml, err := source.File("deploy/apko.yaml").Contents(ctx)
	if err != nil {
		return nil, nil, err
	}
	apkoYaml = strings.Replace(apkoYaml, "VERSION: 1.0.0", fmt.Sprintf("VERSION: %s", version), 1)
	source = source.WithNewFile("deploy/apko.yaml", apkoYaml)

	// Publish multi-arch image with apko
	image := dag.Apko().Publish(
		source.WithDirectory("packages", amdPackages).
			WithDirectory("packages", armPackages).
			WithFile("melange.rsa.pub", melangeKey),
		source.File("deploy/apko.yaml"),
		[]string{fmt.Sprintf("ttl.sh/replicated-sdk:%s-1h", version)},
		dagger.ApkoPublishOpts{
			Arch: "x86_64,aarch64",
		},
	).Container()

	return image, image, nil
}
