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
		name            string
		pod             *corev1.Pod
		containerStatus corev1.ContainerStatus
		isInitContainer bool
		expected        appstatetypes.ImageInfo
	}{
		{
			name: "image with SHA digest - use spec with tag",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "my-container",
						Image: "registry.com/my-app:v1.0",
					}},
				},
			},
			containerStatus: corev1.ContainerStatus{
				Name:    "my-container",
				Image:   "sha256:blah",
				ImageID: "registry.com/my-app@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
			isInitContainer: false,
			expected: appstatetypes.ImageInfo{
				Name: "registry.com/my-app:v1.0",
				SHA:  "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
		},
		{
			name: "init container with SHA digest - use spec with tag",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{
						Name:  "init-container",
						Image: "busybox:1.36",
					}},
				},
			},
			containerStatus: corev1.ContainerStatus{
				Name:    "init-container",
				Image:   "sha256:xyz123",
				ImageID: "busybox@sha256:efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678",
			},
			isInitContainer: true,
			expected: appstatetypes.ImageInfo{
				Name: "busybox:1.36",
				SHA:  "sha256:efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678",
			},
		},
		{
			name: "container not in spec - fallback to ImageID",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "different-container",
						Image: "other:v1",
					}},
				},
			},
			containerStatus: corev1.ContainerStatus{
				Name:    "my-container",
				Image:   "sha256:abcd1234",
				ImageID: "registry.com/my-app@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
			isInitContainer: false,
			expected: appstatetypes.ImageInfo{
				Name: "registry.com/my-app",
				SHA:  "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
		},
		{
			name: "empty spec - fallback to ImageID",
			pod:  &corev1.Pod{},
			containerStatus: corev1.ContainerStatus{
				Name:    "my-container",
				Image:   "sha256:abcd1234",
				ImageID: "registry.com/my-app@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
			isInitContainer: false,
			expected: appstatetypes.ImageInfo{
				Name: "registry.com/my-app",
				SHA:  "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
		},
		{
			name: "image with tag only - no SHA",
			pod:  &corev1.Pod{},
			containerStatus: corev1.ContainerStatus{
				Image:   "registry.com/my-app:latest",
				ImageID: "registry.com/my-app:latest",
			},
			isInitContainer: false,
			expected: appstatetypes.ImageInfo{
				Name: "",
				SHA:  "",
			},
		},
		{
			name: "image with multiple @ symbols",
			pod:  &corev1.Pod{},
			containerStatus: corev1.ContainerStatus{
				Image:   "registry@company.com/my-app",
				ImageID: "registry@company.com/my-app@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
			isInitContainer: false,
			expected: appstatetypes.ImageInfo{
				Name: "registry@company.com/my-app",
				SHA:  "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			},
		},
		{
			name: "spec has tag, status.Image is sha256 - prefer spec",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "nginx-container",
						Image: "nginx:1.25",
					}},
				},
			},
			containerStatus: corev1.ContainerStatus{
				Name:    "nginx-container",
				Image:   "sha256:1234567890abcd",
				ImageID: "nginx@sha256:1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234567890",
			},
			isInitContainer: false,
			expected: appstatetypes.ImageInfo{
				Name: "nginx:1.25",
				SHA:  "sha256:1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234567890",
			},
		},
		{
			name: "image without registry",
			pod:  &corev1.Pod{},
			containerStatus: corev1.ContainerStatus{
				Image:   "my-app:v1.0.0",
				ImageID: "my-app:v1.0.0",
			},
			isInitContainer: false,
			expected: appstatetypes.ImageInfo{
				Name: "",
				SHA:  "",
			},
		},
		{
			name: "spec has image with tag and digest - strip digest",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "operator",
						Image: "proxy.staging.replicated.com/anonymous/replicated/embedded-cluster-operator-image:v2.11.3-k8s-1.33-amd64@sha256:bf5128d4b342ccb8c7c42f1c8f1df6569fcb3e3ea3c58f43e0e3d40cabd4e75b",
					}},
				},
			},
			containerStatus: corev1.ContainerStatus{
				Name:    "operator",
				Image:   "sha256:a2f472acaf6839c6e046f3c157f9b47be1e01128695f14a3f108e68b078a3f90",
				ImageID: "proxy.staging.replicated.com/anonymous/replicated/embedded-cluster-operator-image@sha256:bf5128d4b342ccb8c7c42f1c8f1df6569fcb3e3ea3c58f43e0e3d40cabd4e75b",
			},
			isInitContainer: false,
			expected: appstatetypes.ImageInfo{
				Name: "proxy.staging.replicated.com/anonymous/replicated/embedded-cluster-operator-image:v2.11.3-k8s-1.33-amd64",
				SHA:  "sha256:bf5128d4b342ccb8c7c42f1c8f1df6569fcb3e3ea3c58f43e0e3d40cabd4e75b",
			},
		},
		{
			name: "spec has image with digest only - strip digest",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "operator",
						Image: "proxy.staging.replicated.com/anonymous/replicated/embedded-cluster-operator-image@sha256:bf5128d4b342ccb8c7c42f1c8f1df6569fcb3e3ea3c58f43e0e3d40cabd4e75b",
					}},
				},
			},
			containerStatus: corev1.ContainerStatus{
				Name:    "operator",
				Image:   "sha256:a2f472acaf6839c6e046f3c157f9b47be1e01128695f14a3f108e68b078a3f90",
				ImageID: "proxy.staging.replicated.com/anonymous/replicated/embedded-cluster-operator-image@sha256:bf5128d4b342ccb8c7c42f1c8f1df6569fcb3e3ea3c58f43e0e3d40cabd4e75b",
			},
			isInitContainer: false,
			expected: appstatetypes.ImageInfo{
				Name: "proxy.staging.replicated.com/anonymous/replicated/embedded-cluster-operator-image",
				SHA:  "sha256:bf5128d4b342ccb8c7c42f1c8f1df6569fcb3e3ea3c58f43e0e3d40cabd4e75b",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractImageInfo(tt.pod, tt.containerStatus, tt.isInitContainer)
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
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "nginx-container",
				Image: "nginx:1",
			}, {
				Name:  "redis-container",
				Image: "redis:2",
			}},
			InitContainers: []corev1.Container{{
				Name:  "busybox-container",
				Image: "busybox:3",
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "nginx-container",
				Image:   "nginx",
				ImageID: "nginx@sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			}, {
				Name:    "redis-container",
				Image:   "redis",
				ImageID: "redis@sha256:efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678",
			}},
			InitContainerStatuses: []corev1.ContainerStatus{{
				Name:    "busybox-container",
				Image:   "busybox",
				ImageID: "busybox@sha256:ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012",
			}},
		},
	}
	pod1Pending := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", UID: pod1UID},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
	}
	pod2Running := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-2", UID: pod2UID},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "postgres-container",
				Image: "postgres:17",
			}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "postgres-container",
				Image:   "postgres",
				ImageID: "postgres@sha256:mnop3456789012mnop3456789012mnop3456789012mnop3456789012mnop3456",
			}},
		},
	}

	expectedPod1Images := []appstatetypes.ImageInfo{
		{Name: "nginx:1", SHA: "sha256:abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234"},
		{Name: "redis:2", SHA: "sha256:efgh5678901234efgh5678901234efgh5678901234efgh5678901234efgh5678"},
		{Name: "busybox:3", SHA: "sha256:ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012345678ijkl9012"},
	}
	expectedPod2Images := []appstatetypes.ImageInfo{
		{Name: "postgres:17", SHA: "sha256:mnop3456789012mnop3456789012mnop3456789012mnop3456789012mnop3456"},
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
