package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"strings"
)

func buildAndPublishChainguardImage(
	ctx context.Context,
	dag *dagger.Client,
	source *dagger.Directory,
	version string,
) (*dagger.Directory, *dagger.Directory, *dagger.File, error) {
	// Update melange.yaml with correct version
	melangeYaml, err := source.File("deploy/melange.yaml").Contents(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	melangeYaml = strings.Replace(melangeYaml, "version: 1.0.0", fmt.Sprintf("version: %s", sanitizeVersionForMelange(version)), 1)
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

	// Update apko.yaml with just the VERSION environment variable
	apkoYaml, err := source.File("deploy/apko.yaml").Contents(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	apkoYaml = strings.Replace(apkoYaml, "VERSION: 1.0.0", fmt.Sprintf("VERSION: %s", version), 1)
	source = source.WithNewFile("deploy/apko.yaml", apkoYaml)

	return amdPackages, armPackages, melangeKey, nil
}

func publishChainguardImage(
	ctx context.Context,
	dag *dagger.Client,
	source *dagger.Directory,
	amdPackages *dagger.Directory,
	armPackages *dagger.Directory,
	melangeKey *dagger.File,
	version string,
	imagePath string,
	username string,
	password *dagger.Secret,
) (string, error) {
	// Update apko.yaml to set the package version constraint
	apkoYaml, err := source.File("deploy/apko.yaml").Contents(ctx)
	if err != nil {
		return "", err
	}

	// Update the package list to include version constraint
	apkoYaml = strings.Replace(
		apkoYaml,
		"    - replicated\n",
		fmt.Sprintf("    - replicated=%s-r0\n", sanitizeVersionForMelange(version)),
		1,
	)

	// Create a new source directory with the updated apko.yaml
	updatedSource := source.WithNewFile("deploy/apko.yaml", apkoYaml)

	build := dag.Apko().
		Build(
			updatedSource.WithDirectory("packages", amdPackages).
				WithDirectory("packages", armPackages).
				WithFile("melange.rsa.pub", melangeKey),
			updatedSource.File("deploy/apko.yaml"),
			fmt.Sprintf("%s:%s", imagePath, version),
			dagger.ApkoBuildOpts{
				Arch: "x86_64,aarch64",
			},
		)

	image := build.Publish(dagger.ApkoBuildPublishOpts{
		RegistryUsername: username,
		RegistryPassword: password,
	})

	// return the image digest
	digest, err := image.ID(ctx)
	if err != nil {
		return "", err
	}

	return string(digest), nil
}

func sanitizeVersionForMelange(version string) string {
	v := strings.ReplaceAll(version, "-beta.", "_beta")
	v = strings.ReplaceAll(v, "-alpha.", "_alpha")
	v = strings.ReplaceAll(v, "-", "_") // catch any remaining dashes
	return v
}
