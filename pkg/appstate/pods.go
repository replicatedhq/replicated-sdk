package appstate

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type ImageInfo struct {
	Name string
	SHA  string
}

type ImageShaTracker struct {
	clientset kubernetes.Interface
	namespace string
	podImages map[string][]ImageInfo // pod UID -> images
	mutex     sync.RWMutex
	cancel    context.CancelFunc
}

// Global registry of image SHA trackers keyed by namespace. This allows other
// packages to aggregate running images across all namespaces currently being
// tracked by the app monitor.
var (
	imageShaTrackers     = make(map[string]*ImageShaTracker)
	imageShaTrackersLock sync.Mutex
	// Optional: simple reference counts so multiple callers can ensure tracking
	// for the same namespace without double-starting, and release when done.
	imageShaTrackerRefs = make(map[string]int)
)

// EnsureImageShaTrackers starts trackers for the provided namespaces. It is
// safe to call multiple times; subsequent calls for the same namespace will
// increment a reference counter rather than starting a new tracker.
func EnsureImageShaTrackers(clientset kubernetes.Interface, namespaces []string) {
	imageShaTrackersLock.Lock()
	defer imageShaTrackersLock.Unlock()

	for _, ns := range namespaces {
		if ns == "" {
			continue
		}
		if tracker, ok := imageShaTrackers[ns]; ok && tracker != nil {
			imageShaTrackerRefs[ns] = imageShaTrackerRefs[ns] + 1
			continue
		}
		imageShaTrackers[ns] = NewImageShaTracker(clientset, ns)
		imageShaTrackerRefs[ns] = 1
	}
}

// ReleaseImageShaTrackers decrements references for the provided namespaces
// and stops trackers that are no longer referenced.
func ReleaseImageShaTrackers(namespaces []string) {
	imageShaTrackersLock.Lock()
	defer imageShaTrackersLock.Unlock()

	for _, ns := range namespaces {
		if ns == "" {
			continue
		}
		if refs, ok := imageShaTrackerRefs[ns]; ok {
			if refs <= 1 {
				if tracker := imageShaTrackers[ns]; tracker != nil {
					tracker.Shutdown()
				}
				delete(imageShaTrackers, ns)
				delete(imageShaTrackerRefs, ns)
			} else {
				imageShaTrackerRefs[ns] = refs - 1
			}
		}
	}
}

// GetActiveRunningImages aggregates all currently known images across all
// active trackers, returning a map of image name -> unique list of content SHAs.
func GetActiveRunningImages() map[string][]string {
	// Copy references under lock to avoid holding the lock during aggregation.
	imageShaTrackersLock.Lock()
	trackers := make([]*ImageShaTracker, 0, len(imageShaTrackers))
	for _, t := range imageShaTrackers {
		if t != nil {
			trackers = append(trackers, t)
		}
	}
	imageShaTrackersLock.Unlock()

	resultSet := make(map[string]map[string]struct{})
	for _, t := range trackers {
		infos := t.GetActiveImageInfo()
		for _, info := range infos {
			if info.Name == "" || info.SHA == "" {
				continue
			}
			if _, ok := resultSet[info.Name]; !ok {
				resultSet[info.Name] = make(map[string]struct{})
			}
			resultSet[info.Name][info.SHA] = struct{}{}
		}
	}

	// Convert set to slice
	result := make(map[string][]string, len(resultSet))
	for name, shas := range resultSet {
		list := make([]string, 0, len(shas))
		for sha := range shas {
			list = append(list, sha)
		}
		result[name] = list
	}
	return result
}

func NewImageShaTracker(clientset kubernetes.Interface, namespace string) *ImageShaTracker {
	ctx, cancel := context.WithCancel(context.Background())
	tracker := &ImageShaTracker{
		clientset: clientset,
		namespace: namespace,
		podImages: make(map[string][]ImageInfo),
		cancel:    cancel,
	}

	go tracker.run(ctx)
	return tracker
}

func (t *ImageShaTracker) GetActiveImageInfo() []ImageInfo {
	if t == nil {
		return []ImageInfo{}
	}

	t.mutex.RLock()
	defer t.mutex.RUnlock()

	// Aggregate unique images from all pods
	imageInfoSet := make(map[string]ImageInfo)
	for _, images := range t.podImages {
		for _, info := range images {
			// Use image name + SHA as key to avoid duplicates
			key := info.Name + "@" + info.SHA
			imageInfoSet[key] = info
		}
	}

	// Convert set to slice
	result := make([]ImageInfo, 0, len(imageInfoSet))
	for _, info := range imageInfoSet {
		result = append(result, info)
	}

	return result
}

func (t *ImageShaTracker) Shutdown() {
	if t != nil {
		t.cancel()
	}
}

func (t *ImageShaTracker) run(ctx context.Context) {
	log.Printf("Starting pod watcher for namespace: %s", t.namespace)

	listwatch := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return t.clientset.CoreV1().Pods(t.namespace).List(context.TODO(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return t.clientset.CoreV1().Pods(t.namespace).Watch(context.TODO(), options)
		},
	}

	informer := cache.NewSharedInformer(
		listwatch,
		&corev1.Pod{},
		time.Minute,
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			t.handlePodAdd(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			t.handlePodUpdate(new)
		},
		DeleteFunc: func(obj interface{}) {
			t.handlePodDelete(obj)
		},
	})

	informer.Run(ctx.Done())
}

func (t *ImageShaTracker) handlePodAdd(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	// Only track running pods
	if pod.Status.Phase != corev1.PodRunning {
		return
	}

	images := t.extractPodImages(pod)
	if len(images) == 0 {
		return
	}

	t.mutex.Lock()
	t.podImages[string(pod.UID)] = images
	t.mutex.Unlock()

	log.Printf("Added pod %s with images: %v", pod.Name, images)
}

func (t *ImageShaTracker) handlePodUpdate(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	podUID := string(pod.UID)

	t.mutex.Lock()
	defer t.mutex.Unlock()

	// If pod is no longer running, remove it from tracking
	if pod.Status.Phase != corev1.PodRunning {
		if _, exists := t.podImages[podUID]; exists {
			delete(t.podImages, podUID)
			log.Printf("Removed non-running pod %s from tracking", pod.Name)
		}
		return
	}

	// Update images for running pod
	images := t.extractPodImages(pod)
	if len(images) == 0 {
		delete(t.podImages, podUID)
		return
	}

	t.podImages[podUID] = images
	log.Printf("Updated pod %s with images: %v", pod.Name, images)
}

func (t *ImageShaTracker) handlePodDelete(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	podUID := string(pod.UID)

	t.mutex.Lock()
	if _, exists := t.podImages[podUID]; exists {
		delete(t.podImages, podUID)
		log.Printf("Deleted pod %s from tracking", pod.Name)
	}
	t.mutex.Unlock()
}

func (t *ImageShaTracker) extractPodImages(pod *corev1.Pod) []ImageInfo {
	var images []ImageInfo

	// Extract from regular container statuses
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if info := extractImageInfo(containerStatus.Image); info.SHA != "" {
			images = append(images, info)
		}
	}

	// Extract from init container statuses
	for _, containerStatus := range pod.Status.InitContainerStatuses {
		if info := extractImageInfo(containerStatus.Image); info.SHA != "" {
			images = append(images, info)
		}
	}

	return images
}

// extractImageInfo extracts the image name and SHA digest from an image reference
// Examples:
//   - "registry.com/image@sha256:abcd1234..." -> ImageInfo{Name: "registry.com/image", SHA: "sha256:abcd1234..."}
//   - "registry.com/image:tag" -> ImageInfo{Name: "", SHA: ""}
func extractImageInfo(imageRef string) ImageInfo {
	// Look for '@sha256:' or similar digest formats
	atIndex := strings.LastIndex(imageRef, "@")
	if atIndex == -1 {
		// No digest found
		return ImageInfo{Name: "", SHA: ""}
	}

	imageName := imageRef[:atIndex]
	sha := imageRef[atIndex+1:]

	return ImageInfo{
		Name: imageName,
		SHA:  sha,
	}
}
