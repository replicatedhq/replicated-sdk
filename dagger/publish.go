package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"os"
)

// Publish publishes the Replicated SDK images and chart
// to staging and production registries
func (m *ReplicatedSdk) Publish(
	ctx context.Context,

	// +defaultPath="/"
	source *dagger.Directory,

	opServiceAccount *dagger.Secret,

	version string,

	// +default=true
	staging bool,

	// +default=false
	production bool,
) error {
	if err := generateReleaseNotesPR(ctx, source, opServiceAccount); err != nil {
		return err
	}

	// version must be passed in, it will be used to tag the image
	digest, err := buildAndPublishChainguardImage(ctx, dag, source, version)
	if err != nil {
		return err
	}

	err = buildAndPublishChart(ctx, dag, source, version, staging, production)
	if err != nil {
		return err
	}

	// if we are running in CI we trigger the SLSA provenance workflow
	if os.Getenv("CI") == "true" {
		ghToken := dag.SetSecret("GITHUB_TOKEN", os.Getenv("GITHUB_TOKEN"))
		ctr := dag.Gh().
			Run(fmt.Sprintf(`api /repos/replicatedhq/replicated-sdk/actions/workflows/slsa.yml/dispatches \
				-f ref=main \
				-f inputs[digest]=%s`, digest),
				dagger.GhRunOpts{
					Token: ghToken,
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
