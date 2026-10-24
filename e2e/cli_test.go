//go:build unix

package e2e

import (
	"context"
	"testing"
	"time"
)

func TestCLIOutputAndFailure(t *testing.T) {
	out, err := run(t.Context(), "sh", "-c", `echo "$1"; echo diagnostic >&2; exit 7`, "sh", "literal $() ' text")
	if out != "literal $() ' text\n" || err == nil {
		t.Fatalf("stdout=%q error=%v", out, err)
	}
}
func TestCLICancellationStopsChildren(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := run(ctx, "sh", "-c", "sleep 30 & wait")
	if err == nil {
		t.Fatal("cancelled command succeeded")
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("command subprocesses survived cancellation")
	}
}
func TestWaitRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := wait(ctx, time.Hour); err != context.Canceled {
		t.Fatalf("wait returned %v", err)
	}
}
