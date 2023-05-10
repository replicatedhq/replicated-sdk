package appstate

import (
	"context"
	"time"

	"github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	DeploymentResourceKind = "deployment"
)

func runDeploymentController(
	ctx context.Context, clientset kubernetes.Interface, targetNamespace string,
	labelSelector string, resourceStateCh chan<- types.ResourceState,
) {
	listwatch := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = labelSelector
			return clientset.AppsV1().Deployments(targetNamespace).List(context.TODO(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = labelSelector
			return clientset.AppsV1().Deployments(targetNamespace).Watch(context.TODO(), options)
		},
	}
	informer := cache.NewSharedInformer(
		listwatch,
		&appsv1.Deployment{},
		time.Minute,
	)

	eventHandler := NewDeploymentEventHandler(
		resourceStateCh,
	)

	runInformer(ctx, informer, eventHandler)
}

type deploymentEventHandler struct {
	resourceStateCh chan<- types.ResourceState
}

func NewDeploymentEventHandler(resourceStateCh chan<- types.ResourceState) *deploymentEventHandler {
	return &deploymentEventHandler{
		resourceStateCh: resourceStateCh,
	}
}

func (h *deploymentEventHandler) ObjectCreated(obj interface{}) {
	r := h.cast(obj)
	h.resourceStateCh <- makeDeploymentResourceState(r, calculateDeploymentState(r))
}

func (h *deploymentEventHandler) ObjectUpdated(obj interface{}) {
	r := h.cast(obj)
	h.resourceStateCh <- makeDeploymentResourceState(r, calculateDeploymentState(r))
}

func (h *deploymentEventHandler) ObjectDeleted(obj interface{}) {
	r := h.cast(obj)
	h.resourceStateCh <- makeDeploymentResourceState(r, types.StateMissing)
}

func (h *deploymentEventHandler) cast(obj interface{}) *appsv1.Deployment {
	r, _ := obj.(*appsv1.Deployment)
	return r
}

func makeDeploymentResourceState(r *appsv1.Deployment, state types.State) types.ResourceState {
	return types.ResourceState{
		Kind:      DeploymentResourceKind,
		Name:      r.Name,
		Namespace: r.Namespace,
		State:     state,
	}
}

func calculateDeploymentState(r *appsv1.Deployment) types.State {
	if r.Status.ObservedGeneration != r.ObjectMeta.Generation {
		return types.StateUpdating
	}
	var desiredReplicas int32
	if r.Spec.Replicas == nil {
		desiredReplicas = 1
	} else {
		desiredReplicas = *r.Spec.Replicas
	}
	if r.Status.ReadyReplicas >= desiredReplicas {
		if r.Status.UnavailableReplicas > 0 {
			return types.StateUpdating
		}
		return types.StateReady
	}
	if r.Status.ReadyReplicas > 0 {
		return types.StateDegraded
	}
	return types.StateUnavailable
}
