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

	// get the registry address from the image path
	registry := strings.Split(imagePath, "/")[0]

	// Create a fresh Apko instance and explicitly set auth
	apko := dag.Apko()

	var apkoWithAuth *dagger.Apko
	// Now set the actual auth
	if username != "" && password != nil {
		apkoWithAuth = apko.WithRegistryAuth(username, password, dagger.ApkoWithRegistryAuthOpts{
			Address: registry,
		})
	} else {
		apkoWithAuth = apko
	}

	image := apkoWithAuth.
		Publish(
			updatedSource.File("deploy/apko.yaml"),
			[]string{fmt.Sprintf("%s:%s", imagePath, version)},
			dagger.ApkoPublishOpts{
				Arch: []dagger.Platform{dagger.Platform("linux/amd64"), dagger.Platform("linux/arm64")},
				Source: updatedSource.WithDirectory("packages", amdPackages).
					WithDirectory("packages", armPackages).
					WithFile("melange.rsa.pub", melangeKey),
				Sbom: true,
			},
		)

	// return the image digest
	digest, err := image.Digest(ctx)
	if err != nil {
		return "", err
	}

	//
	// Verify SBOM was generated and attached
	//
	craneContainer := dag.Container().From("gcr.io/go-containerregistry/crane:latest")

	if username != "" && password != nil {
		craneContainer = craneContainer.
			WithEnvVariable("CRANE_USERNAME", username).
			WithSecretVariable("CRANE_PASSWORD", password)
	}

	// Get the manifest using environment variables for auth
	manifest, err := craneContainer.
		WithExec([]string{"crane", "manifest", fmt.Sprintf("%s:%s", imagePath, version)}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get image manifest: %w", err)
	}

	// Check for SBOM attestation in the manifest
	if !strings.Contains(manifest, "application/spdx+json") && !strings.Contains(manifest, "application/vnd.cyclonedx+json") {
		return "", fmt.Errorf("SBOM attestation not found in image manifest for %s:%s", imagePath, version)
	}

	fmt.Printf("Successfully verified SBOM generation and attachment for image %s:%s\n", imagePath, version)
	return digest, nil
}

func sanitizeVersionForMelange(version string) string {
	v := strings.ReplaceAll(version, "-beta.", "_beta")
	v = strings.ReplaceAll(v, "-alpha.", "_alpha")
	v = strings.ReplaceAll(v, "-", "_") // catch any remaining dashes
	return v
}
