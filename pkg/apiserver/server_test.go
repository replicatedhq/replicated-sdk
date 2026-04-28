package apiserver

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/replicatedhq/replicated-sdk/pkg/startupstate"
	"github.com/stretchr/testify/require"
)

// fastDeps returns bootstrapDeps with short timing so tests run quickly.
// Tests pass critical and background as parameters because each scenario
// needs its own behavior.
func fastDeps(critical, background func(APIServerParams) error) bootstrapDeps {
	return bootstrapDeps{
		critical:      critical,
		background:    background,
		deadline:      50 * time.Millisecond,
		retryInterval: 5 * time.Millisecond,
	}
}

func TestRunBootstrapResilient_CriticalTimesOutThenSucceeds(t *testing.T) {
	state := startupstate.New()
	var bgRan atomic.Bool
	criticalCalls := 0
	criticalSlow := func(APIServerParams) error {
		criticalCalls++
		// First two attempts: pretend the upstream is slow / failing.
		// Third attempt: succeed. The first two attempts together must
		// outlast the deadline so we exercise the timeout-then-success
		// path.
		if criticalCalls < 3 {
			time.Sleep(40 * time.Millisecond)
			return errors.New("transient")
		}
		return nil
	}

	deps := fastDeps(
		criticalSlow,
		func(APIServerParams) error { bgRan.Store(true); return nil },
	)
	deps.deadline = 30 * time.Millisecond

	start := time.Now()
	require.NoError(t, runBootstrapResilient(APIServerParams{}, state, deps))
	elapsed := time.Since(start)

	require.True(t, state.IsReady(), "expected state Ready, got %s", state.Get())
	require.True(t, bgRan.Load(), "expected background to run once critical eventually succeeded")
	require.GreaterOrEqual(t, elapsed, deps.deadline, "expected to wait at least the deadline")
}

func TestRunBootstrapResilient_CriticalPermanentError_FailsFast(t *testing.T) {
	state := startupstate.New()
	var bgRan atomic.Bool

	deps := fastDeps(
		func(APIServerParams) error {
			return backoff.Permanent(errors.New("license is expired"))
		},
		func(APIServerParams) error { bgRan.Store(true); return nil },
	)

	err := runBootstrapResilient(APIServerParams{}, state, deps)
	require.Error(t, err)
	require.Equal(t, startupstate.Failed, state.Get())
	require.False(t, bgRan.Load(), "background must not run after a permanent critical failure")
}

func TestRunBootstrapResilient_BackgroundFailureDoesNotAffectReady(t *testing.T) {
	state := startupstate.New()

	deps := fastDeps(
		func(APIServerParams) error { return nil },
		func(APIServerParams) error { return errors.New("upstream sync failed") },
	)

	require.NoError(t, runBootstrapResilient(APIServerParams{}, state, deps), "background failures should not bubble up")
	require.True(t, state.IsReady(), "expected state Ready despite background failure")
}

func TestRunBootstrapStrict_BlocksReadyUntilFullBootstrapSucceeds(t *testing.T) {
	state := startupstate.New()
	var phase atomic.Int32

	deps := fastDeps(
		func(APIServerParams) error {
			phase.Store(1)
			return nil
		},
		func(APIServerParams) error {
			if !state.IsReady() && phase.Load() == 1 {
				// We deliberately observe state HERE — strict mode must
				// not have flipped Ready before background returns.
				phase.Store(2)
			}
			return nil
		},
	)
	params := APIServerParams{RequireUpstreamOnStartup: true}

	require.NoError(t, runBootstrapWithDeps(params, state, deps))
	require.Equal(t, int32(2), phase.Load(), "background did not observe pre-Ready state")
	require.True(t, state.IsReady(), "expected state Ready after strict bootstrap")
}

func TestRunBootstrapStrict_BackgroundFailure_BlocksReady(t *testing.T) {
	state := startupstate.New()

	deps := fastDeps(
		func(APIServerParams) error { return nil },
		func(APIServerParams) error {
			// Permanent so the retry loop gives up quickly.
			return backoff.Permanent(errors.New("upstream sync permanently failed"))
		},
	)
	params := APIServerParams{RequireUpstreamOnStartup: true}

	err := runBootstrapWithDeps(params, state, deps)
	require.Error(t, err, "expected an error when strict-mode background fails")
	require.Equal(t, startupstate.Failed, state.Get())
}
