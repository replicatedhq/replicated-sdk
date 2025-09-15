package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"os"
	"strings"
	"time"
)

// SecureBuild environment constants
const (
	SecureBuildEnvDev        = "dev"
	SecureBuildEnvStaging    = "staging"
	SecureBuildEnvProduction = "production"
)

// isProductionEnvironment determines if environment uses production credentials
func isProductionEnvironment(environment string) bool {
	return environment == SecureBuildEnvProduction
}

// buildAndPushImageToTTL builds and pushes development images
// Supports both TTL.sh (legacy) and SecureBuild via feature flag
func buildAndPushImageToTTL(
	ctx context.Context,
	source *dagger.Directory,
) (string, string, string, error) {
	// Check for SecureBuild feature flag
	if useSecureBuild := os.Getenv("USE_SECUREBUILD"); useSecureBuild == "true" {
		// For development builds, use placeholder opServiceAccount
		opServiceAccount := dag.SetSecret("op-service-account", "placeholder")
		return buildAndPushImageWithSecureBuild(ctx, source, SecureBuildEnvDev, "", opServiceAccount)
	}

	// Default to TTL.sh for backward compatibility
	return buildAndPushImageToTTLLegacy(ctx, source)
}

// buildAndPushImageToTTLLegacy is the original TTL.sh implementation
func buildAndPushImageToTTLLegacy(
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

// buildAndPushImageWithSecureBuild is the unified entry point for all SecureBuild builds
// Routes to appropriate SecureBuild function based on environment
func buildAndPushImageWithSecureBuild(
	ctx context.Context,
	source *dagger.Directory,
	environment string,
	version string,
	opServiceAccount *dagger.Secret,
) (string, string, string, error) {
	fmt.Printf("Using SecureBuild for %s image building\n", environment)

	switch environment {
	case SecureBuildEnvDev:
		// Generate dev version if not provided
		if version == "" {
			now := time.Now().Format("20060102150405")
			version = fmt.Sprintf("dev-%s", now)
		}

		imageRef, err := (&ReplicatedSdk{}).BuildDevSecureBuild(ctx, source, version, opServiceAccount)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to build with SecureBuild dev: %w", err)
		}

		fmt.Printf("SecureBuild: Development build completed, image available at %s\n", imageRef)

		// Parse TTL.sh URL to extract components (ttl.sh/automated-20240101120000/replicated-image/replicated-sdk:version)
		// Return format should match legacy: (registry, repository, tag)
		if strings.HasPrefix(imageRef, "ttl.sh/") {
			// Extract parts: ttl.sh/automated-20240101120000/replicated-image/replicated-sdk:version
			parts := strings.Split(imageRef, "/")
			if len(parts) >= 4 {
				registry := "ttl.sh"
				repository := strings.Join(parts[1:len(parts)-1], "/") + "/" + strings.Split(parts[len(parts)-1], ":")[0]
				tag := strings.Split(parts[len(parts)-1], ":")[1]
				return registry, repository, tag, nil
			}
		}

		// Fallback to cve0.io format if parsing fails
		// Uses consistent image naming with SecureBuild pipeline
		return "cve0.io", "replicated-sdk", version, nil

	case SecureBuildEnvStaging:
		imageRef, err := (&ReplicatedSdk{}).PublishSecureBuild(ctx, source, version, SecureBuildEnvStaging, opServiceAccount)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to build with SecureBuild %s: %w", environment, err)
		}

		fmt.Printf("SecureBuild: %s build completed, image available at %s\n", environment, imageRef)
		return "cve0.io", "replicated-sdk", version, nil

	case SecureBuildEnvProduction:
		imageRef, err := (&ReplicatedSdk{}).PublishSecureBuild(ctx, source, version, SecureBuildEnvProduction, opServiceAccount)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to build with SecureBuild %s: %w", environment, err)
		}

		fmt.Printf("SecureBuild: %s build completed, image available at %s\n", environment, imageRef)
		return "cve0.io", "replicated-sdk", version, nil

	default:
		return "", "", "", fmt.Errorf("unsupported SecureBuild environment: %s (use %s, %s, or %s)",
			environment, SecureBuildEnvDev, SecureBuildEnvStaging, SecureBuildEnvProduction)
	}
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
