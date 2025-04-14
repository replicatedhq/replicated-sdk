package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
)

// Publish publishes the Replicated SDK images and chart
// to staging and production registries
func (m *ReplicatedSdk) Publish(
	ctx context.Context,

	// +defaultPath="/"
	source *dagger.Directory,

	opServiceAccount *dagger.Secret,
) error {
	if err := generateReleaseNotesPR(ctx, source, opServiceAccount); err != nil {
		return err
	}

	_, _, err := buildChainguardImage(ctx, source, "0.0.1-dev")
	if err != nil {
		return err
	}

	return nil
}
