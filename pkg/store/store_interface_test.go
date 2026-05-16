package store

import (
	"sync"
	"sync/atomic"
	"testing"

	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/stretchr/testify/require"
)

// TestSetStore_GetStore_ConcurrentSwapAndRead verifies that the
// package-level store pointer can be swapped while readers are calling
// GetStore() in parallel. Without atomic.Pointer guarding `storePtr`,
// `go test -race` would flag the read at GetStore against the write at
// SetStore — and even without -race, an unsynchronized interface read
// could observe a torn (itab, data) pair on platforms where two-word
// loads aren't atomic. This is the production scenario the listener +
// background-bootstrap refactor introduced: handlers calling GetStore
// race with bootstrapCritical's InitInMemory → SetStore.
func TestSetStore_GetStore_ConcurrentSwapAndRead(t *testing.T) {
	t.Cleanup(func() { SetStore(nil) })

	const goroutines = 64
	const iterations = 1000

	a := &InMemoryStore{}
	b := &InMemoryStore{}

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	var reads atomic.Uint64
	for i := 0; i < goroutines; i++ {
		go func(swapToA bool) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if swapToA {
					SetStore(a)
				} else {
					SetStore(b)
				}
			}
		}(i%2 == 0)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				s := GetStore()
				require.NotNil(t, s, "GetStore must never return nil")
				reads.Add(1)
			}
		}()
	}

	wg.Wait()
	require.Equal(t, uint64(goroutines*iterations), reads.Load())
}

func TestSetStore_NilClearsStore(t *testing.T) {
	t.Cleanup(func() { SetStore(nil) })

	s := &InMemoryStore{}
	SetStore(s)
	require.Same(t, s, GetStore(), "SetStore should install the exact provided store")

	SetStore(nil)
	got := GetStore()
	require.NotNil(t, got, "GetStore must return a non-nil fallback after SetStore(nil)")
	require.NotSame(t, s, got, "SetStore(nil) should clear; subsequent GetStore must not return the previously installed store")
}

// TestGetStore_FallbackIsSharedAcrossCalls locks in the contract that
// when no store has been installed (the resilient-mode-timeout window
// where the pod is Ready but bootstrapCritical hasn't run InitInMemory
// yet), consecutive GetStore calls return the SAME fallback instance —
// not a fresh empty one each time. Otherwise a handler doing
// `s := GetStore(); s.SetX(v)` followed by `GetStore().GetX()` would
// silently lose the write because the second call would see a different
// zero-value store. This test would fail with the per-call
// `&InMemoryStore{}` fallback that pre-dated the fix.
func TestGetStore_FallbackIsSharedAcrossCalls(t *testing.T) {
	t.Cleanup(func() { SetStore(nil) })
	SetStore(nil)

	a := GetStore()
	b := GetStore()
	require.Same(t, a, b, "fallback must be shared across consecutive GetStore calls")

	// Mutate via one handle, observe via another to prove the writes
	// land on a single underlying instance and aren't discarded.
	a.SetAppStatus(appstatetypes.AppStatus{State: appstatetypes.StateReady})
	require.Equal(t, appstatetypes.StateReady, b.GetAppStatus().State, "writes against the fallback must persist across GetStore calls")
}
