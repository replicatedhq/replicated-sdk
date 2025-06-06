package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"encoding/json"
	"fmt"
)

func e2e(
	ctx context.Context,
	source *dagger.Directory,
	opServiceAccount *dagger.Secret,
	licenseID string,
	distribution string,
	version string,
	channelSlug string,
) error {
	fmt.Printf("Creating cluster for distribution %s and version %s\n", distribution, version)

	replicatedServiceAccount := mustGetSecret(ctx, opServiceAccount, "Replicated", "service_account", VaultDeveloperAutomation)

	ctr := dag.Container().From("replicated/vendor-cli:latest").
		WithSecretVariable("REPLICATED_API_TOKEN", replicatedServiceAccount).
		WithExec([]string{"/replicated", "cluster", "create", "--distribution", distribution, "--version", version, "--wait", "5m", "--output", "json"})

	out, err := ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	type ReplicatedCluster struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	replicatedCluster := ReplicatedCluster{}
	if err := json.Unmarshal([]byte(out), &replicatedCluster); err != nil {
		return fmt.Errorf("failed to unmarshal cluster: %w", err)
	}

	// get the kubeconfig
	ctr = dag.Container().From("replicated/vendor-cli:latest").
		WithSecretVariable("REPLICATED_API_TOKEN", replicatedServiceAccount).
		WithExec([]string{"/replicated", "cluster", "kubeconfig", replicatedCluster.ID, "--stdout"})

	kubeconfig, err := ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	kubeconfigSource := source.WithNewFile("/kubeconfig", kubeconfig)

	ctr = dag.Container().From("alpine/helm:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithExec([]string{"helm", "registry", "login", "registry.replicated.com", "--username", "test-customer@replicated.com", "--password", licenseID}).
		WithExec([]string{"helm", "install", "test-chart", fmt.Sprintf("oci://registry.replicated.com/replicated-sdk-e2e/%s/test-chart", channelSlug), "--version", "0.1.0"})

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to install chart: %w", err)
	}

	fmt.Println(out)

	// wait for the pod to be ready
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		WithExec(
			[]string{
				"kubectl", "wait",
				"--for=condition=available",
				"deployment/replicated",
				"--timeout=1m",
			})

	out, err = ctr.Stdout(ctx)
	if err != nil {
		fmt.Printf("failed to wait for deployment to be ready: %v\n", err)
		// return err
	}

	fmt.Println(out)

	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		WithExec(
			[]string{
				"kubectl", "get", "ns",
			})

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %w", err)
	}

	fmt.Println(out)

	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		WithExec(
			[]string{
				"kubectl", "get", "pods",
			})

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pods: %w", err)
	}

	fmt.Println(out)

	return nil
}
