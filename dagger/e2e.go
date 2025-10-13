package main

import (
	"context"
	"dagger/replicated-sdk/internal/dagger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// kubeconfigPath is the location inside the container where we mount the kubeconfig.
// The Bitnami images we use run as the non-root user 1001, which cannot access /root,
// so we place the file in that user’s home directory instead.
const kubeconfigPath = "/home/1001/.kube/config"

func e2e(
	ctx context.Context,
	source *dagger.Directory,
	opServiceAccount *dagger.Secret,
	appID string,
	customerID string,
	sdkImage string,
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

	kubeconfigSource := source.WithNewFile("/kubeconfig", kubeconfig, dagger.DirectoryWithNewFileOpts{
		Permissions: 0644,
	})

	// if the cluster type is eks, we need to patch the storage class to be default - otherwise the statefulset will fail to create
	// kubectl patch storageclass gp2 -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
	if distribution == "eks" {
		fmt.Println("Patching eks gp2 storage class to be default...")
		ctr = dag.Container().From("bitnami/kubectl:latest").
			WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
			WithEnvVariable("KUBECONFIG", kubeconfigPath).
			WithExec([]string{"kubectl", "patch", "storageclass", "gp2", "-p", `{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}`})
		out, err = ctr.Stdout(ctx)
		if err != nil {
			stderr, _ := ctr.Stderr(ctx)
			fmt.Printf("failed to patch storage class: %v\n\nStderr: %s\n\nStdout: %s", err, stderr, out)
			return fmt.Errorf("failed to patch storage class: %w", err)
		}
		fmt.Println(out)
	}

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
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
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
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
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
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
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
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithFile("/certs/test-cert.crt", certDir.File("/test-cert.crt")).
		WithFile("/certs/test-key.key", certDir.File("/test-key.key")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
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
	err = upgradeChartAndRestart(ctx, kubeconfigSource, licenseID, channelSlug, []string{"--set", "replicated.tlsCertSecretName=test-tls"})
	if err != nil {
		return fmt.Errorf("failed to upgrade chart enabling TLS: %w", err)
	}

	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
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
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
		With(CacheBustingExec(
			[]string{
				"kubectl", "get", "pods",
			}))

	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pods: %w", err)
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
        image: docker.io/alpine/curl:latest
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
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
		WithFile("/tmp/replicated-ssl-test.yaml", deploymentSource.File("/replicated-ssl-test.yaml")).
		WithExec([]string{"kubectl", "apply", "-f", "/tmp/replicated-ssl-test.yaml"})
	out, err = ctr.Stdout(ctx)
	if err != nil {
		// Get stderr to see the actual error
		stderr, _ := ctr.Stderr(ctx)
		return fmt.Errorf("failed to apply replicated-ssl-test deployment: %w\n\nStderr: %s\n\nStdout: %s", err, stderr, out)
	}
	fmt.Println(out)

	// wait for the replicated-ssl-test deployment to be ready
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
		WithExec([]string{"kubectl", "wait", "--for=condition=available", "deployment/replicated-ssl-test", "--timeout=1m"})
	out, err = ctr.Stdout(ctx)
	if err != nil {
		ctr = dag.Container().From("bitnami/kubectl:latest").
			WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
			WithEnvVariable("KUBECONFIG", kubeconfigPath).
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
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
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
	err = upgradeChartAndRestart(ctx, kubeconfigSource, licenseID, channelSlug, []string{
		"--set", "replicated.tlsCertSecretName=test-tls",
		"--set", "replicated.minimalRBAC=true",
		"--set-json", `replicated.statusInformers=["deployment/test-chart","service/test-chart","daemonset/test-daemonset","statefulset/test-statefulset","pvc/test-pvc"]`,
	})
	if err != nil {
		return fmt.Errorf("failed to upgrade chart enabling minimal RBAC: %w", err)
	}

	// Check the role to verify minimal RBAC is applied
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
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
	// persistentvolumeclaims      []                 [test-pvc]        [get]
	// persistentvolumeclaims      []                 []                                       [list watch]
	if !regexp.MustCompile(`persistentvolumeclaims +\[\] +\[test-pvc\] +\[get\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain 'test-pvc' PVC get permission as expected")
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

	// Get final pod status
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
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
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
		With(CacheBustingExec(
			[]string{
				"kubectl", "logs", "deployment/replicated", "--tail=100",
			}))
	out, err2 := ctr.Stdout(ctx)
	if err2 != nil {
		return fmt.Errorf("failed to get logs for replicated deployment: %w", err2)
	}
	fmt.Println("SDK logs after minimal RBAC test:")
	fmt.Println(out)

	// Extract instanceAppID from the SDK logs
	var instanceAppID string
	lines := strings.Split(out, "\n")
	instanceAppIDRegex := regexp.MustCompile(`appID:\s+([a-f0-9-]+)`)
	for _, line := range lines {
		if match := instanceAppIDRegex.FindStringSubmatch(line); match != nil {
			instanceAppID = match[1]
			fmt.Printf("Extracted instanceAppID: %s\n", instanceAppID)
			break
		}
	}
	if instanceAppID == "" {
		return fmt.Errorf("instanceAppID not found in SDK logs")
	}

	// make a request to https://api.replicated.com/v1/instance/{instanceID}/events?pageSize=500
	tokenPlaintext, err := replicatedServiceAccount.Plaintext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get service account token: %w", err)
	}

	resourceNames := []Resource{
		{Kind: "deployment", Name: "test-chart"},
		{Kind: "service", Name: "test-chart"},
		{Kind: "daemonset", Name: "test-daemonset"},
		{Kind: "statefulset", Name: "test-statefulset"},
		{Kind: "persistentvolumeclaim", Name: "test-pvc"},
	}

	// Retry up to 5 times with 30 seconds between attempts
	err = waitForResourcesReady(ctx, resourceNames, 30, 5*time.Second, tokenPlaintext, instanceAppID, distribution)
	if err != nil {
		return fmt.Errorf("failed to wait for resources to be ready: %w", err)
	}

	// Upgrade the chart to enable minimal RBAC without status informers - this looks for a resource that has not been previously reported
	err = upgradeChartAndRestart(ctx, kubeconfigSource, licenseID, channelSlug, []string{
		"--set", "replicated.tlsCertSecretName=test-tls",
		"--set", "replicated.minimalRBAC=true",
		"--set", "replicated.statusInformers=null",
	})
	if err != nil {
		return fmt.Errorf("failed to upgrade chart enabling minimal RBAC without status informers: %w", err)
	}

	newResourceNames := []Resource{
		{Kind: "deployment", Name: "second-test-chart"},
		{Kind: "service", Name: "replicated"},
	}
	err = waitForResourcesReady(ctx, newResourceNames, 30, 5*time.Second, tokenPlaintext, instanceAppID, distribution)
	if err != nil {
		return fmt.Errorf("failed to wait for resources to be ready: %w", err)
	}

	fmt.Printf("E2E test for distribution %s and version %s passed\n", distribution, version)

	// Validate running images via vendor API
	// 1. Call vendor API to get running images for this instance
	imagesSet, err := getRunningImages(ctx, appID, customerID, instanceAppID, tokenPlaintext)
	if err != nil {
		return fmt.Errorf("failed to get running images: %w", err)
	}

	// 2. Validate expected images
	required := []string{"docker.io/library/nginx:latest", "docker.io/library/nginx:alpine", strings.TrimSpace(sdkImage)}
	forbidden := []string{"docker.io/alpine/curl:latest"}
	missing := []string{}
	for _, img := range required {
		if img == "" {
			continue
		}
		if _, ok := imagesSet[img]; !ok {
			missing = append(missing, img)
		}
	}
	if len(missing) > 0 {
		// Build a small preview of what we saw for debugging
		seen := make([]string, 0, len(imagesSet))
		for k := range imagesSet {
			seen = append(seen, k)
		}
		return fmt.Errorf("running images missing expected entries: %v. Seen: %v", missing, seen)
	}
	for _, img := range forbidden {
		if _, ok := imagesSet[img]; ok {
			return fmt.Errorf("running images contains forbidden entry: %s", img)
		}
	}

	return nil
}

type Event struct {
	ReportedAt    string `json:"reportedAt"`
	FieldName     string `json:"fieldName"`
	IsCustom      bool   `json:"isCustom"`
	PreviousValue string `json:"previousValue"`
	NewValue      string `json:"newValue"`
	Meta          struct {
		ResourceStates []struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			State     string `json:"state"`
			Namespace string `json:"namespace"`
		} `json:"resourceStates"`
	} `json:"meta"`
}

func getEvents(ctx context.Context, authToken string, instanceAppID string) ([]Event, error) {
	url := fmt.Sprintf("https://api.replicated.com/v1/instance/%s/events?pageSize=500", instanceAppID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	type Resp struct {
		Events []Event `json:"events"`
	}

	var respObj Resp
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return respObj.Events, nil
}

func checkResourceUpdatingOrReady(events []Event, kind string, name string) bool {
	foundFalse := false
	for _, event := range events {
		if event.FieldName == "appStatus" {
			for _, resourceState := range event.Meta.ResourceStates {
				if resourceState.Kind == kind && resourceState.Name == name {
					fmt.Printf("%s resourceState for %s %s: %s\n", event.ReportedAt, kind, name, resourceState.State)
					if resourceState.State == "ready" || resourceState.State == "updating" {
						return true
					} else {
						foundFalse = true
					}
				}
			}
		}
	}

	if foundFalse {
		fmt.Printf("only found not ready states for %s %s\n", kind, name)
	} else {
		fmt.Printf("did not find %s %s in any events\n", kind, name)
	}

	return false
}

type Resource struct {
	Kind string
	Name string
}

func waitForResourcesReady(ctx context.Context, resources []Resource, maxRetries int, retryInterval time.Duration, authToken string, instanceAppID string, distribution string) error {

	for attempt := 1; attempt <= maxRetries; attempt++ {
		fmt.Printf("Attempt %d/%d: Checking resource states...\n", attempt, maxRetries)

		events, err := getEvents(ctx, authToken, instanceAppID)
		if err != nil {
			if attempt == maxRetries {
				return fmt.Errorf("failed to get events after %d attempts: %w", maxRetries, err)
			}
			fmt.Printf("Failed to get events on attempt %d: %v\n", attempt, err)
			time.Sleep(retryInterval)
			continue
		}

		allResourcesReady := true
		for _, resourceName := range resources {
			if !checkResourceUpdatingOrReady(events, resourceName.Kind, resourceName.Name) {
				fmt.Printf("%s Resource %s %s is not ready on attempt %d\n", distribution, resourceName.Kind, resourceName.Name, attempt)
				allResourcesReady = false
			} else {
				fmt.Printf("%s Resource %s %s is ready\n", distribution, resourceName.Kind, resourceName.Name)
			}
		}

		if allResourcesReady {
			fmt.Printf("%s All resources are ready after %d attempt(s)\n", distribution, attempt)
			return nil
		}

		if attempt == maxRetries {
			eventJson, err := json.Marshal(events)
			if err != nil {
				return fmt.Errorf("failed to marshal events: %w", err)
			}
			fmt.Printf("%s events: %s\n", distribution, string(eventJson))
			return fmt.Errorf("not all resources are ready after %d attempts", maxRetries)
		}

		fmt.Printf("Waiting %s before next attempt...\n", retryInterval)
		time.Sleep(retryInterval)
	}
	return fmt.Errorf("unreachable code")
}

// getRunningImages calls the vendor API to retrieve running images for the given instance and returns a set of image names.
func getRunningImages(ctx context.Context, appID string, customerID string, instanceAppID string, authToken string) (map[string]struct{}, error) {
	url := fmt.Sprintf("https://api.replicated.com/vendor/v3/app/%s/customer/%s/instance/%s/running-images", appID, customerID, instanceAppID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vendor API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response struct {
		RunningImages map[string][]string `json:"running_images"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal vendor API response: %w", err)
	}
	if len(response.RunningImages) == 0 {
		return nil, fmt.Errorf("vendor API returned no running_images: %s", string(body))
	}

	imagesSet := map[string]struct{}{}
	for name := range response.RunningImages {
		if name != "" {
			imagesSet[name] = struct{}{}
		}
	}
	return imagesSet, nil
}

// upgradeChartAndRestart upgrades the helm chart with the provided arguments and restarts deployments
func upgradeChartAndRestart(
	ctx context.Context,
	kubeconfigSource *dagger.Directory,
	licenseID string,
	channelSlug string,
	helmArgs []string,
) error {
	// Helm upgrade
	upgradeCmd := []string{"helm", "upgrade", "test-chart", fmt.Sprintf("oci://registry.replicated.com/replicated-sdk-e2e/%s/test-chart", channelSlug), "--version", "0.1.0"}
	upgradeCmd = append(upgradeCmd, helmArgs...)

	ctr := dag.Container().From("alpine/helm:latest").
		WithFile("/root/.kube/config", kubeconfigSource.File("/kubeconfig")).
		WithExec([]string{"helm", "registry", "login", "registry.replicated.com", "--username", "test-customer@replicated.com", "--password", licenseID}).
		With(CacheBustingExec(upgradeCmd))

	out, err := ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to upgrade chart: %w", err)
	}
	fmt.Println(out)

	// Restart replicated deployment
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
		With(CacheBustingExec(
			[]string{
				"kubectl", "rollout", "restart", "deploy/replicated",
			}))
	out, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to restart replicated deployment: %w", err)
	}
	fmt.Println(out)

	// Wait for replicated deployment to be ready
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
		With(CacheBustingExec(
			[]string{
				"kubectl", "rollout", "status",
				"deploy/replicated",
				"--timeout=1m",
			}))

	out, err = ctr.Stdout(ctx)
	if err != nil {
		fmt.Printf("failed to wait for replicated deployment to rollout: %v\n", err)

		// Get logs to help debug if replicated didn't start properly
		ctr = dag.Container().From("bitnami/kubectl:latest").
			WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
			WithEnvVariable("KUBECONFIG", kubeconfigPath).
			With(CacheBustingExec(
				[]string{
					"kubectl", "logs", "-l", "app.kubernetes.io/name=replicated", "--tail=50",
				}))
		out, err2 := ctr.Stdout(ctx)
		if err2 != nil {
			return fmt.Errorf("failed to get logs for replicated deployment: %w", err2)
		}
		fmt.Println("Replicated logs:")
		fmt.Println(out)

		return fmt.Errorf("failed to wait for replicated deployment to rollout: %w", err)
	}
	fmt.Println(out)

	// Restart test-chart deployment
	ctr = dag.Container().From("bitnami/kubectl:latest").
		WithFile(kubeconfigPath, kubeconfigSource.File("/kubeconfig")).
		WithEnvVariable("KUBECONFIG", kubeconfigPath).
		With(CacheBustingExec(
			[]string{
				"kubectl", "rollout", "restart", "deploy/test-chart",
			}))
	_, err = ctr.Stdout(ctx)
	if err != nil {
		return fmt.Errorf("failed to restart test-chart deployment: %w", err)
	}

	return nil
}
