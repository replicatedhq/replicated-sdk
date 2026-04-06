package appstate

import (
	"context"
	"time"

	"github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	ServiceResourceKind = "service"
)

func init() {
	registerResourceKindNames(ServiceResourceKind, "services", "svc")
}

func runServiceController(
	ctx context.Context, clientset kubernetes.Interface, targetNamespace string,
	informers []types.StatusInformer, resourceStateCh chan<- types.ResourceState,
) {
	listwatch := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Services(targetNamespace).List(context.TODO(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Services(targetNamespace).Watch(context.TODO(), options)
		},
	}
	informer := cache.NewSharedInformer(
		listwatch,
		&corev1.Service{},
		// NOTE: services rely on endpoint status as well so unless we add additional
		// informers, we have to resync more frequently.
		10*time.Second,
	)

	eventHandler := NewServiceEventHandler(
		clientset,
		filterStatusInformersByResourceKind(informers, ServiceResourceKind),
		resourceStateCh,
	)

	runInformer(ctx, informer, eventHandler)
	return
}

type serviceEventHandler struct {
	clientset       kubernetes.Interface
	informers       []types.StatusInformer
	resourceStateCh chan<- types.ResourceState
}

func NewServiceEventHandler(clientset kubernetes.Interface, informers []types.StatusInformer, resourceStateCh chan<- types.ResourceState) *serviceEventHandler {
	return &serviceEventHandler{
		clientset:       clientset,
		informers:       informers,
		resourceStateCh: resourceStateCh,
	}
}

func (h *serviceEventHandler) ObjectCreated(obj interface{}) {
	r := h.cast(obj)
	if _, ok := h.getInformer(r); !ok {
		return
	}
	h.resourceStateCh <- makeServiceResourceState(r, CalculateServiceState(h.clientset, r))
}

func (h *serviceEventHandler) ObjectUpdated(obj interface{}) {
	r := h.cast(obj)
	if _, ok := h.getInformer(r); !ok {
		return
	}
	h.resourceStateCh <- makeServiceResourceState(r, CalculateServiceState(h.clientset, r))
}

func (h *serviceEventHandler) ObjectDeleted(obj interface{}) {
	r := h.cast(obj)
	if _, ok := h.getInformer(r); !ok {
		return
	}
	h.resourceStateCh <- makeServiceResourceState(r, types.StateMissing)
}

func (h *serviceEventHandler) cast(obj interface{}) *corev1.Service {
	r, _ := obj.(*corev1.Service)
	return r
}

func (h *serviceEventHandler) getInformer(r *corev1.Service) (types.StatusInformer, bool) {
	if r != nil {
		for _, informer := range h.informers {
			if r.Namespace == informer.Namespace && r.Name == informer.Name {
				return informer, true
			}
		}
	}
	return types.StatusInformer{}, false
}

func makeServiceResourceState(r *corev1.Service, state types.State) types.ResourceState {
	return types.ResourceState{
		Kind:      ServiceResourceKind,
		Name:      r.Name,
		Namespace: r.Namespace,
		State:     state,
	}
}

func CalculateServiceState(clientset kubernetes.Interface, r *corev1.Service) types.State {
	var states []types.State
	// https://github.com/kubernetes/kubectl/blob/6b77b0790ab40d2a692ad80e9e4c962e784bb9b8/pkg/describe/versioned/describe.go#L4617
	states = append(states, serviceGetStateFromEndpoints(clientset, r))
	// https://github.com/kubernetes/kubernetes/blob/badcd4af3f592376ce891b7c1b7a43ed6a18a348/pkg/printers/internalversion/printers.go#L1003
	states = append(states, serviceGetStateFromExternalIP(r))
	return types.MinState(states...)
}

func serviceGetStateFromEndpoints(clientset kubernetes.Interface, svc *corev1.Service) (minState types.State) {
	selector := labels.Set{discoveryv1.LabelServiceName: svc.Name}.AsSelector()
	endpointSlices, err := clientset.DiscoveryV1().EndpointSlices(svc.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil || endpointSlices == nil || len(endpointSlices.Items) == 0 {
		return types.StateUnavailable
	}
	for i := range svc.Spec.Ports {
		sp := &svc.Spec.Ports[i]
		minState = types.MinState(minState, servicePortGetStateFromEndpointSlices(endpointSlices.Items, sets.NewString(sp.Name)))
	}
	return
}

func servicePortGetStateFromEndpointSlices(slices []discoveryv1.EndpointSlice, ports sets.String) (minState types.State) {
	hasMatchingPort := false
	for _, slice := range slices {
		if len(slice.Ports) == 0 {
			// It's possible to have headless services with no ports.
			for _, ep := range slice.Endpoints {
				if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
					minState = types.MinState(minState, types.StateDegraded)
				}
			}
			continue
		}
		for _, port := range slice.Ports {
			if port.Name == nil || !ports.Has(*port.Name) {
				continue
			}
			hasMatchingPort = true
			readyCount := 0
			notReadyCount := 0
			for _, ep := range slice.Endpoints {
				if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
					readyCount++
				} else {
					notReadyCount++
				}
			}
			if readyCount == 0 {
				minState = types.MinState(minState, types.StateUnavailable)
			} else if notReadyCount > 0 {
				minState = types.MinState(minState, types.StateDegraded)
			} else {
				minState = types.MinState(minState, types.StateReady)
			}
		}
	}
	if !hasMatchingPort {
		return types.StateUnavailable
	}
	return
}

func serviceGetStateFromExternalIP(svc *corev1.Service) types.State {
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return types.StateReady
	}
	if len(svc.Spec.ExternalIPs) > 0 {
		return types.StateReady
	}
	lbIps := loadBalancerStatusIPs(svc.Status.LoadBalancer)
	if len(lbIps) > 0 {
		return types.StateReady
	}
	return types.StateUnavailable
}

func loadBalancerStatusIPs(s corev1.LoadBalancerStatus) sets.String {
	ingress := s.Ingress
	result := sets.NewString()
	for i := range ingress {
		if ingress[i].IP != "" {
			result.Insert(ingress[i].IP)
		} else if ingress[i].Hostname != "" {
			result.Insert(ingress[i].Hostname)
		}
	}
	return result
}
