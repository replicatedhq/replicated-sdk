package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"regexp"
)

var stableReleaseVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// Publish publishes the Replicated SDK images and chart
// to staging and production registries
//
//	this is set up to default publish to ttl.sh and that's it
func (m *ReplicatedSdk) Publish(
	ctx context.Context,

	// +defaultPath="/"
	source *dagger.Directory,

	opServiceAccount *dagger.Secret,

	// +optional
	opServiceAccountProduction *dagger.Secret,

	version string,

	// +default=false
	staging bool,

	// +default=false
	production bool,

	// +default=true
	dev bool,

	// Skip container image publishing when SecureBuild owns the release images.
	// +default=false
	skipImage bool,

	// +optional
	githubToken *dagger.Secret,

	// +optional
	cosignKey *dagger.Secret,

	// +optional
	cosignPassword *dagger.Secret,
) error {
	if (staging || production) && stableReleaseVersion.MatchString(version) && !skipImage {
		return fmt.Errorf("container images for stable release %s must be published by SecureBuild", version)
	}

	if skipImage {
		if err := buildAndPublishChart(ctx, dag, source, version, staging, production, opServiceAccountProduction); err != nil {
			return err
		}

		if production {
			if err := dag.Gh().
				WithToken(githubToken).
				WithRepo("replicatedhq/replicated-sdk").
				WithSource(source).
				Release().
				Create(ctx, version, version); err != nil {
				return err
			}
		}

		return nil
	}

	// version must be passed in, it will be used to tag the image
	amdPackages, armPackages, melangeKey, err := buildImage(ctx, dag, source, version, []string{"x86_64", "aarch64"})
	if err != nil {
		return err
	}

	if dev {
		// In dev mode, get cosign key from dev vault if not provided
		if cosignKey == nil {
			cosignKey = mustGetSecret(ctx, opServiceAccount, "Replicated-SDK-Dev-Cosign.key", "cosign.key", VaultDeveloperAutomation)
			cosignPassword = mustGetSecret(ctx, opServiceAccount, "Replicated-SDK-Dev-Cosign.info", "password", VaultDeveloperAutomation)
		}
		// in dev mode we don't have username/password for the registry
		_, err = publishImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "", "ttl.sh/replicated/replicated-sdk", "", nil, cosignKey, cosignPassword)
		if err != nil {
			return err
		}
	}

	if staging {
		// In staging, get cosign key from production vault if not provided
		if cosignKey == nil {
			cosignKey = mustGetSecret(ctx, opServiceAccountProduction, "Replicated-SDK-Staging-Cosign.key", "cosign.key", VaultDeveloperAutomationProduction)
			cosignPassword = mustGetSecret(ctx, opServiceAccountProduction, "Replicated-SDK-Staging-Cosign.key", "password", VaultDeveloperAutomationProduction)
		}

		username := mustGetNonSensitiveSecret(ctx, opServiceAccountProduction, "Docker Hub Release Account", "username", VaultDeveloperAutomationProduction)
		password := mustGetSecret(ctx, opServiceAccountProduction, "Docker Hub Release Account", "password", VaultDeveloperAutomationProduction)

		libraryUsername := mustGetNonSensitiveSecret(ctx, opServiceAccountProduction, "Replicated SDK Publish", "library_username", VaultDeveloperAutomationProduction)
		libraryPassword := mustGetSecret(ctx, opServiceAccountProduction, "Replicated SDK Publish", "staging_library_password", VaultDeveloperAutomationProduction)

		_, err = publishImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "", "index.docker.io/replicated/replicated-sdk", username, password, cosignKey, cosignPassword)
		if err != nil {
			return err
		}

		_, err = publishImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "", "registry.staging.replicated.com/library/replicated-sdk-image", libraryUsername, libraryPassword, cosignKey, cosignPassword)
		if err != nil {
			return err
		}
	}

	if production {
		// In production, get cosign key from production vault if not provided
		if cosignKey == nil {
			cosignKey = mustGetSecret(ctx, opServiceAccountProduction, "Replicated-SDK-Production-Cosign.key", "cosign.key", VaultDeveloperAutomationProduction)
			cosignPassword = mustGetSecret(ctx, opServiceAccountProduction, "Replicated-SDK-Production-Cosign.key", "password", VaultDeveloperAutomationProduction)
		}

		username := mustGetNonSensitiveSecret(ctx, opServiceAccountProduction, "Docker Hub Release Account", "username", VaultDeveloperAutomationProduction)
		password := mustGetSecret(ctx, opServiceAccountProduction, "Docker Hub Release Account", "password", VaultDeveloperAutomationProduction)

		libraryUsername := mustGetNonSensitiveSecret(ctx, opServiceAccountProduction, "Replicated SDK Publish", "library_username", VaultDeveloperAutomationProduction)
		libraryPassword := mustGetSecret(ctx, opServiceAccountProduction, "Replicated SDK Publish", "library_password", VaultDeveloperAutomationProduction)

		_, err = publishImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "", "index.docker.io/replicated/replicated-sdk", username, password, cosignKey, cosignPassword)
		if err != nil {
			return err
		}

		_, err = publishImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "", "registry.replicated.com/library/replicated-sdk-image", libraryUsername, libraryPassword, cosignKey, cosignPassword)
		if err != nil {
			return err
		}
	}

	err = buildAndPublishChart(ctx, dag, source, version, staging, production, opServiceAccountProduction)
	if err != nil {
		return err
	}

	if production {
		// create a release on github
		if err := dag.Gh().
			WithToken(githubToken).
			WithRepo("replicatedhq/replicated-sdk").
			WithSource(source).
			Release().
			Create(ctx, version, version); err != nil {
			return err
		}
	}

	return nil
}
