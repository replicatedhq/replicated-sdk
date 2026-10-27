package heartbeat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/licensestate"
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

		// A managed installation receives successors through the signed rotation
		// protocol instead. The legacy refresh must not race it or replace the
		// installation-targeted license it saved.
		if !util.IsAirgap() && !licensestate.SDKManaged() {
			licenseData, err := sdklicense.GetLatestLicense(store.GetStore().GetLicense(), store.GetStore().GetReplicatedAppEndpoint())
			if err != nil {
				logger.Error(errors.Wrap(err, "failed to get latest license"))
			} else {
				store.GetStore().SetLicense(licenseData.License)
			}
		}
		// Checking for a credential rotation is the same kind of check-in on
		// the same cadence, so it rides this schedule rather than running a
		// second timer with its own, unjittered, interval.
		if manager := licensestate.Current(); manager != nil {
			if _, _, err := manager.SynchronizePortalRotation(context.Background(), store.GetStore().GetReplicatedAppEndpoint()); err != nil {
				logger.Infof("SDK managed license synchronization failed: %v", err)
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
