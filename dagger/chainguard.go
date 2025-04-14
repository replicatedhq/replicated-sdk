package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"strings"
)

func buildAndPublishChainguardImage(ctx context.Context, dag *dagger.Client, source *dagger.Directory, version string) (string, error) {
	// Update melange.yaml with correct version
	melangeYaml, err := source.File("deploy/melange.yaml").Contents(ctx)
	if err != nil {
		return "", err
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
		return "", err
	}
	apkoYaml = strings.Replace(apkoYaml, "VERSION: 1.0.0", fmt.Sprintf("VERSION: %s", version), 1)
	source = source.WithNewFile("deploy/apko.yaml", apkoYaml)

	// publish to docker hub (legacy, will be removed in the future)
	image := dag.Apko().Publish(
		source.WithDirectory("packages", amdPackages).
			WithDirectory("packages", armPackages).
			WithFile("melange.rsa.pub", melangeKey),
		source.File("deploy/apko.yaml"),
		[]string{fmt.Sprintf("index.docker.io/replicated/replicated-sdk:%s", version)},
		dagger.ApkoPublishOpts{
			Arch: "x86_64,aarch64",
		},
	).Container()

	// publish to staging
	dag.Apko().Publish(
		source.WithDirectory("packages", amdPackages).
			WithDirectory("packages", armPackages).
			WithFile("melange.rsa.pub", melangeKey),
		source.File("deploy/apko.yaml"),
		[]string{fmt.Sprintf("registry.staging.replicated.com/library/replicated-sdk:%s", version)},
	)

	// publish to production
	dag.Apko().Publish(
		source.WithDirectory("packages", amdPackages).
			WithDirectory("packages", armPackages).
			WithFile("melange.rsa.pub", melangeKey),
		source.File("deploy/apko.yaml"),
		[]string{fmt.Sprintf("registry.replicated.com/library/replicated-sdk:%s", version)},
	)

	// return the image digest
	digest, err := image.ID(ctx)
	if err != nil {
		return "", err
	}

	return string(digest), nil
}
