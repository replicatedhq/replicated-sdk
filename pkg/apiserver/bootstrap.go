package apiserver

import (
	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/appstate"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat"
	"github.com/replicatedhq/replicated-sdk/pkg/helm"
	"github.com/replicatedhq/replicated-sdk/pkg/integration"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
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

	var unverifiedLicense *kotsv1beta1.License
	if len(params.LicenseBytes) > 0 {
		l, err := sdklicense.LoadLicenseFromBytes(params.LicenseBytes)
		if err != nil {
			return errors.Wrap(err, "failed to parse license from base64")
		}
		unverifiedLicense = l
	} else if params.IntegrationLicenseID != "" {
		l, err := sdklicense.GetLicenseByID(params.IntegrationLicenseID, params.ReplicatedAppEndpoint)
		if err != nil {
			return backoff.Permanent(errors.Wrap(err, "failed to get license by id for integration license id"))
		}
		if l.Spec.LicenseType != "dev" {
			return errors.New("integration license must be a dev license")
		}
		unverifiedLicense = l
	}

	verifiedLicense, err := sdklicense.VerifySignature(unverifiedLicense)
	if err != nil {
		return backoff.Permanent(errors.Wrap(err, "failed to verify license signature"))
	}

	if !util.IsAirgap() {
		// sync license
		licenseData, err := sdklicense.GetLatestLicense(verifiedLicense, params.ReplicatedAppEndpoint)
		if err != nil {
			return errors.Wrap(err, "failed to get latest license")
		}
		verifiedLicense = licenseData.License
	}

	// check license expiration
	expired, err := sdklicense.LicenseIsExpired(verifiedLicense)
	if err != nil {
		return errors.Wrap(err, "failed to check if license is expired")
	}
	if expired {
		return backoff.Permanent(errors.New("License is expired"))
	}

	channelID := params.ChannelID
	if channelID == "" {
		channelID = verifiedLicense.Spec.ChannelID
	}

	channelName := params.ChannelName
	if channelName == "" {
		channelName = verifiedLicense.Spec.ChannelName
	}

	store.InitInMemory(store.InitInMemoryStoreOptions{
		License:               verifiedLicense,
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
		Namespace:             params.Namespace,
		ReplicatedID:          replicatedID,
		AppID:                 appID,
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
