package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
)

// Validate runs the complete validation pipeline or specific components
// This is the primary entry point for all CI/CD operations
func (m *ReplicatedSdk) Validate(
	ctx context.Context,

	// +defaultPath="/"
	source *dagger.Directory,

	opServiceAccount *dagger.Secret,
) error {
	imageRegistry, imageRepository, imageTag, err := buildAndPushImageToTTL(ctx, source)
	if err != nil {
		return err
	}

	chart, err := buildAndPushChartToTTL(ctx, source, imageRegistry, imageRepository, imageTag)
	if err != nil {
		return err
	}

	wrappedChart, err := createWrappedTestChart(ctx, source, chart)
	if err != nil {
		return err
	}

	fmt.Printf("Wrapped chart created: %s\n", wrappedChart)

	return nil
}
