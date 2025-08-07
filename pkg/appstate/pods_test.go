package appstate

import (
	"testing"

	"github.com/golang/mock/gomock"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	mock_store "github.com/replicatedhq/replicated-sdk/pkg/store/mock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func TestExtractImageInfo(t *testing.T) {
	tests := []struct {
		name     string
		imageRef string
		expected appstatetypes.ImageInfo
	}{
		{
			name:     "image with SHA digest",
			imageRef: "registry.com/my-app@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			expected: appstatetypes.ImageInfo{
				Name: "registry.com/my-app",
				SHA:  "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
		},
		{
			name:     "image with tag only",
			imageRef: "registry.com/my-app:latest",
			expected: appstatetypes.ImageInfo{
				Name: "",
				SHA:  "",
			},
		},
		{
			name:     "image with multiple @ symbols",
			imageRef: "registry@company.com/my-app@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			expected: appstatetypes.ImageInfo{
				Name: "registry@company.com/my-app",
				SHA:  "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
		},
		{
			name:     "simple image name with SHA",
			imageRef: "nginx@sha256:1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234567890",
			expected: appstatetypes.ImageInfo{
				Name: "nginx",
				SHA:  "sha256:1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234567890",
			},
		},
		{
			name:     "image without registry",
			imageRef: "my-app:v1.0.0",
			expected: appstatetypes.ImageInfo{
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

func TestPodImageEventHandler_MockStoreCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mock_store.NewMockStore(ctrl)
	store.SetStore(m)

	ns := "ns"
	handler := &podImageEventHandler{namespace: ns}

	pod1UID := k8stypes.UID("pod-1-uid")
	pod2UID := k8stypes.UID("pod-2-uid")

	pod1Running := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", UID: pod1UID},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Image: "nginx@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			}, {
				Image: "redis@sha256:efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678",
			}},
			InitContainerStatuses: []corev1.ContainerStatus{{
				Image: "busybox@sha256:ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012",
			}},
		},
	}
	pod1Pending := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", UID: pod1UID},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
	}
	pod2Running := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-2", UID: pod2UID},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Image: "postgres@sha256:mnop3456789012mnop3456789012mnop3456789012mnop3456789012mnop3456",
			}},
		},
	}

	expectedPod1Images := []appstatetypes.ImageInfo{
		{Name: "nginx", SHA: "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234"},
		{Name: "redis", SHA: "sha256:efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678"},
		{Name: "busybox", SHA: "sha256:ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012"},
	}
	expectedPod2Images := []appstatetypes.ImageInfo{
		{Name: "postgres", SHA: "sha256:mnop3456789012mnop3456789012mnop3456789012mnop3456789012mnop3456"},
	}

	gomock.InOrder(
		// Creating a running pod should set images
		m.EXPECT().SetPodImages(ns, string(pod1UID), expectedPod1Images),
		// Creating a pending pod should delete images
		m.EXPECT().DeletePodImages(ns, string(pod1UID)),
		// Updating to pending should delete images
		m.EXPECT().DeletePodImages(ns, string(pod1UID)),
		// Creating another running pod sets its images
		m.EXPECT().SetPodImages(ns, string(pod2UID), expectedPod2Images),
		// Updating back to running sets images again
		m.EXPECT().SetPodImages(ns, string(pod1UID), expectedPod1Images),
		// Deleting pod2 deletes its images
		m.EXPECT().DeletePodImages(ns, string(pod2UID)),
		// Deleting pod1 deletes its images
		m.EXPECT().DeletePodImages(ns, string(pod1UID)),
	)

	handler.ObjectCreated(pod1Running)
	handler.ObjectCreated(pod1Pending)
	handler.ObjectUpdated(pod1Pending)
	handler.ObjectCreated(pod2Running)
	handler.ObjectUpdated(pod1Running)
	handler.ObjectDeleted(pod2Running)
	handler.ObjectDeleted(pod1Running)
}
