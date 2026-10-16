package licensestate

import (
	"sync"
)

var configured struct {
	sync.RWMutex
	manager *Manager
}

// Configure installs the process-local handle to the SDK-owned state Secret.
// The Secret remains authoritative; this is only a convenience for refresh
// paths that already received a signed artifact from Portal.
func Configure(manager *Manager) {
	configured.Lock()
	defer configured.Unlock()
	configured.manager = manager
}

func Current() *Manager {
	configured.RLock()
	defer configured.RUnlock()
	return configured.manager
}

// SDKManaged reports whether this installation's license is owned by the SDK
// rather than rendered by Helm. It is true exactly when the chart enabled
// sdkManagedLicense, because that is what causes a manager to be configured.
//
// Callers use it to stay out of the legacy license flow, which authenticates
// with a mutable license ID and knows nothing about installation generations.
func SDKManaged() bool {
	return Current() != nil
}
