package appstate

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/mitchellh/hashstructure"
	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/report"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"k8s.io/client-go/kubernetes"
)

var (
	operator *Operator
)

type Operator struct {
	targetNamespace string
	clientset       kubernetes.Interface
	appStateMonitor *Monitor
}

// NewOperator creates and initializes a new Operator.
func InitOperator(clientset kubernetes.Interface, targetNamespace string) *Operator {
	operator = &Operator{
		clientset:       clientset,
		targetNamespace: targetNamespace,
	}
	return operator
}

func MustGetOperator() *Operator {
	if operator != nil {
		return operator
	}
	panic("operator not initialized")
}

func (o *Operator) Start() {
	o.appStateMonitor = NewMonitor(o.clientset, o.targetNamespace)
	go o.runAppStateMonitor()
}

func (o *Operator) Shutdown() {
	log.Println("Shutting down the status informers client")

	if o.appStateMonitor != nil {
		o.appStateMonitor.Shutdown()
	}
}

func (o *Operator) runAppStateMonitor() error {
	m := map[string]func(f func()){}
	hash := map[string]uint64{}
	var mtx sync.Mutex

	for appStatus := range o.appStateMonitor.AppStatusChan() {
		throttled, ok := m[appStatus.AppSlug]
		if !ok {
			throttled = util.NewThrottle(time.Second)
			m[appStatus.AppSlug] = throttled
		}
		throttled(func() {
			mtx.Lock()
			lastHash := hash[appStatus.AppSlug]
			nextHash, _ := hashstructure.Hash(appStatus, nil)
			hash[appStatus.AppSlug] = nextHash
			mtx.Unlock()
			if lastHash != nextHash {
				b, _ := json.Marshal(appStatus)
				log.Printf("Updating app status %s", b)
			}
			if err := o.setAppStatus(appStatus); err != nil {
				log.Printf("error updating app status: %v", err)
			}
		})
	}

	return errors.New("app state monitor shutdown")
}

func (o *Operator) ApplyAppInformers(args types.AppInformersArgs) {
	log.Printf("received an inform event: %#v", args)

	appSlug := args.AppSlug
	sequence := args.Sequence
	informerStrings := args.Informers

	var informers []types.StatusInformer
	for _, str := range informerStrings {
		informer, err := str.Parse()
		if err != nil {
			log.Printf("failed to parse informer %s: %s", str, err.Error())
			continue // don't stop
		}
		informers = append(informers, informer)
	}

	if len(informers) == 0 {
		// no informers, set state to ready and return
		defaultReadyStatus := types.AppStatus{
			AppSlug:        appSlug,
			ResourceStates: types.ResourceStates{},
			UpdatedAt:      time.Now(),
			State:          types.StateReady,
			Sequence:       sequence,
		}
		if err := o.setAppStatus(defaultReadyStatus); err != nil {
			log.Printf("error updating app status: %v", err)
		}
		return
	}

	o.appStateMonitor.Apply(appSlug, sequence, informers)
}

func (o *Operator) setAppStatus(newAppStatus types.AppStatus) error {
	currentAppStatus := store.GetStore().GetAppStatus()
	store.GetStore().SetAppStatus(newAppStatus)

	if newAppStatus.State != currentAppStatus.State {
		log.Printf("app state changed from %q to %q", currentAppStatus.State, newAppStatus.State)
		go func() {
			clientset, err := k8sutil.GetClientset()
			if err != nil {
				logger.Error(errors.Wrap(err, "failed to get clientset"))
				return
			}
			if err := report.SendInstanceData(clientset, store.GetStore()); err != nil {
				logger.Error(errors.Wrap(err, "failed to send instance data"))
			}
		}()
	}

	return nil
}
