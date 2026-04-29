package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsAirgap_TruthTable defends the doc claim in env.go that the
// airgapOverride and DISABLE_OUTBOUND_CONNECTIONS env var are independent
// inputs to the same boolean — IsAirgap returns true if either or both
// is set, and there is no precedence between them. A future contributor
// who introduces a precedence rule (e.g. "env wins" or "override wins")
// would silently change a documented invariant; this table makes the
// invariant testable.
func TestIsAirgap_TruthTable(t *testing.T) {
	cases := []struct {
		name     string
		envValue string
		override bool
		want     bool
	}{
		{"both off", "", false, false},
		{"override only", "", true, true},
		{"env only", "true", false, true},
		{"both on", "true", true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DISABLE_OUTBOUND_CONNECTIONS", tc.envValue)
			t.Cleanup(ResetAirgapOverride)
			SetAirgapOverride(tc.override)

			require.Equal(t, tc.want, IsAirgap())
		})
	}
}

// TestIsAirgap_EnvVarRequiresExactTrue locks in the strict-string
// contract of the env-var path: only the literal "true" enables airgap.
// "1", "TRUE", "yes" and friends do NOT. Without this test, a future
// "be more lenient with env values" change would silently broaden the
// airgap surface, which is the wrong direction for a flag that disables
// an important class of behavior.
func TestIsAirgap_EnvVarRequiresExactTrue(t *testing.T) {
	t.Cleanup(ResetAirgapOverride)
	ResetAirgapOverride()

	for _, v := range []string{"", "1", "yes", "TRUE", "True", "false", "0"} {
		t.Run("env="+v, func(t *testing.T) {
			t.Setenv("DISABLE_OUTBOUND_CONNECTIONS", v)
			require.False(t, IsAirgap(),
				"env=%q must not enable airgap; only the exact string \"true\" does", v)
		})
	}
}

// TestSetAirgapOverride_Idempotent verifies that flipping the override
// to true repeatedly is safe. bootstrapCritical can be re-invoked by
// the orchestrator's retry loop and applyDevOfflineGuard re-runs each
// time, so SetAirgapOverride(true) is called more than once in normal
// operation. The atomic.Bool semantics make this trivially correct, but
// codifying the expectation prevents a future "guard against double-set"
// refactor from introducing accidental panics or state-machine logic.
func TestSetAirgapOverride_Idempotent(t *testing.T) {
	t.Setenv("DISABLE_OUTBOUND_CONNECTIONS", "")
	t.Cleanup(ResetAirgapOverride)
	ResetAirgapOverride()
	require.False(t, IsAirgap(), "precondition: override and env both off")

	SetAirgapOverride(true)
	SetAirgapOverride(true)
	SetAirgapOverride(true)
	require.True(t, IsAirgap())

	ResetAirgapOverride()
	require.False(t, IsAirgap())
}
