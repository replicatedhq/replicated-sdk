package handlers

import (
	"net/http"
	"sync/atomic"

	"github.com/replicatedhq/replicated-sdk/pkg/buildversion"
	"github.com/replicatedhq/replicated-sdk/pkg/startupstate"
)

type HealthzResponse struct {
	Version string `json:"version"`
	Status  string `json:"status,omitempty"`
}

// startupTracker is a package-level pointer to the bootstrap-state tracker.
// apiserver.Start() installs the tracker via SetStartupState before serving.
// We use atomic.Pointer so reads from /healthz are safe regardless of when
// the tracker is installed.
var startupTracker atomic.Pointer[startupstate.Tracker]

// SetStartupState installs the bootstrap-state tracker for /healthz to consult.
// Pass nil to clear (used by tests).
func SetStartupState(t *startupstate.Tracker) {
	startupTracker.Store(t)
}

func Healthz(w http.ResponseWriter, r *http.Request) {
	t := startupTracker.Load()
	// Fail closed: if no tracker is installed, treat the SDK as still
	// starting. Production wiring always installs a tracker before the
	// listener accepts traffic, so reaching this branch indicates either a
	// misconfigured caller or a regression — either way, 503 is the safe
	// answer.
	state := startupstate.Starting
	if t != nil {
		state = t.Get()
	}

	resp := HealthzResponse{
		Version: buildversion.Version(),
		Status:  state.String(),
	}

	switch state {
	case startupstate.Ready:
		JSON(w, http.StatusOK, resp)
	default:
		JSON(w, http.StatusServiceUnavailable, resp)
	}
}
