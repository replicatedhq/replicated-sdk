package apiserver

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	licensewrappertypes "github.com/replicatedhq/kotskinds/pkg/licensewrapper/types"
	"github.com/replicatedhq/replicated-sdk/pkg/appstate"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat"
	"github.com/replicatedhq/replicated-sdk/pkg/helm"
	"github.com/replicatedhq/replicated-sdk/pkg/integration"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/leader"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/report"
	reporttypes "github.com/replicatedhq/replicated-sdk/pkg/report/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	"k8s.io/client-go/kubernetes"
)

func bootstrap(params APIServerParams) error {
	clientset, err := k8sutil.GetClientset()
	if err != nil {
		return errors.Wrap(err, "failed to get clientset")
	}

	replicatedID, appID := params.ReplicatedID, params.AppID
	if replicatedID == "" || appID == "" {
		// retrieve replicated and app ids
		replicatedID, appID, err = util.GetReplicatedAndAppIDs(clientset, params.Namespace)
		if err != nil {
			return errors.Wrap(err, "failed to get replicated and app ids")
		}
	}
	if replicatedID == "" {
		return backoff.Permanent(errors.New("Replicated ID not found"))
	}
	if appID == "" {
		return backoff.Permanent(errors.New("App ID not found"))
	}

	log.Println("replicatedID:", replicatedID)
	log.Println("appID:", appID)

	// In Embedded Cluster installations, automatically enable reporting all images
	reportAllImages := params.ReportAllImages
	if !reportAllImages {
		distribution := report.GetDistribution(clientset)
		if distribution == reporttypes.EmbeddedCluster {
			reportAllImages = true
			log.Println("Detected Embedded Cluster installation, enabling reportAllImages")
		}
	}

	var unverifiedWrapper licensewrapper.LicenseWrapper
	if len(params.LicenseBytes) > 0 {
		wrapper, err := sdklicense.LoadLicenseFromBytes(params.LicenseBytes)
		if err != nil {
			return errors.Wrap(err, "failed to parse license from base64")
		}
		unverifiedWrapper = wrapper
	} else if params.IntegrationLicenseID != "" {
		wrapper, err := sdklicense.GetLicenseByID(params.IntegrationLicenseID, params.ReplicatedAppEndpoint)
		if err != nil {
			return backoff.Permanent(errors.Wrap(err, "failed to get license by id for integration license id"))
		}
		if wrapper.GetLicenseType() != "dev" {
			return errors.New("integration license must be a dev license")
		}
		unverifiedWrapper = wrapper
	}

	err = unverifiedWrapper.VerifySignature()
	if err != nil {
		if licensewrappertypes.IsLicenseDataValidationError(err) {
			// this is not a fatal error, it means that the license data outside of the signature was changed
			// however, the data inside the signature was still valid, and so the license has been updated to use that data instead
			log.Println(err.Error())
		} else {
			return backoff.Permanent(errors.Wrap(err, "failed to verify license signature"))
		}
	}
	verifiedWrapper := unverifiedWrapper

	if !util.IsAirgap() {
		// sync license
		licenseData, err := sdklicense.GetLatestLicense(verifiedWrapper, params.ReplicatedAppEndpoint)
		if err != nil {
			return errors.Wrap(err, "failed to get latest license")
		}
		verifiedWrapper = licenseData.License
	}

	// check license expiration
	expired, err := sdklicense.LicenseIsExpired(verifiedWrapper)
	if err != nil {
		return errors.Wrap(err, "failed to check if license is expired")
	}
	if expired {
		return backoff.Permanent(errors.New("License is expired"))
	}

	channelID := params.ChannelID
	if channelID == "" {
		channelID = verifiedWrapper.GetChannelID()
	}

	channelName := params.ChannelName
	if channelName == "" {
		channelName = verifiedWrapper.GetChannelName()
	}

	store.InitInMemory(store.InitInMemoryStoreOptions{
		License:               verifiedWrapper,
		LicenseFields:         params.LicenseFields,
		AppName:               params.AppName,
		ChannelID:             channelID,
		ChannelName:           channelName,
		ChannelSequence:       params.ChannelSequence,
		ReleaseSequence:       params.ReleaseSequence,
		ReleaseCreatedAt:      params.ReleaseCreatedAt,
		ReleaseNotes:          params.ReleaseNotes,
		VersionLabel:          params.VersionLabel,
		ReplicatedAppEndpoint: params.ReplicatedAppEndpoint,
		ReleaseImages:         params.ReleaseImages,
		Namespace:             params.Namespace,
		ReplicatedID:          replicatedID,
		AppID:                 appID,
		ReportAllImages:       reportAllImages,
	})

	isIntegrationModeEnabled, err := integration.IsEnabled(params.Context, clientset, store.GetStore().GetNamespace(), store.GetStore().GetLicense())
	if err != nil {
		return errors.Wrap(err, "failed to check if integration mode is enabled")
	}

	if !util.IsAirgap() && !isIntegrationModeEnabled {
		// retrieve and cache updates
		currentCursor := upstreamtypes.ReplicatedCursor{
			ChannelID:       store.GetStore().GetChannelID(),
			ChannelName:     store.GetStore().GetChannelName(),
			ChannelSequence: store.GetStore().GetChannelSequence(),
		}
		updates, err := upstream.GetUpdates(store.GetStore(), store.GetStore().GetLicense(), currentCursor)
		if err != nil {
			return errors.Wrap(err, "failed to get updates")
		}
		store.GetStore().SetUpdates(updates)
	}

	appStateOperator := appstate.InitOperator(clientset, params.Namespace)
	appStateOperator.Start()

	// if no status informers are provided, generate them from the helm release
	informers := params.StatusInformers
	if informers == nil && helm.IsHelmManaged() {
		helmRelease, err := helm.GetRelease(helm.GetReleaseName())
		if err != nil {
			return errors.Wrap(err, "failed to get helm release")
		}
		if helmRelease != nil {
			informers = appstate.GenerateStatusInformersForManifest(helmRelease.Manifest)
		}
	}

	appStateOperator.ApplyAppInformers(appstatetypes.AppInformersArgs{
		AppSlug:   store.GetStore().GetAppSlug(),
		Sequence:  store.GetStore().GetReleaseSequence(),
		Informers: informers,
	})

	// Initialize and start leader election if HA mode is enabled
	if os.Getenv("REPLICATED_HA_ENABLED") == "true" {
		leaderConfig := createLeaderElectionConfig(params, clientset, appStateOperator)
		leaderElector, err := leader.NewLeaderElector(clientset, leaderConfig)
		if err != nil {
			return errors.Wrap(err, "failed to create leader elector")
		}

		store.GetStore().SetLeaderElector(leaderElector)

		// Start leader election in background (tracked so we can wait on shutdown).
		startLeaderElectionAsync(params.Context, leaderElector)
	}

	if err := heartbeat.Start(); err != nil {
		return errors.Wrap(err, "failed to start heartbeat")
	}

	// this is at the end of the bootstrap function so that it doesn't re-run on retry
	if !util.IsAirgap() && store.GetStore().IsDevLicense() {
		go func() {
			if err := util.WarnOnOutdatedReplicatedVersion(); err != nil {
				logger.Infof("Failed to check if running an outdated replicated version: %v", err)
			}
		}()
	}

	return nil
}

// createLeaderElectionConfig creates a leader election config from environment variables and params
func createLeaderElectionConfig(params APIServerParams, clientset kubernetes.Interface, appStateOperator *appstate.Operator) leader.Config {
	leaseDuration := 15 * time.Second
	renewDeadline := 10 * time.Second
	retryPeriod := 2 * time.Second

	// Parse from environment if provided
	if dur := os.Getenv("REPLICATED_LEASE_DURATION"); dur != "" {
		if parsed, err := time.ParseDuration(dur); err == nil {
			leaseDuration = parsed
		}
	}
	if dur := os.Getenv("REPLICATED_LEASE_RENEW_DEADLINE"); dur != "" {
		if parsed, err := time.ParseDuration(dur); err == nil {
			renewDeadline = parsed
		}
	}
	if dur := os.Getenv("REPLICATED_LEASE_RETRY_PERIOD"); dur != "" {
		if parsed, err := time.ParseDuration(dur); err == nil {
			retryPeriod = parsed
		}
	}

	return leader.Config{
		LeaseName:      "replicated-sdk-leader",
		LeaseNamespace: params.Namespace,
		LeaseDuration:  leaseDuration,
		RenewDeadline:  renewDeadline,
		RetryPeriod:    retryPeriod,
		OnStartedLeading: func(ctx context.Context) {
			logger.Infof("This instance became the leader")
			// Immediately send instance data when we become the leader
			// This ensures no reporting gaps after rollouts or failovers
			go func() {
				// // for testing send data twice, once before and once after waiting for appstate to sync
				// _ = report.SendInstanceData(clientset, store.GetStore())
				// // Ensure appstate has synced its informers before reporting, so we don't send "empty" state on startup/failover.
				// waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				// defer cancel()
				// if appStateOperator != nil {
				// 	if err := appStateOperator.WaitForSynced(waitCtx); err != nil {
				// 		logger.Errorf("Timed out waiting for appstate to sync before reporting: %v", err)
				// 	}
				// }

				// if err := report.SendInstanceData(clientset, store.GetStore()); err != nil {
				// 	logger.Errorf("Failed to send instance data after becoming leader: %v", err)
				// } else {
				// 	logger.Debugf("Successfully sent instance data after becoming leader")
				// }
			}()
		},
		OnStoppedLeading: func() {
			logger.Infof("This instance lost leadership")
		},
		OnNewLeader: func(identity string) {
			logger.Infof("New leader elected: %s", identity)
		},
	}
}
