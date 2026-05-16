package util

import (
	"os"
	"sync/atomic"
)

// airgapOverride is a runtime opt-in that forces IsAirgap() to report
// true even when DISABLE_OUTBOUND_CONNECTIONS is unset. It exists so the
// devOffline workflow (a dev-license-only convenience for working
// offline without configuring full air-gap support) can reuse the
// existing !IsAirgap() gates throughout the codebase rather than
// introducing a parallel set of checks.
//
// The override is set once during bootstrapCritical when params.DevOffline
// is true and the loaded license is a dev license. Tests that flip the
// override must call ResetAirgapOverride in t.Cleanup so state does not
// leak between tests.
//
// The override and DISABLE_OUTBOUND_CONNECTIONS are independent inputs
// to the same boolean: if either (or both) is set, IsAirgap() returns
// true and behavior is identical to either alone. There is no precedence
// to reason about because there is nothing to disagree on.
var airgapOverride atomic.Bool

// SetAirgapOverride forces IsAirgap to return true regardless of the
// DISABLE_OUTBOUND_CONNECTIONS env var. Call once during bootstrap.
func SetAirgapOverride(on bool) {
	airgapOverride.Store(on)
}

// ResetAirgapOverride clears the override. Intended for test cleanup
// (t.Cleanup); production callers do not unset the override.
func ResetAirgapOverride() {
	airgapOverride.Store(false)
}

func IsAirgap() bool {
	if airgapOverride.Load() {
		return true
	}
	return os.Getenv("DISABLE_OUTBOUND_CONNECTIONS") == "true"
}

func IsDevEnv() bool {
	return os.Getenv("REPLICATED_ENV") == "dev"
}
