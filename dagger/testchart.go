package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"time"
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

	now := time.Now().Format("20060102150405")
	chartRef := fmt.Sprintf("oci://ttl.sh/automated-%s/wrapped-chart", now)
	chartFile := "/chart/test-chart-0.1.0.tgz"

	_ = dag.Container().From("alpine/helm:latest").
		WithFile("/chart/test-chart-0.1.0.tgz", wrappedChart).
		WithExec([]string{"helm", "push", chartFile, chartRef})
	fmt.Printf("\n\nWrapped chart pushed to %s:0.1.0\n\n", chartRef)

	// Print summary at the end
	fmt.Printf("\n========================================\n")
	fmt.Printf("Build Complete - Artifacts Published:\n")
	fmt.Printf("========================================\n\n")
	fmt.Printf("1. Image:         %s/%s:%s\n", imageRegistry, imageRepository, imageTag)
	fmt.Printf("2. SDK Chart:     %s:1.0.0\n", chart)
	fmt.Printf("3. Wrapper Chart: %s:0.1.0\n", chartRef)
	fmt.Printf("\n========================================\n\n")

	return nil
}
