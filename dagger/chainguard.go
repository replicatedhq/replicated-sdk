package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"encoding/json"
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
	// version to tag the image with
	version string,
	// full image path including registry (e.g. "ttl.sh/replicated/replicated-sdk")
	imagePath string,
	// registry username (empty for ttl.sh)
	username string,
	// registry password (nil for ttl.sh)
	password *dagger.Secret,
	// cosign private key for signing SBOM attestations
	cosignKey *dagger.Secret,
	// password to decrypt the cosign private key
	cosignPassword *dagger.Secret,
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
		return "", fmt.Errorf("failed to get manifest: %w", err)
	}

	// Check for SBOM attestation in the manifest
	if !strings.Contains(manifest, "application/spdx+json") && !strings.Contains(manifest, "application/vnd.cyclonedx+json") {
		fmt.Printf("SBOM attestation not found in manifest, will attempt to create it...\n")

		// Get the SBOMs that were generated during publish
		sbomDir := image.Sbom()
		sbomFiles, err := sbomDir.Entries(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list SBOM files: %w", err)
		}
		if len(sbomFiles) == 0 {
			return "", fmt.Errorf("no SBOM files were generated during image publish")
		}

		// Parse the manifest to get digests for each architecture
		var manifestObj struct {
			Manifests []struct {
				Digest   string `json:"digest"`
				Platform struct {
					Architecture string `json:"architecture"`
				} `json:"platform"`
			} `json:"manifests"`
		}

		if err := json.Unmarshal([]byte(manifest), &manifestObj); err != nil {
			return "", fmt.Errorf("failed to parse manifest: %w", err)
		}

		// Map of architecture to its digest
		archDigests := make(map[string]string)
		for _, m := range manifestObj.Manifests {
			archDigests[m.Platform.Architecture] = m.Digest
			fmt.Printf("Found digest %s for architecture %s\n", m.Digest, m.Platform.Architecture)
		}

		// For each architecture
		for _, arch := range []string{"amd64", "arm64"} {
			digest, ok := archDigests[arch]
			if !ok {
				return "", fmt.Errorf("digest not found for architecture %s", arch)
			}

			// Find matching SBOM file for this architecture
			var sbomFile string
			archMapping := map[string]string{
				"amd64": "x86_64",
				"arm64": "aarch64",
			}
			mappedArch := archMapping[arch]
			for _, f := range sbomFiles {
				if strings.Contains(f, mappedArch) {
					sbomFile = f
					break
				}
			}
			if sbomFile == "" {
				return "", fmt.Errorf("no SBOM file found for architecture %s (mapped to %s) in files: %v", arch, mappedArch, sbomFiles)
			}

			// Create a temporary directory with the SBOM file
			tmpDir := dag.Directory()
			tmpDir = tmpDir.WithFile(sbomFile, sbomDir.File(sbomFile))

			// Use cosign to create SBOM attestation
			cosignContainer := dag.Container().From("gcr.io/projectsigstore/cosign:v2.2.3")
			if username != "" && password != nil {
				cosignContainer = cosignContainer.
					WithEnvVariable("COSIGN_USERNAME", username).
					WithSecretVariable("COSIGN_PASSWORD", password)
			}

			// Set up cosign key if provided
			if cosignKey != nil {
				cosignContainer = cosignContainer.
					WithSecretVariable("COSIGN_PASSWORD", cosignPassword).
					WithSecretVariable("COSIGN_KEY", cosignKey)
			}

			// Set COSIGN_YES to skip confirmation prompts
			cosignContainer = cosignContainer.WithEnvVariable("COSIGN_YES", "true")

			// Build cosign command based on whether we have a key
			var attestArgs []string
			if cosignKey != nil {
				attestArgs = []string{
					"cosign", "attest", "--yes",
					"--key", "env://COSIGN_KEY",
					"--type", "spdxjson",
					"--predicate", sbomFile,
					fmt.Sprintf("%s@%s", imagePath, digest),
				}
			} else {
				return "", fmt.Errorf("cosign key is required for SBOM attestation")
			}

			attestContainer := cosignContainer.
				WithMountedDirectory("/sbom", tmpDir).
				WithWorkdir("/sbom").
				WithExec(attestArgs)

			if _, err := attestContainer.Sync(ctx); err != nil {
				return "", fmt.Errorf("failed to create SBOM attestation for %s: %w", arch, err)
			}

			fmt.Printf("Successfully created SBOM attestation for %s\n", arch)
		}

		fmt.Printf("Successfully created all SBOM attestations\n")
	} else {
		fmt.Printf("SBOM attestation already exists in manifest for %s:%s\n", imagePath, version)
	}

	return digest, nil
}

func sanitizeVersionForMelange(version string) string {
	v := strings.ReplaceAll(version, "-beta.", "_beta")
	v = strings.ReplaceAll(v, "-alpha.", "_alpha")
	v = strings.ReplaceAll(v, "-", "_") // catch any remaining dashes
	return v
}
