package leader

import (
	"context"
	"time"
)

// LeaderElector defines the interface for leader election operations
type LeaderElector interface {
	// IsLeader returns true if this instance is currently the leader
	IsLeader() bool
	// GetIdentity returns the unique identity of this instance
	GetIdentity() string
	// Start begins the leader election process and blocks until context is cancelled
	Start(ctx context.Context) error
	// WaitForLeader blocks until a leader is elected (either this instance or another)
	// Returns true if this instance is the leader, false if another instance is the leader
	WaitForLeader(ctx context.Context) (bool, error)
}

// Config contains the configuration for leader election
type Config struct {
	// LeaseName is the name of the Lease resource
	LeaseName string
	// LeaseNamespace is the namespace where the Lease resource resides
	LeaseNamespace string
	// LeaseDuration is the duration that non-leader candidates will wait to force acquire leadership
	LeaseDuration time.Duration
	// RenewDeadline is the duration that the acting leader will retry refreshing leadership before giving up
	RenewDeadline time.Duration
	// RetryPeriod is the duration the LeaderElector clients should wait between tries of actions
	RetryPeriod time.Duration
	// OnStartedLeading is called when this instance becomes the leader
	OnStartedLeading func(ctx context.Context)
	// OnStoppedLeading is called when this instance loses leadership
	OnStoppedLeading func()
	// OnNewLeader is called when a new leader is elected (including this instance)
	OnNewLeader func(identity string)
}
