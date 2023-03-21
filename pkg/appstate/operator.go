package appstate

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/mitchellh/hashstructure"
	"github.com/pkg/errors"
	"github.com/replicatedhq/kots-sdk/pkg/appstate/types"
	"github.com/replicatedhq/kots-sdk/pkg/reporting"
	"github.com/replicatedhq/kots-sdk/pkg/store"
	"github.com/replicatedhq/kots-sdk/pkg/util"
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
	labelSelector := args.LabelSelector

	o.appStateMonitor.Apply(appSlug, sequence, labelSelector)
}

func (o *Operator) setAppStatus(newAppStatus types.AppStatus) error {
	currentAppStatus := store.GetStore().GetAppStatus()
	store.GetStore().SetAppStatus(newAppStatus)

	if newAppStatus.State != currentAppStatus.State {
		log.Printf("app state changed from %s to %s", currentAppStatus.State, newAppStatus.State)
		go reporting.SendAppInfo()
	}

	return nil
}
