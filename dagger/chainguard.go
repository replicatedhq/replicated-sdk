package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"encoding/base64"
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

	// Get the SBOMs that were generated during publish
	sbomDir := image.Sbom()
	sbomFiles, err := sbomDir.Entries(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list SBOM files: %w", err)
	}
	if len(sbomFiles) == 0 {
		return "", fmt.Errorf("no SBOM files were generated during image publish")
	}

	// For each SBOM file, modify it to add Replicated, Inc. to the creators field
	for _, sbomFile := range sbomFiles {
		// Read the SBOM content
		sbomContent, err := sbomDir.File(sbomFile).Contents(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to read SBOM file %s: %w", sbomFile, err)
		}

		// Parse the SBOM JSON
		var sbom map[string]interface{}
		if err := json.Unmarshal([]byte(sbomContent), &sbom); err != nil {
			return "", fmt.Errorf("failed to parse SBOM JSON: %w", err)
		}

		// Add Replicated, Inc. to the creators field if it exists
		if creationInfo, ok := sbom["creationInfo"].(map[string]interface{}); ok {
			if creators, ok := creationInfo["creators"].([]interface{}); ok {
				// Add Replicated, Inc. if not already present
				replicatedCreator := "Organization: Replicated, Inc."
				hasReplicated := false
				for _, creator := range creators {
					if creator.(string) == replicatedCreator {
						hasReplicated = true
						break
					}
				}
				if !hasReplicated {
					creationInfo["creators"] = append(creators, replicatedCreator)
				}
			} else {
				// If no creators field exists, create it
				creationInfo["creators"] = []interface{}{
					"Organization: Replicated, Inc.",
				}
			}
		}

		// Convert back to JSON
		modifiedSbom, err := json.MarshalIndent(sbom, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal modified SBOM: %w", err)
		}

		// Update the SBOM file in the directory
		sbomDir = sbomDir.WithNewFile(sbomFile, string(modifiedSbom))
	}

	// Get the manifest to find the correct digests
	craneContainer := dag.Container().From("gcr.io/go-containerregistry/crane:latest")

	if username != "" && password != nil {
		craneContainer = craneContainer.
			WithEnvVariable("CRANE_USERNAME", username).
			WithSecretVariable("CRANE_PASSWORD", password)
	}

	manifest, err := craneContainer.
		WithExec([]string{"crane", "manifest", fmt.Sprintf("%s:%s", imagePath, version)}).
		Stdout(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get manifest: %w", err)
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

	// Get the first digest as our main reference
	if len(manifestObj.Manifests) == 0 {
		return "", fmt.Errorf("no manifests found in image")
	}
	mainDigest := manifestObj.Manifests[0].Digest

	// Check for SBOM attestation in the manifest
	if !strings.Contains(manifest, "application/spdx+json") && !strings.Contains(manifest, "application/vnd.cyclonedx+json") {
		fmt.Printf("SBOM attestation not found in manifest, will attempt to create it...\n")

		// Get the base64 encoded key as a string and decode it
		encodedKeyPlaintext, err := cosignKey.Plaintext(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get cosign key: %w", err)
		}

		// Decode the cosignKey from base64
		decodedBytes, err := base64.StdEncoding.DecodeString(encodedKeyPlaintext)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 key: %w", err)
		}

		// The Attest function expects a dagger.Secret, so we need to change the type
		// Convert decoded bytes back to a dagger.Secret
		decodedCosignKey := dag.SetSecret("decodedCosignKey", string(decodedBytes))

		// Use our modified SBOM directory that includes Replicated, Inc.
		sbomFiles, err := sbomDir.Entries(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list SBOM files: %w", err)
		}
		if len(sbomFiles) == 0 {
			return "", fmt.Errorf("no SBOM files were generated during image publish")
		}

		// Map of architecture to its digest
		archDigests := make(map[string]string)
		for _, m := range manifestObj.Manifests {
			archDigests[m.Platform.Architecture] = m.Digest
		}

		// Use cosign module to create SBOM attestation
		cosignModule := dag.Cosign()

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
			tmpDir := dag.Directory().WithFile(sbomFile, sbomDir.File(sbomFile))

			// Attest the SBOM using the decoded key
			_, err := cosignModule.Attest(
				ctx,
				decodedCosignKey,
				cosignPassword,
				fmt.Sprintf("%s@%s", imagePath, digest),
				tmpDir.File(sbomFile),
				dagger.CosignAttestOpts{
					SbomType:         "spdxjson",
					RegistryUsername: username,
					RegistryPassword: password,
				},
			)
			if err != nil {
				return "", fmt.Errorf("failed to create SBOM attestation for %s: %w", arch, err)
			}

			fmt.Printf("Successfully created SBOM attestation for %s\n", arch)
		}

		fmt.Printf("Successfully created all SBOM attestations\n")
	} else {
		fmt.Printf("SBOM attestation already exists in manifest for %s:%s\n", imagePath, version)
	}

	// Print verification instructions
	fmt.Printf("\n✨ Image successfully built, published, and signed!\n")
	fmt.Printf("To verify the image, run:\n\n")

	// Determine environment from image path
	env := "dev"
	if strings.Contains(imagePath, "registry.staging.replicated.com") {
		env = "stage"
	} else if strings.Contains(imagePath, "registry.replicated.com") {
		env = "prod"
	}

	fmt.Printf("./verify-image.sh --env %s --version %s --digest %s\n\n", env, version, mainDigest)

	return mainDigest, nil
}

func sanitizeVersionForMelange(version string) string {
	v := strings.ReplaceAll(version, "-beta.", "_beta")
	v = strings.ReplaceAll(v, "-alpha.", "_alpha")
	v = strings.ReplaceAll(v, "-", "_") // catch any remaining dashes
	return v
}
