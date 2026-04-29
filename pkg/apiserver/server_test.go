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
		bgMaxRetries:  100, // generous; individual tests override when they want to exercise the cap
	}
}

func TestRunBootstrap_CriticalTimesOutThenSucceeds(t *testing.T) {
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
	require.NoError(t, runBootstrapWithDeps(APIServerParams{}, state, deps))
	elapsed := time.Since(start)

	require.True(t, state.IsReady(), "expected state Ready, got %s", state.Get())
	require.True(t, bgRan.Load(), "expected background to run once critical eventually succeeded")
	require.GreaterOrEqual(t, elapsed, deps.deadline, "expected to wait at least the deadline")
}

func TestRunBootstrap_CriticalPermanentError_FailsFast(t *testing.T) {
	state := startupstate.New()
	var bgRan atomic.Bool

	deps := fastDeps(
		func(APIServerParams) error {
			return backoff.Permanent(errors.New("license is expired"))
		},
		func(APIServerParams) error { bgRan.Store(true); return nil },
	)

	err := runBootstrapWithDeps(APIServerParams{}, state, deps)
	require.Error(t, err)
	require.Equal(t, startupstate.Failed, state.Get())
	require.False(t, bgRan.Load(), "background must not run after a permanent critical failure")
}

func TestRunBootstrap_BackgroundPermanentFailureDoesNotAffectReady(t *testing.T) {
	state := startupstate.New()

	deps := fastDeps(
		func(APIServerParams) error { return nil },
		func(APIServerParams) error {
			// Permanent so the retry loop gives up quickly — otherwise
			// this test would block forever, since the loop must keep
			// retrying transient background errors (heartbeat startup,
			// upstream sync) for the pod's entire lifetime.
			return backoff.Permanent(errors.New("upstream sync permanently failed"))
		},
	)

	require.NoError(t, runBootstrapWithDeps(APIServerParams{}, state, deps), "background failures should not bubble up")
	require.True(t, state.IsReady(), "expected state Ready despite background failure")
}

// TestRunBootstrap_BackgroundRetriesUntilSuccess verifies that the
// orchestrator keeps retrying bootstrapBackground after transient
// failures rather than swallowing them. Without this retry, a momentary
// hiccup on the first background attempt (e.g. heartbeat cron init,
// upstream license sync) would silently disable the heartbeat job and
// every subsequent license refresh for the entire pod lifetime.
func TestRunBootstrap_BackgroundRetriesUntilSuccess(t *testing.T) {
	state := startupstate.New()
	var bgCalls atomic.Int32

	deps := fastDeps(
		func(APIServerParams) error { return nil },
		func(APIServerParams) error {
			n := bgCalls.Add(1)
			if n < 3 {
				return errors.New("transient")
			}
			return nil
		},
	)

	require.NoError(t, runBootstrapWithDeps(APIServerParams{}, state, deps))
	require.True(t, state.IsReady())
	require.GreaterOrEqual(t, bgCalls.Load(), int32(3), "must retry transient background failures")
}

// TestRunBootstrap_BackgroundGivesUpAfterMaxRetries verifies the retry
// loop terminates in finite time when bootstrapBackground returns
// transient errors that never resolve. bootstrapBackground in production
// returns stderrors.Join(errs...), which strips backoff.Permanent
// wrapping from inner steps; without an explicit max-retries cap, the
// loop would log-spam every retryInterval forever and the give-up
// branch in runBootstrapWithDeps would be dead code. This test exercises
// the cap.
func TestRunBootstrap_BackgroundGivesUpAfterMaxRetries(t *testing.T) {
	state := startupstate.New()
	var bgCalls atomic.Int32

	deps := fastDeps(
		func(APIServerParams) error { return nil },
		func(APIServerParams) error {
			bgCalls.Add(1)
			// Plain transient error — never wrapped in
			// backoff.Permanent. Without WithMaxRetries this
			// would retry forever.
			return errors.New("persistent transient failure")
		},
	)
	deps.bgMaxRetries = 3

	require.NoError(t, runBootstrapWithDeps(APIServerParams{}, state, deps),
		"background give-up must not bubble up — pod stays Ready")
	require.True(t, state.IsReady())
	// WithMaxRetries(b, n) allows n retries AFTER the initial attempt,
	// so total calls is n+1.
	require.Equal(t, int32(deps.bgMaxRetries+1), bgCalls.Load(),
		"loop must terminate after bgMaxRetries+1 total attempts, not retry forever")
}

// TestRunBootstrap_CriticalFailsAfterDeadline_StaysReadyButSkipsBackground
// pins the deliberate "false-readiness over Ready→crash flap" trade-off
// in runBootstrapWithDeps: when bootstrapCritical outlasts the readiness
// timer (so the pod has already been marked Ready) and then ultimately
// returns a permanent error, the pod stays Ready and the background
// phase is intentionally skipped. The orchestrator returns nil because
// flipping back to Failed → log.Fatalf would produce a
// Ready→crash→restart→Ready→crash loop where every cycle briefly exposes
// an under-initialized store to traffic.
//
// A future change that "fixes" this perceived false-Ready by transitioning
// to Failed, by running background anyway, or by returning a non-nil
// error from runBootstrap would silently alter a documented contract.
// This test should fail in that case so the change is conscious.
func TestRunBootstrap_CriticalFailsAfterDeadline_StaysReadyButSkipsBackground(t *testing.T) {
	state := startupstate.New()
	var bgRan atomic.Bool
	var criticalCalls atomic.Int32

	criticalSlowThenPermanent := func(APIServerParams) error {
		n := criticalCalls.Add(1)
		if n == 1 {
			// First attempt: sleep past the deadline so the timer
			// fires and the orchestrator marks Ready before we
			// return. The returned error must be transient so
			// RetryNotify schedules a second attempt.
			time.Sleep(40 * time.Millisecond)
			return errors.New("transient first failure outlasting deadline")
		}
		// Second attempt: permanent so RetryNotify gives up and the
		// orchestrator observes a non-nil criticalErr post-deadline.
		return backoff.Permanent(errors.New("license is unrecoverable"))
	}

	deps := fastDeps(
		criticalSlowThenPermanent,
		func(APIServerParams) error { bgRan.Store(true); return nil },
	)
	deps.deadline = 30 * time.Millisecond

	require.NoError(t, runBootstrapWithDeps(APIServerParams{}, state, deps),
		"post-deadline critical failure must not bubble up — pod stays Ready")
	require.True(t, state.IsReady(),
		"pod must stay Ready after the Ready→fail transition; flipping to Failed would cause a Ready→crash flap")
	require.False(t, bgRan.Load(),
		"background must be skipped when critical permanently fails after the readiness deadline")
}
