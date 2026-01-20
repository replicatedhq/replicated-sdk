package apiserver

import (
	"context"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/leader"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
)

// NOTE: Leader election uses client-go's ReleaseOnCancel behavior, which runs on the leader-election goroutine.
// We must not allow process exit immediately after ctx cancellation, otherwise the goroutine may not complete
// the lease release and other pods may have to wait out LeaseDuration before acquiring leadership.

var leaderElectionWg sync.WaitGroup

func startLeaderElectionAsync(ctx context.Context, elector leader.LeaderElector) {
	leaderElectionWg.Add(1)
	go func() {
		defer leaderElectionWg.Done()
		if err := elector.Start(ctx); err != nil {
			// Start() normally returns nil when ctx is cancelled; any error is worth logging.
			logger.Error(pkgerrors.Wrap(err, "leader election stopped"))
		}
	}()
}

func waitForLeaderElectionStop(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		leaderElectionWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(timeout):
		logger.Warnf("Timed out waiting for leader election to stop")
		return
	}
}
