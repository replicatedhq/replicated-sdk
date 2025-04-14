package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
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

	image, err := buildChainguardImage(ctx, source, "0.0.1")
	if err != nil {
		return err
	}

	digest, err := image.Digest(ctx)
	if err != nil {
		return err
	}

	fmt.Println("Multi-arch Image:", digest)

	return nil
}
