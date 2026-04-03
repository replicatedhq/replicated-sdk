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
) (string, string, string, string, error) {
	now := time.Now().Format("20060102150405")
	// Use a valid melange version for package building and apko package pinning.
	// No dashes allowed — sanitizeVersionForMelange converts them to underscores
	// which melange then rejects. Dots are safe.
	// The image tag is "24h" (ttl.sh expiry duration), separate from the build version.
	version := fmt.Sprintf("0.0.%s", now)

	amdPackages, armPackages, melangeKey, err := buildImage(ctx, dag, source, version, []string{"x86_64"})
	if err != nil {
		return "", "", "", "", err
	}

	imagePath := fmt.Sprintf("ttl.sh/automated-%s/replicated-image/replicated-sdk", now)
	digest, err := publishImage(ctx, dag, source, amdPackages, armPackages, melangeKey,
		version, "24h", imagePath, "", nil, nil, nil)
	if err != nil {
		return "", "", "", "", err
	}

	return "ttl.sh", fmt.Sprintf("automated-%s/replicated-image/replicated-sdk", now), "24h", digest, nil
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

	valuesYAML = strings.Replace(valuesYAML, "registry: proxy.replicated.com", fmt.Sprintf("registry: %s", imageRegistry), 1)
	valuesYAML = strings.Replace(valuesYAML, `repository: "library/replicated-sdk-image"`, fmt.Sprintf(`repository: "%s"`, imageRepository), 1)
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
