package leader

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// kubernetesLeaderElector implements the LeaderElector interface using Kubernetes leases
type kubernetesLeaderElector struct {
	identity      string
	isLeader      bool
	mu            sync.RWMutex
	config        Config
	leaderElector *leaderelection.LeaderElector
}

// NewLeaderElector creates a new leader elector using Kubernetes leases
func NewLeaderElector(clientset kubernetes.Interface, config Config) (LeaderElector, error) {
	// Get pod identity from environment variable
	identity := os.Getenv("REPLICATED_POD_NAME")
	if identity == "" {
		return nil, errors.New("REPLICATED_POD_NAME environment variable is not set")
	}

	// Validate config
	if config.LeaseName == "" {
		return nil, errors.New("lease name is required")
	}
	if config.LeaseNamespace == "" {
		return nil, errors.New("lease namespace is required")
	}
	if config.LeaseDuration == 0 {
		config.LeaseDuration = 15 * time.Second
	}
	if config.RenewDeadline == 0 {
		config.RenewDeadline = 10 * time.Second
	}
	if config.RetryPeriod == 0 {
		config.RetryPeriod = 2 * time.Second
	}

	kle := &kubernetesLeaderElector{
		identity: identity,
		isLeader: false,
		config:   config,
	}

	// Create lease lock
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      config.LeaseName,
			Namespace: config.LeaseNamespace,
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: identity,
		},
	}

	// Create leader elector
	leConfig := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: config.LeaseDuration,
		RenewDeadline: config.RenewDeadline,
		RetryPeriod:   config.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				kle.setLeader(true)
				if config.OnStartedLeading != nil {
					config.OnStartedLeading(ctx)
				}
			},
			OnStoppedLeading: func() {
				kle.setLeader(false)
				if config.OnStoppedLeading != nil {
					config.OnStoppedLeading()
				}
			},
			OnNewLeader: func(identity string) {
				if identity == kle.identity {
					kle.setLeader(true)
				} else {
					kle.setLeader(false)
				}
				if config.OnNewLeader != nil {
					config.OnNewLeader(identity)
				}
			},
		},
	}

	le, err := leaderelection.NewLeaderElector(leConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create leader elector")
	}

	kle.leaderElector = le
	return kle, nil
}

// IsLeader returns true if this instance is currently the leader
func (k *kubernetesLeaderElector) IsLeader() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.isLeader
}

// GetIdentity returns the unique identity of this instance
func (k *kubernetesLeaderElector) GetIdentity() string {
	return k.identity
}

// Start begins the leader election process and blocks until context is cancelled
func (k *kubernetesLeaderElector) Start(ctx context.Context) error {
	if k.leaderElector == nil {
		return errors.New("leader elector not initialized")
	}

	// Run blocks until context is cancelled
	k.leaderElector.Run(ctx)
	return nil
}

// setLeader updates the leader status in a thread-safe manner
func (k *kubernetesLeaderElector) setLeader(isLeader bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.isLeader = isLeader
}

// String returns a human-readable representation of the leader elector
func (k *kubernetesLeaderElector) String() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return fmt.Sprintf("LeaderElector{identity=%s, isLeader=%v}", k.identity, k.isLeader)
}
