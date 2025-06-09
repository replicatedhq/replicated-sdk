package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"encoding/json"
	"fmt"
	"time"
)

// createWrappedTestChart creates a test chart with the replicated dependency and returns the chart file
func createWrappedTestChart(
	ctx context.Context,
	source *dagger.Directory,
	sdkChartRepository string,
) (*dagger.File, error) {
	source = source.Directory("test-chart")

	chartYAML, err := source.File("Chart.yaml").Contents(ctx)
	if err != nil {
		return nil, err
	}

	// append the dependency to the test-chart chart.yaml
	chartYAML = fmt.Sprintf(`%s
dependencies:
- name: replicated
  version: 1.0.0
  repository: %s
`, chartYAML, sdkChartRepository)

	fmt.Println(chartYAML)
	source = source.WithNewFile("Chart.yaml", chartYAML)

	testChartFile := dag.Container().From("alpine/helm:latest").
		WithMountedDirectory("/chart", source).
		WithWorkdir("/chart").
		WithExec([]string{"helm", "dep", "update"}).
		WithExec([]string{"helm", "package", "."}).
		File("test-chart-0.1.0.tgz")

	return testChartFile, nil
}

func createAppTestRelease(
	ctx context.Context,
	source *dagger.Directory,
	opServiceAccount *dagger.Secret,
	sdkChartRepository string,
) (string, error) {
	replicatedServiceAccount := mustGetSecret(ctx, opServiceAccount, "Replicated", "service_account", VaultDeveloperAutomation)

	testChartFile, err := createWrappedTestChart(ctx, source, sdkChartRepository)
	if err != nil {
		return "", err
	}

	now := time.Now().Format("20060102150405")
	channelName := fmt.Sprintf("automated-%s", now)

	ctr := dag.Container().From("replicated/vendor-cli:latest").
		WithSecretVariable("REPLICATED_API_TOKEN", replicatedServiceAccount).
		WithMountedFile("/test-chart-0.1.0.tgz", testChartFile).
		WithExec([]string{"/replicated", "channel", "create", "--app", "replicated-sdk-e2e", "--name", channelName}).
		WithExec([]string{"/replicated", "release", "create", "--app", "replicated-sdk-e2e", "--version", "0.1.0", "--promote", channelName, "--chart", "/test-chart-0.1.0.tgz"})

	out, err := ctr.Stdout(ctx)
	if err != nil {
		return "", err
	}

	fmt.Println(out)

	return channelName, nil
}

func createCustomer(
	ctx context.Context,
	channelSlug string,
	opServiceAccount *dagger.Secret,
) (string, string, error) {
	replicatedServiceAccount := mustGetSecret(ctx, opServiceAccount, "Replicated", "service_account", VaultDeveloperAutomation)

	ctr := dag.Container().From("replicated/vendor-cli:latest").
		WithSecretVariable("REPLICATED_API_TOKEN", replicatedServiceAccount).
		WithExec([]string{"/replicated", "customer", "create", "--app", "replicated-sdk-e2e", "--kots-install=false", "--name", "test-customer", "--channel", channelSlug, "--email", "test-customer@replicated.com", "--output", "json"})

	out, err := ctr.Stdout(ctx)
	if err != nil {
		return "", "", err
	}

	type ReplicatedCustomer struct {
		ID             string `json:"id"`
		InstallationID string `json:"installationId"`
	}
	replicatedCustomer := ReplicatedCustomer{}
	if err := json.Unmarshal([]byte(out), &replicatedCustomer); err != nil {
		return "", "", err
	}

	return replicatedCustomer.ID, replicatedCustomer.InstallationID, nil
}
