package appstate

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/report"
	reporttypes "github.com/replicatedhq/replicated-sdk/pkg/report/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Monitor struct {
	clientset       kubernetes.Interface
	targetNamespace string
	appInformersCh  chan appInformer
	appStatusCh     chan types.AppStatus
	cancel          context.CancelFunc
}

type EventHandler interface {
	ObjectCreated(obj interface{})
	ObjectUpdated(obj interface{})
	ObjectDeleted(obj interface{})
}

type appInformer struct {
	appSlug   string
	sequence  int64
	informers []types.StatusInformer
}

func NewMonitor(clientset kubernetes.Interface, targetNamespace string) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Monitor{
		clientset:       clientset,
		targetNamespace: targetNamespace,
		appInformersCh:  make(chan appInformer),
		appStatusCh:     make(chan types.AppStatus),
		cancel:          cancel,
	}
	go m.run(ctx)
	return m
}

func (m *Monitor) Shutdown() {
	m.cancel()
}

func (m *Monitor) Apply(appSlug string, sequence int64, informers []types.StatusInformer) {
	m.appInformersCh <- struct {
		appSlug   string
		sequence  int64
		informers []types.StatusInformer
	}{
		appSlug:   appSlug,
		sequence:  sequence,
		informers: informers,
	}
}

func (m *Monitor) AppStatusChan() <-chan types.AppStatus {
	return m.appStatusCh
}

func (m *Monitor) run(ctx context.Context) {
	log.Println("Starting monitor loop")

	defer close(m.appStatusCh)

	appMonitors := make(map[string]*AppMonitor)
	defer func() {
		for _, appMonitor := range appMonitors {
			appMonitor.Shutdown()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case appInformer := <-m.appInformersCh:
			appMonitor, ok := appMonitors[appInformer.appSlug]
			if !ok || appMonitor.sequence != appInformer.sequence {
				if appMonitor != nil {
					appMonitor.Shutdown()
				}
				appMonitor = NewAppMonitor(m.clientset, m.targetNamespace, appInformer.appSlug, appInformer.sequence)
				go func() {
					for appStatus := range appMonitor.AppStatusChan() {
						m.appStatusCh <- appStatus
					}
				}()
				appMonitors[appInformer.appSlug] = appMonitor
			}
			appMonitor.Apply(appInformer.informers)
		}
	}
}

type AppMonitor struct {
	clientset       kubernetes.Interface
	targetNamespace string
	appSlug         string
	informersCh     chan []types.StatusInformer
	appStatusCh     chan types.AppStatus
	cancel          context.CancelFunc
	sequence        int64
}

func NewAppMonitor(clientset kubernetes.Interface, targetNamespace, appSlug string, sequence int64) *AppMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	m := &AppMonitor{
		appSlug:         appSlug,
		clientset:       clientset,
		targetNamespace: targetNamespace,
		informersCh:     make(chan []types.StatusInformer),
		appStatusCh:     make(chan types.AppStatus),
		cancel:          cancel,
		sequence:        sequence,
	}
	go m.run(ctx)
	return m
}

func (m *AppMonitor) Shutdown() {
	m.cancel()
}

func (m *AppMonitor) Apply(informers []types.StatusInformer) {
	m.informersCh <- informers
}

func (m *AppMonitor) AppStatusChan() <-chan types.AppStatus {
	return m.appStatusCh
}

func (m *AppMonitor) run(ctx context.Context) {
	log.Println("Starting app monitor loop")

	defer close(m.informersCh)
	defer close(m.appStatusCh)

	prevCancel := context.CancelFunc(func() {})
	defer func() {
		// wrap this in a function to cancel the variable when updated
		prevCancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case informers := <-m.informersCh:
			prevCancel() // cancel previous loop

			log.Println("App monitor got new informers")

			ctx, cancel := context.WithCancel(ctx)
			prevCancel = cancel
			go m.runInformers(ctx, informers)
		}
	}
}

type runControllerFunc func(context.Context, kubernetes.Interface, string, []types.StatusInformer, chan<- types.ResourceState)

func (m *AppMonitor) runInformers(ctx context.Context, informers []types.StatusInformer) {
	informers = normalizeStatusInformers(informers, m.targetNamespace)

	log.Printf("Running informers: %#v", informers)

	resourceStates := buildResourceStatesFromStatusInformers(informers)
	appStatus := types.AppStatus{
		AppSlug:        m.appSlug,
		ResourceStates: resourceStates,
		UpdatedAt:      time.Now(),
		Sequence:       m.sequence,
		State:          types.GetState(resourceStates),
	}
	m.appStatusCh <- appStatus // reset last app status

	var shutdown sync.WaitGroup
	resourceStateCh := make(chan types.ResourceState)
	defer func() {
		shutdown.Wait()
		close(resourceStateCh)
	}()

	// Collect namespace/kind pairs
	namespaceKinds := make(map[string]map[string][]types.StatusInformer)
	for _, informer := range informers {
		kindsInNs, ok := namespaceKinds[informer.Namespace]
		if !ok {
			kindsInNs = make(map[string][]types.StatusInformer)
		}
		kindsInNs[informer.Kind] = append(kindsInNs[informer.Kind], informer)
		namespaceKinds[informer.Namespace] = kindsInNs
	}

	goRun := func(fn runControllerFunc, namespace string, informers []types.StatusInformer) {
		shutdown.Add(1)
		go func() {
			fn(ctx, m.clientset, namespace, informers, resourceStateCh)
			shutdown.Done()
		}()
	}

	kindImpls := map[string]runControllerFunc{
		DaemonSetResourceKind:             runDaemonSetController,
		DeploymentResourceKind:            runDeploymentController,
		IngressResourceKind:               runIngressController,
		PersistentVolumeClaimResourceKind: runPersistentVolumeClaimController,
		ServiceResourceKind:               runServiceController,
		StatefulSetResourceKind:           runStatefulSetController,
	}
	// Start a Pod image controller per namespace
	// When reportAllImages is true or in embedded cluster, watch all accessible namespaces
	sdkStore := store.GetStore()
	shouldWatchAllNamespaces := sdkStore.GetReportAllImages() || report.GetDistribution(m.clientset) == reporttypes.EmbeddedCluster

	informerNamespaces := make(map[string]struct{})
	for _, informer := range informers {
		informerNamespaces[informer.Namespace] = struct{}{}
	}
	namespacesToWatch := make(map[string]struct{})
	if shouldWatchAllNamespaces {
		// Check if we have permission to list namespaces before attempting
		if canListNamespaces(ctx, m.clientset) {
			// Get all namespaces
			namespaces, err := m.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				log.Printf("Failed to list namespaces for pod image tracking: %v", err)
				// Fall back to only watching namespaces with informers
				namespacesToWatch = informerNamespaces
			} else {
				for _, ns := range namespaces.Items {
					namespacesToWatch[ns.Name] = struct{}{}
				}
			}
		} else {
			log.Printf("No permission to list namespaces, falling back to namespaces with status informers")
			// Fall back to only watching namespaces with informers
			namespacesToWatch = informerNamespaces
		}
	} else {
		// Only watch namespaces that have status informers
		namespacesToWatch = informerNamespaces
	}

	// Filter out namespaces we don't have permission to access
	for ns := range namespacesToWatch {
		if canAccessPodsInNamespace(ctx, m.clientset, ns) {
			goRun(runPodImageController, ns, nil)
		}
	}
	for namespace, kinds := range namespaceKinds {
		for kind, informers := range kinds {
			if impl, ok := kindImpls[kind]; ok {
				goRun(impl, namespace, informers)
			} else {
				log.Printf("Informer requested for unsupported resource kind %v", kind)
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case resourceState := <-resourceStateCh:
			appStatus.ResourceStates = resourceStatesApplyNew(appStatus.ResourceStates, resourceState)
			appStatus.State = types.GetState(appStatus.ResourceStates)
			appStatus.UpdatedAt = time.Now() // TODO: this should come from the informer
			m.appStatusCh <- appStatus

		}
	}
}

func runInformer(ctx context.Context, informer cache.SharedInformer, eventHandler EventHandler) {
	defer utilruntime.HandleCrash()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			eventHandler.ObjectCreated(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			eventHandler.ObjectUpdated(new)
		},
		DeleteFunc: func(obj interface{}) {
			eventHandler.ObjectDeleted(obj)
		},
	})

	informer.Run(ctx.Done())
}

// canAccessPodsInNamespace checks if the current service account has permission to list and watch pods in the given namespace
func canAccessPodsInNamespace(ctx context.Context, clientset kubernetes.Interface, namespace string) bool {
	// Create a SelfSubjectAccessReview to check if we can list pods in this namespace
	sar := &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      "list",
				Group:     "",
				Resource:  "pods",
			},
		},
	}

	result, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to check access for pods in namespace %s: %v", namespace, err)
		return false
	}

	if !result.Status.Allowed {
		log.Printf("No permission to list pods in namespace %s, skipping", namespace)
		return false
	}

	// Also check watch permission
	sar.Spec.ResourceAttributes.Verb = "watch"
	result, err = clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to check watch access for pods in namespace %s: %v", namespace, err)
		return false
	}

	if !result.Status.Allowed {
		log.Printf("No permission to watch pods in namespace %s, skipping", namespace)
		return false
	}

	return true
}

// canListNamespaces checks if the current service account has permission to list namespaces
func canListNamespaces(ctx context.Context, clientset kubernetes.Interface) bool {
	sar := &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:     "list",
				Group:    "",
				Resource: "namespaces",
			},
		},
	}

	result, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to check namespace list permission: %v", err)
		return false
	}

	if !result.Status.Allowed {
		return false
	}

	return true
}
