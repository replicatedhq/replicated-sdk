package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"os"
)

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

	// +default=false
	slsa bool,

	// +optional
	githubToken *dagger.Secret,

	// +optional
	cosignKey *dagger.Secret,

	// +optional
	cosignPassword *dagger.Secret,
) error {
	// Check for SecureBuild feature flag - route staging/production through SecureBuild when enabled
	// Dev always uses Wolfi pipeline for contributor-friendly builds
	if useSecureBuild := os.Getenv("USE_SECUREBUILD"); useSecureBuild == "true" && !dev {
		var environment string
		if staging {
			environment = SecureBuildEnvStaging
		} else if production {
			environment = SecureBuildEnvProduction
		}
		
		_, _, _, err := buildAndPushImageWithSecureBuild(ctx, source, environment, version, opServiceAccount)
		return err
	}

	// Default to current Chainguard pipeline
	amdPackages, armPackages, melangeKey, err := buildAndPublishChainguardImage(ctx, dag, source, version)
	if err != nil {
		return err
	}

	digest := ""
	if dev {
		// In dev mode, get cosign key from dev vault if not provided
		if cosignKey == nil {
			cosignKey = mustGetSecret(ctx, opServiceAccount, "Replicated-SDK-Dev-Cosign.key", "cosign.key", VaultDeveloperAutomation)
			cosignPassword = mustGetSecret(ctx, opServiceAccount, "Replicated-SDK-Dev-Cosign.info", "password", VaultDeveloperAutomation)
		}
		// in dev mode we don't have username/password for the registry
		digest, err = publishChainguardImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "ttl.sh/replicated/replicated-sdk", "", nil, cosignKey, cosignPassword)
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

		_, err = publishChainguardImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "index.docker.io/replicated/replicated-sdk", username, password, cosignKey, cosignPassword)
		if err != nil {
			return err
		}

		digest, err = publishChainguardImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "registry.staging.replicated.com/library/replicated-sdk-image", libraryUsername, libraryPassword, cosignKey, cosignPassword)
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

		_, err = publishChainguardImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "index.docker.io/replicated/replicated-sdk", username, password, cosignKey, cosignPassword)
		if err != nil {
			return err
		}

		digest, err = publishChainguardImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "registry.replicated.com/library/replicated-sdk-image", libraryUsername, libraryPassword, cosignKey, cosignPassword)
		if err != nil {
			return err
		}
	}

	err = buildAndPublishChart(ctx, dag, source, version, staging, production, opServiceAccountProduction)
	if err != nil {
		return err
	}

	// if we are running in CI we trigger the SLSA provenance workflow
	if slsa {
		ctr := dag.Gh().
			Run(fmt.Sprintf(`api --method POST /repos/replicatedhq/replicated-sdk/actions/workflows/slsa.yml/dispatches \
				-f ref=%s \
				-f inputs[digest]=%s \
				-f inputs[production]=%t`, version, digest, production),
				dagger.GhRunOpts{
					Token: githubToken,
				},
			)
		stdOut, err := ctr.Stdout(ctx)
		if err != nil {
			return err
		}
		fmt.Println(stdOut)
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
