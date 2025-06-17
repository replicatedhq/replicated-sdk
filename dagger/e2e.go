package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
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
		WithExec([]string{"/replicated", "cluster", "create", "--distribution", distribution, "--version", version, "--wait", "15m", "--output", "json"})

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
		With(CacheBustingExec(
			[]string{
				"kubectl", "get", "ns",
			}))

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %w", err)
	}

	fmt.Println(out)

	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		With(CacheBustingExec(
			[]string{
				"kubectl", "get", "pods",
			}))

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pods: %w", err)
	}

	fmt.Println(out)

	// create a tls cert and key
	certDir := dag.Container().From("alpine/openssl:latest").
		WithWorkdir("/certs").
		WithExec([]string{"openssl", "req", "-x509", "-newkey", "rsa:4096", "-keyout", "/certs/test-key.key", "-out", "/certs/test-cert.crt", "-days", "365", "-nodes", "-subj", "/CN=test.com"}).
		WithExec([]string{"chmod", "+r", "/certs/test-cert.crt"}).
		WithExec([]string{"chmod", "+r", "/certs/test-key.key"}).
		Directory("/certs")

	// create a TLS secret within the namespace
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithFile("/certs/test-cert.crt", certDir.File("/test-cert.crt")).
		WithFile("/certs/test-key.key", certDir.File("/test-key.key")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		WithExec(
			[]string{
				"kubectl", "create", "secret", "tls", "test-tls", "--cert=/certs/test-cert.crt", "--key=/certs/test-key.key",
			})
	out, err = ctr.Stdout(ctx)
	if err != nil {
		stderr, _ := ctr.Stderr(ctx)
		return fmt.Errorf("failed to create tls secret: %w\n\nStderr: %s\n\nStdout: %s", err, stderr, out)
	}
	fmt.Println(out)

	// update the chart to set TLS to true
	ctr = dag.Container().From("alpine/helm:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithExec([]string{"helm", "registry", "login", "registry.replicated.com", "--username", "test-customer@replicated.com", "--password", licenseID}).
		WithExec([]string{"helm", "upgrade", "test-chart", fmt.Sprintf("oci://registry.replicated.com/replicated-sdk-e2e/%s/test-chart", channelSlug), "--version", "0.1.0", "--set", "replicated.tlsCertSecretName=test-tls"})

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to upgrade chart enabling TLS: %w", err)
	}
	fmt.Println(out)

	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		With(CacheBustingExec(
			[]string{
				"kubectl", "get", "secrets",
			}))

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get secrets: %w", err)
	}

	fmt.Println(out)

	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		With(CacheBustingExec(
			[]string{
				"kubectl", "get", "pods",
			}))

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pods: %w", err)
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
		fmt.Printf("failed to wait for deployment to be ready after enabling TLS: %v\n", err)
		// return err
	}
	fmt.Println(out)

	// create a kubernetes deployment that runs a pod - the pod has a readiness probe that runs 'curl -k https://replicated.svc:3000/health'
	// this will only pass if the replicated pod is ready and serving TLS traffic
	deploymentYaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: replicated-ssl-test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: replicated-ssl-test
  template:
    metadata:
      labels:
        app: replicated-ssl-test
    spec:
      containers:
      - name: replicated-ssl-test
        image: alpine/curl:latest
        command: ["sleep", "500d"]
        ports:
        - containerPort: 3000
        readinessProbe:
          exec:
            command: ["curl", "-k", "https://replicated:3000/health"]
          initialDelaySeconds: 10
          periodSeconds: 10`
	deploymentSource := source.WithNewFile("/replicated-ssl-test.yaml", deploymentYaml)

	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		WithFile("/root/replicated-ssl-test.yaml", deploymentSource.File("/replicated-ssl-test.yaml")).
		WithExec([]string{"kubectl", "apply", "-f", "/root/replicated-ssl-test.yaml"})
	out, err = ctr.Stdout(ctx)
	if err != nil {
		// Get stderr to see the actual error
		stderr, _ := ctr.Stderr(ctx)
		return fmt.Errorf("failed to apply replicated-ssl-test deployment: %w\n\nStderr: %s\n\nStdout: %s", err, stderr, out)
	}
	fmt.Println(out)

	// wait for the replicated-ssl-test deployment to be ready
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		WithExec([]string{"kubectl", "wait", "--for=condition=available", "deployment/replicated-ssl-test", "--timeout=1m"})
	out, err = ctr.Stdout(ctx)
	if err != nil {
		ctr = dag.Container().From("bitnami/kubectl:latest").
			WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
			WithEnvVariable("KUBECONFIG", "/root/.kube/config").
			WithExec([]string{"kubectl", "logs", "-p", "-l", "app.kubernetes.io/name=replicated"})
		out, err2 := ctr.Stdout(ctx)
		if err2 != nil {
			return fmt.Errorf("failed to get logs for replicated deployment: %w", err2)
		}
		fmt.Println(out)

		return fmt.Errorf("failed to wait for replicated deployment to be ready: %w", err)
	}
	fmt.Println(out)

	// print the final pods
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		With(CacheBustingExec(
			[]string{
				"kubectl", "get", "pods",
			}))
	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pods: %w", err)
	}
	fmt.Println(out)

	// Test minimal RBAC functionality
	fmt.Println("Testing minimal RBAC functionality...")

	// Upgrade the chart to enable minimal RBAC
	ctr = dag.Container().From("alpine/helm:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithExec([]string{"helm", "registry", "login", "registry.replicated.com", "--username", "test-customer@replicated.com", "--password", licenseID}).
		WithExec([]string{"helm", "upgrade", "test-chart",
			fmt.Sprintf("oci://registry.replicated.com/replicated-sdk-e2e/%s/test-chart", channelSlug),
			"--version", "0.1.0",
			"--set", "replicated.tlsCertSecretName=test-tls",
			"--set", "replicated.minimalRBAC=true",
		})

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to upgrade chart enabling minimal RBAC: %w", err)
	}
	fmt.Println(out)

	// Check the role to verify minimal RBAC is applied
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		With(CacheBustingExec(
			[]string{
				"kubectl", "describe", "role", "replicated-role",
			}))

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to describe role: %w", err)
	}
	fmt.Println("Role permissions after enabling minimal RBAC:")
	fmt.Println(out)

	// Validate that the role contains the expected resources
	roleOutput := out

	// Check for replicated deployment in the role - note that this is not listed as a status informer, but internal code requires it
	// deployments.apps             []                 [replicated]                   [get list watch]
	if !regexp.MustCompile(`deployments\.apps +\[\] +\[replicated\] +\[get\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain 'replicated' deployment that is required by default")
	}

	// Check for test-tls secret in the role - internal code requires it
	// secrets                      []                 [test-tls]                              [get]
	if !regexp.MustCompile(`secrets +\[\] +\[test-tls\] +\[get\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain 'test-tls' secret permission as expected")
	}

	// Check for test-chart deployment in the role
	// deployments.apps             []                 [test-chart]                            [get]
	// deployments.apps             []                 []                                      [list watch]
	if !regexp.MustCompile(`deployments\.apps +\[\] +\[test-chart\] +\[get\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain 'test-chart' deployment get permission as expected")
	}
	if !regexp.MustCompile(`deployments\.apps +\[\] +\[\] +\[list watch\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain deployment list watch permission as expected")
	}

	// Check for test-daemonset daemonset in the role
	// daemonsets.apps              []                 [test-daemonset]                        [get]
	// daemonsets.apps              []                 []                                      [list watch]
	if !regexp.MustCompile(`daemonsets\.apps +\[\] +\[test-daemonset\] +\[get\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain 'test-daemonset' daemonset get permission as expected")
	}
	if !regexp.MustCompile(`daemonsets\.apps +\[\] +\[\] +\[list watch\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain daemonset list watch permission as expected")
	}

	// Check for test-statefulset statefulset in the role
	// statefulsets.apps            []                 [test-statefulset]                      [get]
	// statefulsets.apps            []                 []                                      [list watch]
	if !regexp.MustCompile(`statefulsets\.apps +\[\] +\[test-statefulset\] +\[get\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain 'test-statefulset' statefulset get permission as expected")
	}
	if !regexp.MustCompile(`statefulsets\.apps +\[\] +\[\] +\[list watch\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain statefulset list watch permission as expected")
	}

	// Check for test-statefulset-0 PVC in the role
	// persistentvolumeclaims      []                 [test-storage-test-statefulset-0]        [get]
	// persistentvolumeclaims      []                 []                                       [list watch]
	if !regexp.MustCompile(`persistentvolumeclaims +\[\] +\[test-storage-test-statefulset-0\] +\[get\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain 'test-storage-test-statefulset-0' PVC get permission as expected")
	}
	if !regexp.MustCompile(`persistentvolumeclaims +\[\] +\[\] +\[list watch\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain PVC list watch permission as expected")
	}

	// Check for test-chart service in the role
	// services                      []                 [test-chart]                            [get]
	// services                      []                 []                                      [list watch]
	// endpoints                     []                 [test-chart]                            [get]
	// endpoints                     []                 []                                      [list watch]
	if !regexp.MustCompile(`services +\[\] +\[test-chart\] +\[get\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain 'test-chart' service get permission as expected")
	}
	if !regexp.MustCompile(`services +\[\] +\[\] +\[list watch\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain service list watch permission as expected")
	}
	if !regexp.MustCompile(`endpoints +\[\] +\[test-chart\] +\[get\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain 'test-chart' endpoint get permission as expected")
	}
	if !regexp.MustCompile(`endpoints +\[\] +\[\] +\[list watch\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain endpoint list watch permission as expected")
	}

	// check that there are not ingress permissions in the role
	if strings.Contains(roleOutput, "ingress") {
		return fmt.Errorf("role contains ingress permissions, which should not be present")
	}

	// restart pods from the replicated deployment to clarify logs later (don't keep a failed pod around, and there will be one from the update)
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		WithExec([]string{"kubectl", "rollout", "restart", "deploy/replicated"})
	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to restart pods from replicated deployment: %w", err)
	}

	// Wait for the pod to be ready after RBAC changes
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		WithExec(
			[]string{
				"kubectl", "rollout", "status",
				"deploy/replicated",
				"--timeout=1m",
			})

	out, err = ctr.Stdout(ctx)
	if err != nil {
		fmt.Printf("failed to wait for deployment to rollout after enabling minimal RBAC: %v\n", err)

		// Get logs to help debug
		ctr = dag.Container().From("bitnami/kubectl:latest").
			WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
			WithEnvVariable("KUBECONFIG", "/root/.kube/config").
			WithExec([]string{"kubectl", "logs", "-l", "app.kubernetes.io/name=replicated", "--tail=50"})
		out, err2 := ctr.Stdout(ctx)
		if err2 != nil {
			return fmt.Errorf("failed to get logs for replicated deployment: %w", err2)
		}
		fmt.Println("Replicated logs after minimal RBAC:")
		fmt.Println(out)

		return fmt.Errorf("failed to wait for replicated deployment to rollout after minimal RBAC: %w", err)
	}
	fmt.Println(out)

	// wait 30 seconds to let the SDK pod send updates
	time.Sleep(time.Second * 30)

	// Get final pod status
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		With(CacheBustingExec(
			[]string{
				"kubectl", "get", "pods", "-o", "wide",
			}))
	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get final pod status: %w", err)
	}
	fmt.Println("final pods:")
	fmt.Println(out)

	// get SDK logs for final debugging
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", "/root/.kube/config").
		WithExec([]string{"kubectl", "logs", "deployment/replicated", "--tail=100"})
	out, err2 := ctr.Stdout(ctx)
	if err2 != nil {
		return fmt.Errorf("failed to get logs for replicated deployment: %w", err2)
	}
	fmt.Println("SDK logs after minimal RBAC test:")
	fmt.Println(out)

	fmt.Printf("E2E test for distribution %s and version %s passed\n", distribution, version)
	return nil
}
