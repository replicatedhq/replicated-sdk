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

	armImage, amdImage, err := buildChainguardImage(ctx, source, "0.0.1")
	if err != nil {
		return err
	}

	armDigest, err := armImage.Digest(ctx)
	if err != nil {
		return err
	}

	amdDigest, err := amdImage.Digest(ctx)
	if err != nil {
		return err
	}

	fmt.Println("ARM64 Image:", armDigest)
	fmt.Println("AMD64 Image:", amdDigest)

	return nil
}
