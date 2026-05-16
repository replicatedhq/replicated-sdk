package startupstate

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarkFailed_BlocksLaterMarkReady(t *testing.T) {
	tr := New()
	tr.MarkFailed()

	require.False(t, tr.MarkReady(), "MarkReady after MarkFailed must not transition")
	require.Equal(t, Failed, tr.Get(), "Failed state must be sticky")
}

func TestConcurrentMarkReady(t *testing.T) {
	tr := New()
	const goroutines = 64

	var wg sync.WaitGroup
	transitions := make(chan bool, goroutines)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			transitions <- tr.MarkReady()
		}()
	}
	wg.Wait()
	close(transitions)

	count := 0
	for ok := range transitions {
		if ok {
			count++
		}
	}

	require.Equal(t, 1, count, "exactly one MarkReady call should report a transition")
	require.True(t, tr.IsReady(), "expected Ready after concurrent MarkReady")
}
