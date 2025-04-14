package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"fmt"
	"strings"
)

func buildAndPublishChart(
	ctx context.Context,
	dag *dagger.Client,
	source *dagger.Directory,
	version string,
	staging bool,
	production bool,
) error {
	helmChartFilename := fmt.Sprintf("replicated-%s.tgz", version)

	// we need to replace some values in the values.yaml before pushing to staging and prod

	valuesYaml, err := source.File("chart/values.yaml").Contents(ctx)
	if err != nil {
		return err
	}

	valuesYaml = strings.Replace(valuesYaml, `tag: "1.0.0"`, fmt.Sprintf(`tag: "%s"`, version), 1)

	if production {
		ctr := dag.Container().From("alpine/helm:latest").
			WithMountedDirectory("/source", source).
			WithWorkdir("/source/chart").
			WithNewFile("/source/chart/values.yaml", valuesYaml).
			WithExec([]string{"helm", "dependency", "update"}).
			WithExec([]string{"helm", "package", "--version", version, "--app-version", version, "."}).
			WithExec([]string{"helm", "push", helmChartFilename, "oci://registry.replicated.com/library"})
		stdout, err := ctr.Stdout(ctx)
		if err != nil {
			return err
		}

		fmt.Println(stdout)
	}

	if staging {
		valuesYaml = strings.Replace(valuesYaml, `registry: registry.replicated.com`, `registry: registry.staging.replicated.com`, 1)
		ctr := dag.Container().From("alpine/helm:latest").
			WithMountedDirectory("/source", source).
			WithWorkdir("/source/chart").
			WithNewFile("/source/chart/values.yaml", valuesYaml).
			WithExec([]string{"helm", "dependency", "update"}).
			WithExec([]string{"helm", "package", "--version", version, "--app-version", version, "."}).
			WithExec([]string{"helm", "push", helmChartFilename, "oci://registry.staging.replicated.com/library"})
		stdout, err := ctr.Stdout(ctx)
		if err != nil {
			return err
		}

		fmt.Println(stdout)
	}

	return nil
}
