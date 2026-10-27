//go:build unix

package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func e2e(
	ctx context.Context,
	workdir string,
	appID string,
	customerID string,
	sdkImage string,
	imageDigest string,
	licenseID string,
	distribution string,
	version string,
	channelSlug string,
) error {
	var out string
	var err error
	var command []string
	// if the cluster type is eks, we need to patch the storage class to be default - otherwise the statefulset will fail to create
	// kubectl patch storageclass gp2 -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
	if distribution == "eks" {
		fmt.Println("Patching eks gp2 storage class to be default...")
		command = []string{"kubectl", "patch", "storageclass", "gp2", "-p", `{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}`}
		out, err = run(ctx, command...)
		if err != nil {
			fmt.Printf("failed to patch storage class: %v\n\nStdout: %s", err, out)
			return fmt.Errorf("failed to patch storage class: %w", err)
		}
		fmt.Println(out)
	}

	command = []string{"helm", "install", "test-chart", fmt.Sprintf("oci://registry.replicated.com/replicated-sdk-e2e/%s/test-chart", channelSlug), "--version", "0.1.0"}

	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to install chart: %w", err)
	}

	fmt.Println(out)

	// wait for the pod to be ready
	command = []string{
		"kubectl", "wait",
		"--for=condition=available",
		"deployment/replicated",
		"--timeout=5m",
	}

	out, err = run(ctx, command...)
	if err != nil {
		fmt.Printf("failed to wait for deployment to be ready: %v\n", err)
		// return err
	}

	fmt.Println(out)

	command = []string{
		"kubectl", "get", "ns",
	}

	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %w", err)
	}

	fmt.Println(out)

	command = []string{
		"kubectl", "get", "pods",
	}

	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to get pods: %w", err)
	}

	fmt.Println(out)

	// create a tls cert and key
	_, err = run(ctx, "openssl", "req", "-x509", "-newkey", "rsa:4096", "-keyout", filepath.Join(workdir, "test-key.key"), "-out", filepath.Join(workdir, "test-cert.crt"), "-days", "365", "-nodes", "-subj", "/CN=test.com")
	if err != nil {
		return fmt.Errorf("create TLS certificate: %w", err)
	}

	// create a TLS secret within the namespace
	command = []string{
		"kubectl", "create", "secret", "tls", "test-tls", "--cert=" + filepath.Join(workdir, "test-cert.crt"), "--key=" + filepath.Join(workdir, "test-key.key"),
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to create tls secret: %w\n\nStdout: %s", err, out)
	}
	fmt.Println(out)

	// update the chart to set TLS to true
	err = upgradeChartAndRestart(ctx, workdir, licenseID, channelSlug, []string{"--set", "replicated.tlsCertSecretName=test-tls"})
	if err != nil {
		return fmt.Errorf("failed to upgrade chart enabling TLS: %w", err)
	}

	command = []string{
		"kubectl", "get", "secrets",
	}

	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to get secrets: %w", err)
	}

	fmt.Println(out)

	command = []string{
		"kubectl", "get", "pods",
	}

	out, err = run(ctx, command...)
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
	if err := os.WriteFile(filepath.Join(workdir, "replicated-ssl-test.yaml"), []byte(deploymentYaml), 0600); err != nil {
		return err
	}

	command = []string{"kubectl", "apply", "-f", filepath.Join(workdir, "replicated-ssl-test.yaml")}
	out, err = run(ctx, command...)
	if err != nil {
		// Get stderr to see the actual error
		return fmt.Errorf("failed to apply replicated-ssl-test deployment: %w\n\nStdout: %s", err, out)
	}
	fmt.Println(out)

	// wait for the replicated-ssl-test deployment to be ready
	command = []string{"kubectl", "wait", "--for=condition=available", "deployment/replicated-ssl-test", "--timeout=1m"}
	out, err = run(ctx, command...)
	if err != nil {
		command = []string{"kubectl", "logs", "-p", "-l", "app.kubernetes.io/name=replicated"}
		out, err2 := run(ctx, command...)
		if err2 != nil {
			return fmt.Errorf("failed to get logs for replicated deployment: %w", err2)
		}
		fmt.Println(out)

		return fmt.Errorf("failed to wait for replicated deployment to be ready: %w", err)
	}
	fmt.Println(out)

	// print the final pods
	command = []string{
		"kubectl", "get", "pods",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to get pods: %w", err)
	}
	fmt.Println(out)

	// Test High Availability with 3 replicas
	fmt.Println("Testing High Availability with 3 replicas...")

	// Upgrade the chart to scale to 3 replicas (no restart needed - deployment controller handles it)
	err = upgradeChartAndRestart(ctx, workdir, licenseID, channelSlug, []string{
		"--set", "replicated.tlsCertSecretName=test-tls",
		"--set", "replicated.replicaCount=3",
	})
	if err != nil {
		return fmt.Errorf("failed to upgrade chart to 3 replicas: %w", err)
	}

	// Verify there are actually 3 replicas
	command = []string{
		"kubectl", "get", "deployment", "replicated", "-o", "jsonpath={.status.replicas},{.status.readyReplicas},{.status.availableReplicas}",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to get replica counts: %w", err)
	}

	// Parse the output: replicas,readyReplicas,availableReplicas
	parts := strings.Split(strings.TrimSpace(out), ",")
	if len(parts) != 3 {
		return fmt.Errorf("unexpected replica count output format: %s", out)
	}
	if parts[0] != "3" || parts[1] != "3" || parts[2] != "3" {
		return fmt.Errorf("expected 3,3,3 replicas (replicas,ready,available) but got: %s", out)
	}
	fmt.Printf("Successfully verified 3 replicas are running: replicas=%s, ready=%s, available=%s\n", parts[0], parts[1], parts[2])

	// Verify we can see 3 pods
	command = []string{
		"kubectl", "get", "pods", "-l", "app.kubernetes.io/name=replicated",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to get replicated pods: %w", err)
	}
	fmt.Println("Replicated pods after scaling to 3:")
	fmt.Println(out)

	// Scale back down to 1 replica for subsequent tests
	fmt.Println("Scaling back down to 1 replica for remaining tests...")
	err = upgradeChartAndRestart(ctx, workdir, licenseID, channelSlug, []string{
		"--set", "replicated.tlsCertSecretName=test-tls",
		"--set", "replicated.replicaCount=1",
	})
	if err != nil {
		return fmt.Errorf("failed to scale back to 1 replica: %w", err)
	}

	// Test minimal RBAC functionality
	fmt.Println("Testing minimal RBAC functionality...")

	// Upgrade the chart to enable minimal RBAC, using image digest instead of tag
	err = upgradeChartAndRestart(ctx, workdir, licenseID, channelSlug, []string{
		"--set", "replicated.tlsCertSecretName=test-tls",
		"--set", "replicated.minimalRBAC=true",
		"--set-json", `replicated.statusInformers=["deployment/test-chart","service/test-chart","daemonset/test-daemonset","statefulset/test-statefulset","pvc/test-pvc"]`,
		"--set", fmt.Sprintf("replicated.image.digest=%s", imageDigest),
		"--set", "replicated.image.tag=",
	})
	if err != nil {
		return fmt.Errorf("failed to upgrade chart enabling minimal RBAC: %w", err)
	}

	// Verify the deployment uses a digest reference (contains @) and matches the expected digest
	command = []string{
		"kubectl", "get", "deploy/replicated", "-o", "jsonpath={.spec.template.spec.containers[0].image}",
	}
	digestImage, err := run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to get replicated deployment image: %w", err)
	}
	fmt.Printf("Deployment image with digest: %s\n", digestImage)
	if !strings.Contains(digestImage, "@") {
		return fmt.Errorf("expected image reference to contain '@' for digest, got: %s", digestImage)
	}
	if !strings.Contains(digestImage, imageDigest) {
		return fmt.Errorf("expected image reference to contain digest %s, got: %s", imageDigest, digestImage)
	}

	// Check the role to verify minimal RBAC is applied
	command = []string{
		"kubectl", "describe", "role", "replicated-role",
	}

	out, err = run(ctx, command...)
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
	// services                                []                 [test-chart]                            [get]
	// services                                []                 []                                      [list watch]
	// endpointslices.discovery.k8s.io         []                 []                                      [list]
	if !regexp.MustCompile(`services +\[\] +\[test-chart\] +\[get\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain 'test-chart' service get permission as expected")
	}
	if !regexp.MustCompile(`services +\[\] +\[\] +\[list watch\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain service list watch permission as expected")
	}
	if !regexp.MustCompile(`endpointslices.discovery.k8s.io +\[\] +\[\] +\[list\]`).MatchString(roleOutput) {
		return fmt.Errorf("role does not contain endpointslices list permission as expected")
	}

	// check that there are not ingress permissions in the role
	if strings.Contains(roleOutput, "ingress") {
		return fmt.Errorf("role contains ingress permissions, which should not be present")
	}

	// Get final pod status
	command = []string{
		"kubectl", "get", "pods", "-o", "wide",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to get final pod status: %w", err)
	}
	fmt.Println("final pods:")
	fmt.Println(out)

	// get SDK logs for final debugging
	command = []string{
		"kubectl", "logs", "deployment/replicated", "--tail=100",
	}
	out, err2 := run(ctx, command...)
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
	tokenPlaintext := os.Getenv("REPLICATED_API_TOKEN")

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

	// Test custom app metrics functionality with minimal RBAC
	// This verifies the SDK can read and write to the replicated-meta-data secret
	fmt.Println("Testing custom app metrics with minimal RBAC...")

	// Use kubectl exec to run curl from inside the replicated-ssl-test pod (which has curl installed)
	customMetricsPayload := `{"data":{"e2e_test_metric":"test_value","e2e_test_number":42}}`
	command = []string{
		"kubectl", "exec", "deployment/replicated-ssl-test", "--",
		"curl", "-k", "-sS", "-w", "\n%{http_code}",
		"--retry", "2", "--retry-delay", "5", "--retry-all-errors",
		"-X", "POST",
		"-H", "Content-Type: application/json",
		"-d", customMetricsPayload,
		"https://replicated:3000/api/v1/app/custom-metrics",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to send custom app metrics: %w\n\nStdout: %s", err, out)
	}

	// Parse the response - last line is the HTTP status code
	outputLines := strings.Split(strings.TrimSpace(out), "\n")
	if len(outputLines) < 1 {
		return fmt.Errorf("unexpected empty response from custom metrics endpoint")
	}
	httpStatus := outputLines[len(outputLines)-1]
	if httpStatus != "200" {
		return fmt.Errorf("custom metrics endpoint returned non-200 status: %s, response: %s", httpStatus, out)
	}
	fmt.Println("Successfully sent custom app metrics with minimal RBAC enabled")

	// Wait for the custom metric to appear in events
	fmt.Println("Waiting for custom metrics to be reported...")
	err = waitForCustomMetric(ctx, tokenPlaintext, instanceAppID, "e2e_test_metric", "test_value", 30, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to verify custom metrics were reported: %w", err)
	}
	fmt.Println("Custom app metrics test with minimal RBAC passed")

	// Test support bundle metadata endpoints
	err = testSupportBundleMetadata(ctx, workdir)
	if err != nil {
		return fmt.Errorf("support bundle metadata test failed: %w", err)
	}

	// Collect a real support bundle from outside the cluster and upload via port-forward.
	// The test-chart includes a support bundle spec (with supportBundleMetadata collector)
	// that is discovered via --load-cluster-specs.
	fmt.Println("Collecting and uploading real support bundle...")

	forward := exec.CommandContext(ctx, "kubectl", "port-forward", "svc/replicated", "3000:3000")
	forward.Stdout = os.Stdout
	forward.Stderr = os.Stderr
	if err := forward.Start(); err != nil {
		return err
	}
	defer func() { _ = forward.Process.Kill(); _ = forward.Wait() }()
	if err := wait(ctx, 3*time.Second); err != nil {
		return err
	}
	out, err = run(ctx, "support-bundle", "--load-cluster-specs", "--interactive=false", "--metadata", "ticketID=ISSUE-42", "-o", filepath.Join(workdir, "bundle"))
	if err != nil {
		return err
	}
	fmt.Println(out)
	out, err = run(ctx, "curl", "-k", "-sS", "-w", "\n%{http_code}", "--retry", "2", "--retry-delay", "5", "--retry-all-errors", "-X", "POST", "-H", "Content-Type: application/gzip", "--data-binary", "@"+filepath.Join(workdir, "bundle.tar.gz"), "https://localhost:3000/api/v1/supportbundle")
	if err != nil {
		return err
	}
	outputLines = strings.Split(strings.TrimSpace(out), "\n")
	httpStatus = outputLines[len(outputLines)-1]
	bundleResponseBody := strings.Join(outputLines[:len(outputLines)-1], "\n")

	if httpStatus != "201" {
		return fmt.Errorf("support bundle upload returned non-201 status: %s, response: %s", httpStatus, bundleResponseBody)
	}

	var bundleResp struct {
		BundleID string `json:"bundleId"`
		Slug     string `json:"slug"`
	}
	if err := json.Unmarshal([]byte(bundleResponseBody), &bundleResp); err != nil {
		return fmt.Errorf("failed to unmarshal support bundle response: %w, body: %s", err, bundleResponseBody)
	}
	if bundleResp.BundleID == "" || bundleResp.Slug == "" {
		return fmt.Errorf("support bundle response missing bundleId or slug: %s", bundleResponseBody)
	}
	fmt.Printf("Support bundle uploaded: bundleId=%s, slug=%s\n", bundleResp.BundleID, bundleResp.Slug)

	// Upgrade the chart to enable minimal RBAC without status informers - this looks for a resource that has not been previously reported
	err = upgradeChartAndRestart(ctx, workdir, licenseID, channelSlug, []string{
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

	// Validate running images via vendor API (with default filtering)
	// 1. Call vendor API to get running images for this instance
	imagesSet, err := getRunningImages(ctx, appID, customerID, instanceAppID, tokenPlaintext)
	if err != nil {
		return fmt.Errorf("failed to get running images: %w", err)
	}

	// 2. Validate expected images (should only contain images from releaseImages list)
	required := []string{"proxy.replicated.com/proxy/replicated-sdk-e2e/index.docker.io/library/nginx:latest", "proxy.replicated.com/proxy/replicated-sdk-e2e/index.docker.io/library/nginx:alpine", strings.TrimSpace(sdkImage)}
	forbidden := []string{"docker.io/alpine/curl:latest"} // This is from replicated-ssl-test, should be filtered out
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
			return fmt.Errorf("running images contains forbidden entry: %s (should be filtered out)", img)
		}
	}

	fmt.Println("Default image filtering test passed - only releaseImages are reported")

	// Test reportAllImages functionality
	fmt.Println("Testing reportAllImages=true functionality...")

	// Create a ClusterRole and ClusterRoleBinding to allow the SDK to read pods across all namespaces
	fmt.Println("Creating ClusterRole for cross-namespace pod access...")
	clusterRoleYaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: replicated-cross-namespace-reader
rules:
- apiGroups:
  - ""
  resources:
  - "namespaces"
  verbs:
  - "list"
- apiGroups:
  - ""
  resources:
  - "pods"
  verbs:
  - "list"
  - "watch"`
	if err := os.WriteFile(filepath.Join(workdir, "clusterrole.yaml"), []byte(clusterRoleYaml), 0600); err != nil {
		return err
	}

	command = []string{"kubectl", "apply", "-f", filepath.Join(workdir, "clusterrole.yaml")}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to apply clusterrole: %w\n\nStdout: %s", err, out)
	}
	fmt.Println(out)

	// Create ClusterRoleBinding
	clusterRoleBindingYaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: replicated-cross-namespace-reader-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: replicated-cross-namespace-reader
subjects:
- kind: ServiceAccount
  name: replicated
  namespace: default`
	if err := os.WriteFile(filepath.Join(workdir, "clusterrolebinding.yaml"), []byte(clusterRoleBindingYaml), 0600); err != nil {
		return err
	}

	command = []string{"kubectl", "apply", "-f", filepath.Join(workdir, "clusterrolebinding.yaml")}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to apply clusterrolebinding: %w\n\nStdout: %s", err, out)
	}
	fmt.Println(out)

	// Deploy a test pod in kube-system namespace to verify cross-namespace image reporting
	fmt.Println("Deploying test pod in kube-system namespace...")
	kubeSystemPodYaml := `apiVersion: v1
kind: Pod
metadata:
  name: replicated-test-pod
  namespace: kube-system
  labels:
    app: replicated-test
spec:
  containers:
  - name: busybox
    image: docker.io/library/busybox:latest
    command: ["sleep", "500d"]`
	if err := os.WriteFile(filepath.Join(workdir, "kube-system-pod.yaml"), []byte(kubeSystemPodYaml), 0600); err != nil {
		return err
	}

	command = []string{"kubectl", "apply", "-f", filepath.Join(workdir, "kube-system-pod.yaml")}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to apply kube-system pod: %w\n\nStdout: %s", err, out)
	}
	fmt.Println(out)

	// Wait for the kube-system pod to be ready
	command = []string{"kubectl", "wait", "--for=condition=ready", "pod/replicated-test-pod", "-n", "kube-system", "--timeout=1m"}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to wait for kube-system pod to be ready: %w", err)
	}
	fmt.Println(out)

	// Upgrade the chart to enable reportAllImages
	err = upgradeChartAndRestart(ctx, workdir, licenseID, channelSlug, []string{
		"--set", "replicated.tlsCertSecretName=test-tls",
		"--set", "replicated.reportAllImages=true",
	})
	if err != nil {
		return fmt.Errorf("failed to upgrade chart enabling reportAllImages: %w", err)
	}

	err = waitForResourcesReady(ctx, newResourceNames, 30, 5*time.Second, tokenPlaintext, instanceAppID, distribution)
	if err != nil {
		return fmt.Errorf("failed to wait for resources to be ready: %w", err)
	}

	// Get running images again with retries, to allow server to refresh image reporting
	var allImagesSet map[string]struct{}
	// Now we expect both the alpine/curl from default namespace AND busybox from kube-system
	nowRequired := []string{
		"docker.io/alpine/curl:latest",     // from replicated-ssl-test in default namespace
		"docker.io/library/busybox:latest", // from kube-system namespace
	}
	maxAttempts := 5
	retryDelay := 5 * time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var getErr error
		allImagesSet, getErr = getRunningImages(ctx, appID, customerID, instanceAppID, tokenPlaintext)
		if getErr != nil {
			if attempt == maxAttempts {
				return fmt.Errorf("failed to get running images after enabling reportAllImages (after %d attempts): %w", maxAttempts, getErr)
			}
			fmt.Printf("attempt %d/%d: failed to get running images: %v\n", attempt, maxAttempts, getErr)
			if err := wait(ctx, retryDelay); err != nil {
				return err
			}
			continue
		}

		// Validate that previously excluded images are now present
		// The alpine/curl image from replicated-ssl-test should now be reported
		missing = []string{}
		for _, img := range nowRequired {
			if _, ok := allImagesSet[img]; !ok {
				missing = append(missing, img)
			}
		}
		if len(missing) == 0 {
			break
		}

		if attempt == maxAttempts {
			seen := make([]string, 0, len(allImagesSet))
			for k := range allImagesSet {
				seen = append(seen, k)
			}
			return fmt.Errorf("with reportAllImages=true, missing expected images after %d attempts: %v. Seen: %v", maxAttempts, missing, seen)
		}

		fmt.Printf("attempt %d/%d: still missing expected images with reportAllImages=true: %v (retrying)\n", attempt, maxAttempts, missing)
		if err := wait(ctx, retryDelay); err != nil {
			return err
		}
	}

	// Should still have the original required images
	for _, img := range required {
		if img == "" {
			continue
		}
		if _, ok := allImagesSet[img]; !ok {
			return fmt.Errorf("with reportAllImages=true, missing original required image: %s", img)
		}
	}

	fmt.Printf("reportAllImages test passed - found %d total images (vs %d with filtering)\n", len(allImagesSet), len(imagesSet))

	// Verify the support bundle uploaded earlier appears in the vendor API.
	// Placed at the end so analysis runs in parallel with other tests.
	fmt.Println("Verifying support bundle appears in vendor API...")
	expectedMetadata := map[string]string{"key1": "val1", "key2": "updated", "key3": "val3", "ticketID": "ISSUE-42"}
	err = waitForSupportBundle(ctx, tokenPlaintext, appID, bundleResp.BundleID, expectedMetadata, 10, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to verify support bundle in vendor API: %w", err)
	}
	fmt.Println("Support bundle upload test passed")

	// Test read-only mode functionality
	fmt.Println("Testing read-only mode functionality...")

	// Upgrade the chart to enable read-only mode with minimal RBAC
	err = upgradeChartAndRestart(ctx, workdir, licenseID, channelSlug, []string{
		"--set", "replicated.readOnlyMode=true",
		"--set", "replicated.minimalRBAC=true",
		"--set", "replicated.tlsCertSecretName=test-tls",
		"--set-json", `replicated.statusInformers=[]`,
	})
	if err != nil {
		return fmt.Errorf("failed to upgrade chart enabling read-only mode: %w", err)
	}

	// Verify the role does not contain create or update verbs for secrets
	command = []string{
		"kubectl", "describe", "role", "replicated-role",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to describe role in read-only mode: %w", err)
	}
	fmt.Println("Role permissions in read-only mode:")
	fmt.Println(out)

	if strings.Contains(out, "create") {
		return fmt.Errorf("role contains 'create' verb in read-only mode, expected only 'get'")
	}
	if strings.Contains(out, "update") {
		return fmt.Errorf("role contains 'update' verb in read-only mode, expected only 'get'")
	}

	// Verify the support-metadata secret was not created
	command = []string{
		"kubectl", "get", "secret", "replicated-support-metadata", "--ignore-not-found",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to check support-metadata secret: %w", err)
	}
	if strings.Contains(out, "replicated-support-metadata") {
		return fmt.Errorf("replicated-support-metadata secret exists in read-only mode, expected it to not be created")
	}
	fmt.Println("Verified: replicated-support-metadata secret not created in read-only mode")

	// Verify the support bundle secret was not created
	command = []string{
		"kubectl", "get", "secret", "replicated-supportbundle", "--ignore-not-found",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to check support bundle secret: %w", err)
	}
	if strings.Contains(out, "replicated-supportbundle") {
		return fmt.Errorf("support bundle secret exists in read-only mode, expected it to not be created")
	}
	fmt.Println("Verified: support bundle secret not created in read-only mode")

	// Verify the SDK is healthy by checking the license info endpoint
	command = []string{
		"kubectl", "exec", "deployment/replicated-ssl-test", "--",
		"curl", "-k", "-sS", "-w", "\n%{http_code}",
		"--retry", "3", "--retry-delay", "5", "--retry-all-errors",
		"https://replicated:3000/api/v1/license/info",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to get license info in read-only mode: %w\n\nStdout: %s", err, out)
	}
	outputLines = strings.Split(strings.TrimSpace(out), "\n")
	httpStatus = outputLines[len(outputLines)-1]
	if httpStatus != "200" {
		return fmt.Errorf("license info endpoint returned non-200 in read-only mode: %s", httpStatus)
	}
	fmt.Println("Verified: license info API works in read-only mode")

	// Verify instance tags endpoint returns 422 in read-only mode
	command = []string{
		"kubectl", "exec", "deployment/replicated-ssl-test", "--",
		"curl", "-k", "-sS", "-w", "\n%{http_code}",
		"--retry", "3", "--retry-delay", "5", "--retry-all-errors",
		"-X", "POST",
		"-H", "Content-Type: application/json",
		"-d", `{"data":{"test":"value"}}`,
		"https://replicated:3000/api/v1/app/instance-tags",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to test instance tags in read-only mode: %w\n\nStdout: %s", err, out)
	}
	outputLines = strings.Split(strings.TrimSpace(out), "\n")
	httpStatus = outputLines[len(outputLines)-1]
	if httpStatus != "422" {
		return fmt.Errorf("instance tags endpoint returned %s in read-only mode, expected 422", httpStatus)
	}
	fmt.Println("Verified: instance tags endpoint returns 422 in read-only mode")

	// Check SDK logs for any RBAC errors
	command = []string{
		"kubectl", "logs", "deployment/replicated", "--tail=50",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to get logs in read-only mode: %w", err)
	}
	fmt.Println("SDK logs in read-only mode:")
	fmt.Println(out)
	if strings.Contains(out, "forbidden") || strings.Contains(out, "Forbidden") {
		return fmt.Errorf("SDK logs contain RBAC forbidden errors in read-only mode")
	}

	fmt.Println("Read-only mode test passed")

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
			if err := wait(ctx, retryInterval); err != nil {
				return err
			}
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
		if err := wait(ctx, retryInterval); err != nil {
			return err
		}
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

// waitForCustomMetric waits for a custom metric to appear in the events API
func waitForCustomMetric(ctx context.Context, authToken string, instanceAppID string, metricKey string, expectedValue string, maxRetries int, retryInterval time.Duration) error {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		fmt.Printf("Attempt %d/%d: Checking for custom metric %s...\n", attempt, maxRetries, metricKey)

		events, err := getEvents(ctx, authToken, instanceAppID)
		if err != nil {
			if attempt == maxRetries {
				return fmt.Errorf("failed to get events after %d attempts: %w", maxRetries, err)
			}
			fmt.Printf("Failed to get events on attempt %d: %v\n", attempt, err)
			if err := wait(ctx, retryInterval); err != nil {
				return err
			}
			continue
		}

		// Look for custom metric events
		for _, event := range events {
			if event.IsCustom && event.FieldName == metricKey {
				fmt.Printf("Found custom metric %s with value %s (expected %s)\n", metricKey, event.NewValue, expectedValue)
				if event.NewValue == expectedValue {
					fmt.Printf("Custom metric %s verified after %d attempt(s)\n", metricKey, attempt)
					return nil
				}
			}
		}

		if attempt == maxRetries {
			return fmt.Errorf("custom metric %s not found or value mismatch after %d attempts", metricKey, maxRetries)
		}

		fmt.Printf("Custom metric %s not found yet, waiting %s before next attempt...\n", metricKey, retryInterval)
		if err := wait(ctx, retryInterval); err != nil {
			return err
		}
	}
	return fmt.Errorf("unreachable code")
}

// waitForSupportBundle waits for a support bundle to appear in the vendor API
// and validates that it contains the expected metadata.
func waitForSupportBundle(ctx context.Context, authToken string, appID string, bundleID string, expectedMetadata map[string]string, maxRetries int, retryInterval time.Duration) error {
	url := fmt.Sprintf("https://api.replicated.com/vendor/v3/supportbundle/%s", bundleID)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		fmt.Printf("Attempt %d/%d: Checking for support bundle %s...\n", attempt, maxRetries, bundleID)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", authToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			if attempt == maxRetries {
				return fmt.Errorf("failed to get support bundle after %d attempts: %w", maxRetries, err)
			}
			fmt.Printf("Failed to get support bundle on attempt %d: %v\n", attempt, err)
			if err := wait(ctx, retryInterval); err != nil {
				return err
			}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if attempt == maxRetries {
				return fmt.Errorf("vendor API returned status %d after %d attempts: %s", resp.StatusCode, maxRetries, string(body))
			}
			if err := wait(ctx, retryInterval); err != nil {
				return err
			}
			continue
		}

		if !strings.Contains(string(body), bundleID) {
			if attempt == maxRetries {
				return fmt.Errorf("support bundle %s not found in vendor API after %d attempts", bundleID, maxRetries)
			}
			fmt.Printf("Support bundle %s not found yet, waiting %s...\n", bundleID, retryInterval)
			if err := wait(ctx, retryInterval); err != nil {
				return err
			}
			continue
		}
		fmt.Printf("Support bundle %s found in vendor API after %d attempt(s)\n", bundleID, attempt)

		// Validate metadata if expected
		if len(expectedMetadata) > 0 {
			var response struct {
				Bundle struct {
					ID       string            `json:"id"`
					Metadata map[string]string `json:"metadata"`
				} `json:"bundle"`
			}
			if err := json.Unmarshal(body, &response); err != nil {
				return fmt.Errorf("failed to unmarshal support bundle response: %w, body: %s", err, string(body))
			}

			for k, v := range expectedMetadata {
				if response.Bundle.Metadata[k] != v {
					return fmt.Errorf("support bundle metadata mismatch: expected %s=%s, got %s=%s. Full metadata: %v", k, v, k, response.Bundle.Metadata[k], response.Bundle.Metadata)
				}
			}
			fmt.Printf("Support bundle metadata verified: %v\n", response.Bundle.Metadata)
		}

		return nil
	}
	return fmt.Errorf("unreachable code")
}

// testSupportBundleMetadata tests the POST and PATCH /api/v1/supportbundle/metadata endpoints
// and verifies the resulting secret contents.
func testSupportBundleMetadata(ctx context.Context, workdir string) error {
	fmt.Println("Testing support bundle metadata endpoints...")

	// POST metadata (overwrite all)
	postMetadataPayload := `{"data":{"key1":"val1","key2":"val2"}}`
	command := []string{
		"kubectl", "exec", "deployment/replicated-ssl-test", "--",
		"curl", "-k", "-sS", "-w", "\n%{http_code}",
		"--retry", "2", "--retry-delay", "5", "--retry-all-errors",
		"-X", "POST",
		"-H", "Content-Type: application/json",
		"-d", postMetadataPayload,
		"https://replicated:3000/api/v1/supportbundle/metadata",
	}
	out, err := run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to POST support bundle metadata: %w\n\nStdout: %s", err, out)
	}
	outputLines := strings.Split(strings.TrimSpace(out), "\n")
	httpStatus := outputLines[len(outputLines)-1]
	if httpStatus != "200" {
		return fmt.Errorf("POST support bundle metadata returned non-200 status: %s, response: %s", httpStatus, out)
	}
	fmt.Println("POST /api/v1/supportbundle/metadata returned 200")

	// PATCH metadata (merge keys)
	patchMetadataPayload := `{"data":{"key2":"updated","key3":"val3"}}`
	command = []string{
		"kubectl", "exec", "deployment/replicated-ssl-test", "--",
		"curl", "-k", "-sS", "-w", "\n%{http_code}",
		"--retry", "2", "--retry-delay", "5", "--retry-all-errors",
		"-X", "PATCH",
		"-H", "Content-Type: application/json",
		"-d", patchMetadataPayload,
		"https://replicated:3000/api/v1/supportbundle/metadata",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to PATCH support bundle metadata: %w\n\nStdout: %s", err, out)
	}
	outputLines = strings.Split(strings.TrimSpace(out), "\n")
	httpStatus = outputLines[len(outputLines)-1]
	if httpStatus != "200" {
		return fmt.Errorf("PATCH support bundle metadata returned non-200 status: %s, response: %s", httpStatus, out)
	}
	fmt.Println("PATCH /api/v1/supportbundle/metadata returned 200")

	// Read the secret and verify contents: each key is a top-level data key
	// Use -o json to get the full secret and decode all data keys
	command = []string{
		"kubectl", "get", "secret", "replicated-support-metadata",
		"-o", "jsonpath={.data}",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to read support bundle metadata secret: %w\n\nStdout: %s", err, out)
	}

	// jsonpath {.data} returns a map of base64-encoded values like {"key1":"dmFsMQ==","key2":"..."}
	var encodedData map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &encodedData); err != nil {
		return fmt.Errorf("failed to unmarshal secret data: %w, raw: %s", err, out)
	}

	metadataMap := make(map[string]string, len(encodedData))
	for k, v := range encodedData {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return fmt.Errorf("failed to base64 decode key %s: %w", k, err)
		}
		metadataMap[k] = string(decoded)
	}

	expectedMetadata := map[string]string{"key1": "val1", "key2": "updated", "key3": "val3"}
	for k, v := range expectedMetadata {
		if metadataMap[k] != v {
			return fmt.Errorf("support bundle metadata mismatch: expected %s=%s, got %s=%s. Full metadata: %v", k, v, k, metadataMap[k], metadataMap)
		}
	}
	if len(metadataMap) != len(expectedMetadata) {
		return fmt.Errorf("support bundle metadata has unexpected keys: expected %v, got %v", expectedMetadata, metadataMap)
	}
	fmt.Printf("Support bundle metadata verified: %v\n", metadataMap)

	return nil
}

// upgradeChartAndRestart upgrades the helm chart with the provided arguments and restarts deployments
func upgradeChartAndRestart(
	ctx context.Context,
	workdir string,
	licenseID string,
	channelSlug string,
	helmArgs []string,
) error {
	// Helm upgrade
	upgradeCmd := []string{"helm", "upgrade", "test-chart", fmt.Sprintf("oci://registry.replicated.com/replicated-sdk-e2e/%s/test-chart", channelSlug), "--version", "0.1.0"}
	upgradeCmd = append(upgradeCmd, helmArgs...)

	command := upgradeCmd

	out, err := run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to upgrade chart: %w", err)
	}
	fmt.Println(out)

	// Restart replicated deployment
	command = []string{
		"kubectl", "rollout", "restart", "deploy/replicated",
	}
	out, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to restart replicated deployment: %w", err)
	}
	fmt.Println(out)

	// Wait for replicated deployment to be ready
	command = []string{
		"kubectl", "rollout", "status",
		"deploy/replicated",
		"--timeout=5m",
	}

	out, err = run(ctx, command...)
	if err != nil {
		fmt.Printf("failed to wait for replicated deployment to rollout: %v\n", err)

		// A pod may still be pulling or creating its container, in which case
		// kubectl logs fails with a misleading BadRequest. Capture pod and
		// deployment state instead so the actual startup failure is visible.
		for _, args := range [][]string{
			{"kubectl", "get", "pods", "-o", "wide"},
			{"kubectl", "describe", "deployment", "replicated"},
			{"kubectl", "describe", "pods", "-l", "app.kubernetes.io/name=replicated"},
		} {
			command = args
			diagnostic, diagnosticErr := run(ctx, command...)
			if diagnosticErr != nil {
				fmt.Printf("failed to collect diagnostic output for %q: %v\n", args, diagnosticErr)
				continue
			}
			fmt.Printf("Diagnostic output for %q:\n%s\n", args, diagnostic)
		}

		return fmt.Errorf("failed to wait for replicated deployment to rollout: %w", err)
	}
	fmt.Println(out)

	// Restart test-chart deployment
	command = []string{
		"kubectl", "rollout", "restart", "deploy/test-chart",
	}
	_, err = run(ctx, command...)
	if err != nil {
		return fmt.Errorf("failed to restart test-chart deployment: %w", err)
	}

	return nil
}
