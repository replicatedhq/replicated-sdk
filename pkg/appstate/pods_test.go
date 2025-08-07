package appstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestExtractImageInfo(t *testing.T) {
	tests := []struct {
		name     string
		imageRef string
		expected ImageInfo
	}{
		{
			name:     "image with SHA digest",
			imageRef: "registry.com/my-app@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			expected: ImageInfo{
				Name: "registry.com/my-app",
				SHA:  "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
		},
		{
			name:     "image with tag only",
			imageRef: "registry.com/my-app:latest",
			expected: ImageInfo{
				Name: "",
				SHA:  "",
			},
		},
		{
			name:     "image with multiple @ symbols",
			imageRef: "registry@company.com/my-app@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			expected: ImageInfo{
				Name: "registry@company.com/my-app",
				SHA:  "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
		},
		{
			name:     "simple image name with SHA",
			imageRef: "nginx@sha256:1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234567890",
			expected: ImageInfo{
				Name: "nginx",
				SHA:  "sha256:1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234567890",
			},
		},
		{
			name:     "image without registry",
			imageRef: "my-app:v1.0.0",
			expected: ImageInfo{
				Name: "",
				SHA:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractImageInfo(tt.imageRef)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageShaTracker_PodHandlers(t *testing.T) {
	tracker := &ImageShaTracker{
		podImages: make(map[string][]ImageInfo),
	}

	// Test data
	pod1UID := types.UID("pod-1-uid")
	pod2UID := types.UID("pod-2-uid")
	
	pod1Running := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
			UID:  pod1UID,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Image: "nginx@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
				},
				{
					Image: "redis@sha256:efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678",
				},
			},
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Image: "busybox@sha256:ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012",
				},
			},
		},
	}

	pod1Pending := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
			UID:  pod1UID,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Image: "nginx@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
				},
			},
		},
	}

	pod2Running := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-2",
			UID:  pod2UID,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Image: "postgres@sha256:mnop3456789012mnop3456789012mnop3456789012mnop3456789012mnop3456",
				},
			},
		},
	}

	// Test 1: Add running pod
	tracker.handlePodAdd(pod1Running)
	
	images := tracker.GetActiveImageInfo()
	expectedImages := []ImageInfo{
		{Name: "nginx", SHA: "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234"},
		{Name: "redis", SHA: "sha256:efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678"},
		{Name: "busybox", SHA: "sha256:ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012"},
	}
	require.ElementsMatch(t, expectedImages, images, "Should have expected images after adding pod1")

	// Test 2: Add pending pod (should be ignored)
	tracker.handlePodAdd(pod1Pending)
	
	images = tracker.GetActiveImageInfo()
	require.ElementsMatch(t, expectedImages, images, "Should still have same images after adding pending pod")

	// Test 3: Add second running pod
	tracker.handlePodAdd(pod2Running)
	
	images = tracker.GetActiveImageInfo()
	expectedImagesWithPod2 := []ImageInfo{
		{Name: "nginx", SHA: "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234"},
		{Name: "redis", SHA: "sha256:efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678"},
		{Name: "busybox", SHA: "sha256:ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012"},
		{Name: "postgres", SHA: "sha256:mnop3456789012mnop3456789012mnop3456789012mnop3456789012mnop3456"},
	}
	require.ElementsMatch(t, expectedImagesWithPod2, images, "Should have all images after adding pod2")

	// Test 4: Update pod1 to pending (should remove from tracking)
	tracker.handlePodUpdate(pod1Pending)
	
	images = tracker.GetActiveImageInfo()
	expectedOnlyPod2 := []ImageInfo{
		{Name: "postgres", SHA: "sha256:mnop3456789012mnop3456789012mnop3456789012mnop3456789012mnop3456"},
	}
	require.ElementsMatch(t, expectedOnlyPod2, images, "Should only have postgres image after pod1 becomes pending")

	// Test 5: Update pod1 back to running
	tracker.handlePodUpdate(pod1Running)
	
	images = tracker.GetActiveImageInfo()
	require.ElementsMatch(t, expectedImagesWithPod2, images, "Should have all images after pod1 running again")

	// Test 6: Delete pod2
	tracker.handlePodDelete(pod2Running)
	
	images = tracker.GetActiveImageInfo()
	require.ElementsMatch(t, expectedImages, images, "Should have pod1 images after deleting pod2")

	// Test 7: Delete pod1
	tracker.handlePodDelete(pod1Running)
	
	images = tracker.GetActiveImageInfo()
	require.ElementsMatch(t, []ImageInfo{}, images, "Should have no images after deleting all pods")
}