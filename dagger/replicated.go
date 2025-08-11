package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// getAppID resolves the application ID for a given app slug via the vendor API
func getAppID(
	ctx context.Context,
	opServiceAccount *dagger.Secret,
	appSlug string,
) (string, error) {
	token := mustGetSecret(ctx, opServiceAccount, "Replicated", "service_account", VaultDeveloperAutomation)
	tokenPlaintext, err := token.Plaintext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get service account token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.replicated.com/vendor/v3/apps", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", tokenPlaintext)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vendor API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	type app struct {
		ID   string `json:"appId"`
		Slug string `json:"appSlug"`
	}

	var listResp struct {
		Apps []app `json:"apps"`
	}
	if err := json.Unmarshal(body, &listResp); err == nil && len(listResp.Apps) > 0 {
		for _, a := range listResp.Apps {
			if a.Slug == appSlug {
				return a.ID, nil
			}
		}
	}

	return "", fmt.Errorf("app with slug %q not found in vendor API response", appSlug)
}
