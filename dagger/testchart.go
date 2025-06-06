package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
)

// TestChart builds the SDK image and chart and wraps them in a test chart
func (m *ReplicatedSdk) TestChart(
	ctx context.Context,

	// +defaultPath="/"
	source *dagger.Directory,
) error {
	imageRegistry, imageRepository, imageTag, err := buildAndPushImageToTTL(ctx, source)
	if err != nil {
		return err
	}
	fmt.Printf("SDK image pushed to %s/%s:%s\n", imageRegistry, imageRepository, imageTag)

	chart, err := buildAndPushChartToTTL(ctx, source, imageRegistry, imageRepository, imageTag)
	if err != nil {
		return err
	}
	fmt.Printf("SDK chart pushed to %s\n", chart)

	wrappedChart, err := createWrappedTestChart(ctx, source, chart)
	if err != nil {
		return err
	}
	fmt.Printf("Wrapped chart created: %s\n", wrappedChart)

	return nil
}
