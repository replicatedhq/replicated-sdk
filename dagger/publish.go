package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
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

	version string,

	// +default=false
	staging bool,

	// +default=false
	production bool,

	// +default=true
	dev bool,

	// +default=false
	slsa bool,

	// +default="ttl.sh"
	chartRegistry string,

	// +optional
	githubToken *dagger.Secret,
) error {
	if err := generateReleaseNotesPR(ctx, source, opServiceAccount); err != nil {
		return err
	}

	// version must be passed in, it will be used to tag the image
	amdPackages, armPackages, melangeKey, err := buildAndPublishChainguardImage(ctx, dag, source, version)
	if err != nil {
		return err
	}

	digest := ""
	if dev {
		digest, err = publishChainguardImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "ttl.sh/replicated/replicated-sdk", "", nil)
		if err != nil {
			return err
		}
	}

	if staging {
		username := ""
		var password *dagger.Secret
		digest, err = publishChainguardImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "registry.staging.replicated.com/library/replicated-sdk", username, password)
		if err != nil {
			return err
		}
	}

	if production {
		username := ""
		var password *dagger.Secret
		digest, err = publishChainguardImage(ctx, dag, source, amdPackages, armPackages, melangeKey, version, "registry.replicated.com/library/replicated-sdk", username, password)
		if err != nil {
			return err
		}
	}

	err = buildAndPublishChart(ctx, dag, source, version, staging, production)
	if err != nil {
		return err
	}

	// if we are running in CI we trigger the SLSA provenance workflow
	if slsa {
		ctr := dag.Gh().
			Run(fmt.Sprintf(`api /repos/replicatedhq/replicated-sdk/actions/workflows/slsa.yml/dispatches \
				-f ref=main \
				-f inputs[digest]=%s`, digest),
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

	return nil
}
