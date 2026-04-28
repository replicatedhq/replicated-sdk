// Package startupstate exposes a shared, concurrency-safe enum that tracks the
// lifecycle of the SDK's bootstrap process. It is consumed by the apiserver
// (which transitions the state) and by the /healthz handler (which reports the
// state to readiness probes).
//
// The package lives outside both apiserver and handlers to avoid an import
// cycle between them.
package startupstate

import "sync/atomic"

// State enumerates the bootstrap lifecycle phases.
type State int32

const (
	// Starting indicates the API server is up but bootstrap has not yet
	// completed (or, with requireUpstreamOnStartup, has not yet completed
	// the full critical+background path).
	Starting State = iota
	// Ready indicates the SDK is ready to serve requests. With the default
	// configuration this fires after bootstrapCritical succeeds or after
	// the readiness timeout elapses, whichever comes first.
	Ready
	// Failed indicates bootstrap encountered a permanent (non-retryable)
	// error. /healthz will surface 503 in this state and the process will
	// typically exit shortly after.
	Failed
)

// String returns a stable, lower-case representation suitable for logs and
// JSON payloads.
func (s State) String() string {
	switch s {
	case Starting:
		return "starting"
	case Ready:
		return "ready"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

// Tracker holds the current bootstrap state. The zero value is usable and
// reports Starting.
type Tracker struct {
	state atomic.Int32
}

// New returns a Tracker initialized to Starting.
func New() *Tracker {
	return &Tracker{}
}

// Get returns the current state.
func (t *Tracker) Get() State {
	return State(t.state.Load())
}

// Set unconditionally writes the supplied state.
func (t *Tracker) Set(s State) {
	t.state.Store(int32(s))
}

// MarkReady transitions the tracker to Ready unless it is already Failed.
// Returns true if the call caused the transition.
//
// Failed is sticky: once a permanent bootstrap error has been recorded we
// never silently flip back to Ready.
func (t *Tracker) MarkReady() bool {
	for {
		current := State(t.state.Load())
		if current == Failed {
			return false
		}
		if current == Ready {
			return false
		}
		if t.state.CompareAndSwap(int32(current), int32(Ready)) {
			return true
		}
	}
}

// MarkFailed transitions the tracker to Failed. Failed is terminal.
func (t *Tracker) MarkFailed() {
	t.state.Store(int32(Failed))
}

// IsReady is a convenience that returns true iff the current state is Ready.
func (t *Tracker) IsReady() bool {
	return t.Get() == Ready
}
