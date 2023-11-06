package heartbeat

import (
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/report"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	cron "github.com/robfig/cron/v3"
)

var job *cron.Cron
var mtx sync.Mutex

// Start will configure and start a heartbeat cron job for the app to send checkins to the server:
// if enabled, and cron job was NOT found: add a new cron job to send heartbeats
// if enabled, and a cron job was found, update the existing cron job with the latest cron spec
// if disabled: stop the current running cron job (if exists)
func Start() error {
	appSlug := store.GetStore().GetAppSlug()

	logger.Debugf("starting heartbeat for app %s", appSlug)

	mtx.Lock()
	defer mtx.Unlock()

	if job != nil {
		// job already exists, remove entries
		entries := job.Entries()
		for _, entry := range entries {
			job.Remove(entry.ID)
		}
	} else {
		// job does not exist, create a new one
		job = cron.New(cron.WithChain(
			cron.Recover(cron.DefaultLogger),
		))
	}

	// check for updates every 4 hours
	t := time.Now()
	m := t.Minute()
	h := t.Hour() % 4
	cronSpec := fmt.Sprintf("%d %d/4 * * *", m, h)

	_, err := job.AddFunc(cronSpec, func() {
		logger.Debugf("sending a heartbeat for app %s", appSlug)

		if !util.IsAirgap() {
			licenseData, err := sdklicense.GetLatestLicense(store.GetStore().GetLicense(), store.GetStore().GetReplicatedAppEndpoint())
			if err != nil {
				logger.Error(errors.Wrap(err, "failed to get latest license"))
			} else {
				store.GetStore().SetLicense(licenseData.License)
			}
		}

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
	})
	if err != nil {
		return errors.Wrap(err, "failed to add func")
	}

	job.Start()

	return nil
}

// Stop will stop a running cron job (if exists) for the app
func Stop() {
	if job != nil {
		job.Stop()
	} else {
		logger.Debugf("cron job not found")
	}
}
