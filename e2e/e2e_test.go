//go:build unix

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// TestSDK retains the SDK's existing assertions. Cluster lifecycle, credentials,
// kubeconfig and Helm authentication are prepared by the E2E workflow.
func TestSDK(t *testing.T) {
	if os.Getenv("SDK_E2E") != "1" {
		t.Skip("run make test-e2e with build-info.json and E2E_DISTRIBUTION/E2E_VERSION")
	}
	for _, name := range []string{"REPLICATED_API_TOKEN", "KUBECONFIG", "E2E_BUILD_INFO", "E2E_DISTRIBUTION"} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s is required", name)
		}
	}
	var info struct {
		AppID           string `json:"appID"`
		CustomerID      string `json:"customerID"`
		ImageRegistry   string `json:"imageRegistry"`
		ImageRepository string `json:"imageRepository"`
		ImageTag        string `json:"imageTag"`
		ImageDigest     string `json:"imageDigest"`
		LicenseID       string `json:"licenseID"`
		ChannelSlug     string `json:"channelSlug"`
	}
	data, err := os.ReadFile(os.Getenv("E2E_BUILD_INFO"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	if info.AppID == "" || info.CustomerID == "" || info.LicenseID == "" || info.ChannelSlug == "" || info.ImageDigest == "" {
		t.Fatal("build metadata is incomplete")
	}
	workdir := t.TempDir()
	t.Setenv("E2E_ARTIFACT_DIR", workdir)
	image := fmt.Sprintf("%s/%s:%s", info.ImageRegistry, info.ImageRepository, info.ImageTag)
	ctx, stop := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	t.Cleanup(stop)
	if err := e2e(ctx, workdir, info.AppID, info.CustomerID, image, info.ImageDigest, info.LicenseID, os.Getenv("E2E_DISTRIBUTION"), os.Getenv("E2E_VERSION"), info.ChannelSlug); err != nil {
		t.Fatal(err)
	}
}

// run invokes installed CLI tools directly and preserves stdout for JSON parsing.
func run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	// Cancel a CLI's subprocesses too (notably support-bundle and port-forward).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), err
	}
	return stdout.String(), nil
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
