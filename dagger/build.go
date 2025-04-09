package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"strings"
	"time"
)

func buildAndPushImageToTTL(
	ctx context.Context,
	source *dagger.Directory,
) (string, string, string, error) {
	now := time.Now().Format("20060102150405")

	_, err := source.
		DockerBuild(dagger.DirectoryDockerBuildOpts{
			Platform:   "linux/amd64",
			Dockerfile: "deploy/Dockerfile",
		}).
		Publish(ctx, fmt.Sprintf("ttl.sh/automated-%s/replicated-image/replicated-sdk:24h", now))

	if err != nil {
		return "", "", "", err
	}

	return "ttl.sh", fmt.Sprintf("automated-%s/replicated-image/replicated-sdk", now), "24h", nil
}

func buildAndPushChartToTTL(
	ctx context.Context,
	source *dagger.Directory,
	imageRegistry string,
	imageRepository string,
	imageTag string,
) (string, error) {
	source = source.Directory("chart")
	valuesYAML, err := source.File("values.yaml").Contents(ctx)
	if err != nil {
		return "", err
	}

	valuesYAML = strings.Replace(valuesYAML, "registry: registry.replicated.com", fmt.Sprintf("registry: %s", imageRegistry), 1)
	valuesYAML = strings.Replace(valuesYAML, `repository: "library/replicated-sdk"`, fmt.Sprintf(`repository: "%s"`, imageRepository), 1)
	valuesYAML = strings.Replace(valuesYAML, `tag: "1.0.0"`, fmt.Sprintf(`tag: "%s"`, imageTag), 1)

	source = source.WithNewFile("values.yaml", valuesYAML)

	now := time.Now().Format("20060102150405")
	chartRef := fmt.Sprintf("oci://ttl.sh/automated-%s/replicated-chart", now)
	chartFile := "/chart/replicated-1.0.0.tgz"

	ctr := dag.Container().From("alpine/helm:latest").
		WithMountedDirectory("/chart", source).
		WithWorkdir("/chart").
		WithExec([]string{"helm", "package", "."}).
		WithExec([]string{"helm", "push", chartFile, chartRef})

	stdout, err := ctr.Stdout(ctx)
	if err != nil {
		return "", err
	}

	fmt.Println(stdout)

	return chartRef, nil
}
