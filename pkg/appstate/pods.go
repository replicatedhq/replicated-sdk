package appstate

import (
	"context"
	"strings"
	"time"

	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// runPodImageController starts a pod informer for a namespace and updates the
// store with the mapping of pod UID to container image digests. This follows the
// same informer pattern used by other controllers in this package.
func runPodImageController(ctx context.Context, clientset kubernetes.Interface, targetNamespace string, _ []appstatetypes.StatusInformer, _ chan<- appstatetypes.ResourceState) {
	listwatch := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Pods(targetNamespace).List(context.TODO(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Pods(targetNamespace).Watch(context.TODO(), options)
		},
	}
	informer := cache.NewSharedInformer(
		listwatch,
		&corev1.Pod{},
		time.Minute,
	)

	eventHandler := &podImageEventHandler{namespace: targetNamespace}
	runInformer(ctx, informer, eventHandler)
}

type podImageEventHandler struct {
	namespace string
}

func (h *podImageEventHandler) ObjectCreated(obj interface{}) {
	h.handle(obj)
}

func (h *podImageEventHandler) ObjectUpdated(obj interface{}) {
	h.handle(obj)
}

func (h *podImageEventHandler) ObjectDeleted(obj interface{}) {
	pod, _ := obj.(*corev1.Pod)
	if pod == nil {
		return
	}
	store.GetStore().DeletePodImages(h.namespace, string(pod.UID))
}

func (h *podImageEventHandler) handle(obj interface{}) {
	pod, _ := obj.(*corev1.Pod)
	if pod == nil {
		return
	}

	// Only track running pods
	if pod.Status.Phase != corev1.PodRunning {
		store.GetStore().DeletePodImages(h.namespace, string(pod.UID))
		return
	}

	images := extractPodImages(pod)
	if len(images) == 0 {
		store.GetStore().DeletePodImages(h.namespace, string(pod.UID))
		return
	}
	store.GetStore().SetPodImages(h.namespace, string(pod.UID), images)
}

func extractPodImages(pod *corev1.Pod) []appstatetypes.ImageInfo {
	var images []appstatetypes.ImageInfo

	// Extract from regular container statuses
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if info := extractImageInfo(pod, containerStatus, false); info.SHA != "" {
			images = append(images, info)
		}
	}

	// Extract from init container statuses
	for _, containerStatus := range pod.Status.InitContainerStatuses {
		if info := extractImageInfo(pod, containerStatus, true); info.SHA != "" {
			images = append(images, info)
		}
	}

	return images
}

// getContainerImageFromSpec looks up the container image from the pod spec by name
func getContainerImageFromSpec(pod *corev1.Pod, containerName string) string {
	for _, container := range pod.Spec.Containers {
		if container.Name == containerName {
			return container.Image
		}
	}
	return ""
}

// getInitContainerImageFromSpec looks up the init container image from the pod spec by name
func getInitContainerImageFromSpec(pod *corev1.Pod, containerName string) string {
	for _, container := range pod.Spec.InitContainers {
		if container.Name == containerName {
			return container.Image
		}
	}
	return ""
}

// extractImageInfo extracts the image name and SHA digest from a container status
func extractImageInfo(pod *corev1.Pod, containerStatus corev1.ContainerStatus, isInitContainer bool) appstatetypes.ImageInfo {
	imageRef := containerStatus.ImageID
	atIndex := strings.LastIndex(imageRef, "@")
	if atIndex == -1 {
		return appstatetypes.ImageInfo{Name: "", SHA: ""}
	}

	sha := imageRef[atIndex+1:]

	// Try to get the image name from the pod spec first (includes tag)
	var image string
	if isInitContainer {
		image = getInitContainerImageFromSpec(pod, containerStatus.Name)
	} else {
		image = getContainerImageFromSpec(pod, containerStatus.Name)
	}

	// Fall back to ImageID if spec lookup fails
	if image == "" {
		image = imageRef[:atIndex]
	}

	// Last resort: use containerStatus.Image if we still don't have an image
	if image == "" {
		image = containerStatus.Image
	}

	return appstatetypes.ImageInfo{
		Name: image,
		SHA:  sha,
	}
}
