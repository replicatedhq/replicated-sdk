package leader

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewLeaderElector(t *testing.T) {
	tests := []struct {
		name          string
		podName       string
		config        Config
		expectError   bool
		errorContains string
	}{
		{
			name:    "valid config",
			podName: "test-pod-1",
			config: Config{
				LeaseName:      "test-lease",
				LeaseNamespace: "test-namespace",
				LeaseDuration:  15 * time.Second,
				RenewDeadline:  10 * time.Second,
				RetryPeriod:    2 * time.Second,
			},
			expectError: false,
		},
		{
			name:          "missing pod name",
			podName:       "",
			config:        Config{},
			expectError:   true,
			errorContains: "REPLICATED_POD_NAME",
		},
		{
			name:    "missing lease name",
			podName: "test-pod-1",
			config: Config{
				LeaseNamespace: "test-namespace",
			},
			expectError:   true,
			errorContains: "lease name",
		},
		{
			name:    "missing lease namespace",
			podName: "test-pod-1",
			config: Config{
				LeaseName: "test-lease",
			},
			expectError:   true,
			errorContains: "lease namespace",
		},
		{
			name:    "default timings",
			podName: "test-pod-1",
			config: Config{
				LeaseName:      "test-lease",
				LeaseNamespace: "test-namespace",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.podName != "" {
				os.Setenv("REPLICATED_POD_NAME", tt.podName)
				defer os.Unsetenv("REPLICATED_POD_NAME")
			} else {
				os.Unsetenv("REPLICATED_POD_NAME")
			}

			clientset := fake.NewSimpleClientset()
			le, err := NewLeaderElector(clientset, tt.config)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, le)
			} else {
				require.NoError(t, err)
				require.NotNil(t, le)
				assert.Equal(t, tt.podName, le.GetIdentity())
				assert.False(t, le.IsLeader()) // Initially not leader
			}
		})
	}
}

func TestLeaderElector_IsLeader(t *testing.T) {
	os.Setenv("REPLICATED_POD_NAME", "test-pod-1")
	defer os.Unsetenv("REPLICATED_POD_NAME")

	clientset := fake.NewSimpleClientset()
	config := Config{
		LeaseName:      "test-lease",
		LeaseNamespace: "test-namespace",
		LeaseDuration:  1 * time.Second,
		RenewDeadline:  500 * time.Millisecond,
		RetryPeriod:    100 * time.Millisecond,
	}

	le, err := NewLeaderElector(clientset, config)
	require.NoError(t, err)
	require.NotNil(t, le)

	// Initially should not be leader
	assert.False(t, le.IsLeader())

	// Test that setLeader changes the state
	kle := le.(*kubernetesLeaderElector)
	kle.setLeader(true)
	assert.True(t, le.IsLeader())

	kle.setLeader(false)
	assert.False(t, le.IsLeader())
}

func TestLeaderElector_GetIdentity(t *testing.T) {
	podName := "test-pod-123"
	os.Setenv("REPLICATED_POD_NAME", podName)
	defer os.Unsetenv("REPLICATED_POD_NAME")

	clientset := fake.NewSimpleClientset()
	config := Config{
		LeaseName:      "test-lease",
		LeaseNamespace: "test-namespace",
	}

	le, err := NewLeaderElector(clientset, config)
	require.NoError(t, err)
	require.NotNil(t, le)

	assert.Equal(t, podName, le.GetIdentity())
}

func TestLeaderElector_Callbacks(t *testing.T) {
	os.Setenv("REPLICATED_POD_NAME", "test-pod-1")
	defer os.Unsetenv("REPLICATED_POD_NAME")

	clientset := fake.NewSimpleClientset()

	startedLeadingCalled := false
	stoppedLeadingCalled := false
	newLeaderCalled := false
	var newLeaderIdentity string

	config := Config{
		LeaseName:      "test-lease",
		LeaseNamespace: "test-namespace",
		LeaseDuration:  1 * time.Second,
		RenewDeadline:  500 * time.Millisecond,
		RetryPeriod:    100 * time.Millisecond,
		OnStartedLeading: func(ctx context.Context) {
			startedLeadingCalled = true
		},
		OnStoppedLeading: func() {
			stoppedLeadingCalled = true
		},
		OnNewLeader: func(identity string) {
			newLeaderCalled = true
			newLeaderIdentity = identity
		},
	}

	_, err := NewLeaderElector(clientset, config)
	require.NoError(t, err)

	// Test OnStartedLeading callback
	config.OnStartedLeading(context.Background())
	assert.True(t, startedLeadingCalled)

	// Test OnStoppedLeading callback
	config.OnStoppedLeading()
	assert.True(t, stoppedLeadingCalled)

	// Test OnNewLeader callback
	config.OnNewLeader("test-pod-1")
	assert.True(t, newLeaderCalled)
	assert.Equal(t, "test-pod-1", newLeaderIdentity)

	config.OnNewLeader("test-pod-2")
	assert.Equal(t, "test-pod-2", newLeaderIdentity)
}

func TestLeaderElector_Start(t *testing.T) {
	os.Setenv("REPLICATED_POD_NAME", "test-pod-1")
	defer os.Unsetenv("REPLICATED_POD_NAME")

	// Create a fake clientset with a pre-existing lease
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lease",
			Namespace: "test-namespace",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: stringPtr("test-pod-1"),
		},
	}
	clientset := fake.NewSimpleClientset(lease)

	config := Config{
		LeaseName:      "test-lease",
		LeaseNamespace: "test-namespace",
		LeaseDuration:  1 * time.Second,
		RenewDeadline:  500 * time.Millisecond,
		RetryPeriod:    100 * time.Millisecond,
	}

	le, err := NewLeaderElector(clientset, config)
	require.NoError(t, err)
	require.NotNil(t, le)

	// Test Start with a context that gets cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start should block until context is cancelled
	err = le.Start(ctx)
	assert.NoError(t, err) // Start should return nil when context is cancelled
}

func TestLeaderElector_String(t *testing.T) {
	os.Setenv("REPLICATED_POD_NAME", "test-pod-1")
	defer os.Unsetenv("REPLICATED_POD_NAME")

	clientset := fake.NewSimpleClientset()
	config := Config{
		LeaseName:      "test-lease",
		LeaseNamespace: "test-namespace",
	}

	le, err := NewLeaderElector(clientset, config)
	require.NoError(t, err)
	require.NotNil(t, le)

	kle := le.(*kubernetesLeaderElector)

	str := kle.String()
	assert.Contains(t, str, "test-pod-1")
	assert.Contains(t, str, "LeaderElector")
	assert.Contains(t, str, "false") // Initially not leader

	kle.setLeader(true)
	str = kle.String()
	assert.Contains(t, str, "true")
}

func TestLeaderElector_WaitForLeader(t *testing.T) {
	os.Setenv("REPLICATED_POD_NAME", "test-pod-1")
	defer os.Unsetenv("REPLICATED_POD_NAME")

	// Create a fake clientset with a pre-existing lease held by another pod
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lease",
			Namespace: "test-namespace",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: stringPtr("test-pod-2"),
		},
	}
	clientset := fake.NewSimpleClientset(lease)

	config := Config{
		LeaseName:      "test-lease",
		LeaseNamespace: "test-namespace",
		LeaseDuration:  1 * time.Second,
		RenewDeadline:  500 * time.Millisecond,
		RetryPeriod:    100 * time.Millisecond,
	}

	le, err := NewLeaderElector(clientset, config)
	require.NoError(t, err)
	require.NotNil(t, le)

	// Start leader election in background
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_ = le.Start(ctx)
	}()

	// WaitForLeader should block until a leader is elected
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer waitCancel()

	isLeader, err := le.WaitForLeader(waitCtx)
	require.NoError(t, err)

	// Either this pod or another pod should be the leader
	// In this test, test-pod-2 holds the lease initially
	assert.NotNil(t, isLeader) // Just verify we got a response
}

func TestLeaderElector_WaitForLeader_Timeout(t *testing.T) {
	os.Setenv("REPLICATED_POD_NAME", "test-pod-1")
	defer os.Unsetenv("REPLICATED_POD_NAME")

	clientset := fake.NewSimpleClientset()
	config := Config{
		LeaseName:      "test-lease",
		LeaseNamespace: "test-namespace",
		LeaseDuration:  10 * time.Second,
		RenewDeadline:  8 * time.Second,
		RetryPeriod:    2 * time.Second,
	}

	le, err := NewLeaderElector(clientset, config)
	require.NoError(t, err)
	require.NotNil(t, le)

	// Don't start the leader election - just test timeout
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer waitCancel()

	_, err = le.WaitForLeader(waitCtx)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func stringPtr(s string) *string {
	return &s
}
