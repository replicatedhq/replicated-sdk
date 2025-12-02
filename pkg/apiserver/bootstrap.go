package apiserver

import (
	"log"

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
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/report"
	reporttypes "github.com/replicatedhq/replicated-sdk/pkg/report/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
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
