package appstate

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
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
	appSlug       string
	sequence      int64
	labelSelector string
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

func (m *Monitor) Apply(appSlug string, sequence int64, labelSelector string) {
	m.appInformersCh <- struct {
		appSlug       string
		sequence      int64
		labelSelector string
	}{
		appSlug:       appSlug,
		sequence:      sequence,
		labelSelector: labelSelector,
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
			appMonitor.Apply(appInformer.labelSelector)
		}
	}
}

type AppMonitor struct {
	clientset       kubernetes.Interface
	targetNamespace string
	appSlug         string
	informersCh     chan string
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
		informersCh:     make(chan string),
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

func (m *AppMonitor) Apply(labelSelector string) {
	m.informersCh <- labelSelector
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

		case labelSelector := <-m.informersCh:
			prevCancel() // cancel previous loop

			log.Println("App monitor got new informers")

			ctx, cancel := context.WithCancel(ctx)
			prevCancel = cancel
			go m.runInformers(ctx, labelSelector)
		}
	}
}

type runControllerFunc func(context.Context, kubernetes.Interface, string, string, chan<- types.ResourceState)

func (m *AppMonitor) runInformers(ctx context.Context, labelSelector string) {
	log.Printf("Running informers with label selector: %#v", labelSelector)

	resourceStates := types.ResourceStates{}
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

	goRun := func(fn runControllerFunc, namespace string, labelSelector string) {
		shutdown.Add(1)
		go func() {
			fn(ctx, m.clientset, namespace, labelSelector, resourceStateCh)
			shutdown.Done()
		}()
	}

	goRun(runStatefulSetController, m.targetNamespace, labelSelector)
	goRun(runDeploymentController, m.targetNamespace, labelSelector)
	goRun(runDaemonSetController, m.targetNamespace, labelSelector)
	goRun(runIngressController, m.targetNamespace, labelSelector)
	goRun(runPersistentVolumeClaimController, m.targetNamespace, labelSelector)
	goRun(runServiceController, m.targetNamespace, labelSelector)

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
