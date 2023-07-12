package apiserver

import (
	"context"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/appstate"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/heartbeat"
	"github.com/replicatedhq/replicated-sdk/pkg/helm"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/mock"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

func bootstrap(params APIServerParams) error {
	verifiedLicense, err := sdklicense.VerifySignature(params.License)
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

	// generate / retrieve sdk and app ids
	replicatedID, appID, err := util.GenerateIDs(params.Namespace)
	if err != nil {
		return errors.Wrap(err, "failed to generate ids")
	}

	storeOptions := store.InitInMemoryStoreOptions{
		ReplicatedID:          replicatedID,
		AppID:                 appID,
		License:               verifiedLicense,
		LicenseFields:         params.LicenseFields,
		AppName:               params.AppName,
		ChannelID:             params.ChannelID,
		ChannelName:           params.ChannelName,
		ChannelSequence:       params.ChannelSequence,
		ReleaseSequence:       params.ReleaseSequence,
		ReleaseCreatedAt:      params.ReleaseCreatedAt,
		ReleaseNotes:          params.ReleaseNotes,
		VersionLabel:          params.VersionLabel,
		ReplicatedAppEndpoint: params.ReplicatedAppEndpoint,
		Namespace:             params.Namespace,
	}
	if err := store.InitInMemory(storeOptions); err != nil {
		return errors.Wrap(err, "failed to init store")
	}

	if !util.IsAirgap() {
		// retrieve and cache updates
		currentCursor := upstreamtypes.ReplicatedCursor{
			ChannelID:       store.GetStore().GetChannelID(),
			ChannelName:     store.GetStore().GetChannelName(),
			ChannelSequence: store.GetStore().GetChannelSequence(),
		}
		updates, err := upstream.ListPendingChannelReleases(store.GetStore(), store.GetStore().GetLicense(), currentCursor)
		if err != nil {
			return errors.Wrap(err, "failed to list pending channel releases")
		}
		store.GetStore().SetUpdates(updates)
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		return errors.Wrap(err, "failed to get clientset")
	}

	targetNamespace := params.Namespace
	if k8sutil.IsReplicatedClusterScoped(context.Background(), clientset, params.Namespace) {
		targetNamespace = "" // watch all namespaces
	}
	appStateOperator := appstate.InitOperator(clientset, targetNamespace)
	appStateOperator.Start()

	// if no status informers are provided, generate them from the helm release
	informers := params.StatusInformers
	if len(informers) == 0 && helm.IsHelmManaged() {
		helmRelease, err := helm.GetRelease(helm.GetReleaseName())
		if err != nil {
			return errors.Wrap(err, "failed to get helm release")
		}

		i, err := appstate.GenerateStatusInformersForManifest(helmRelease.Manifest)
		if err != nil {
			return errors.Wrap(err, "failed to generate status informers")
		}
		informers = i
	}

	appStateOperator.ApplyAppInformers(appstatetypes.AppInformersArgs{
		AppSlug:   store.GetStore().GetAppSlug(),
		Sequence:  store.GetStore().GetReleaseSequence(),
		Informers: informers,
	})

	if err := heartbeat.Start(); err != nil {
		return errors.Wrap(err, "failed to start heartbeat")
	}

	if store.GetStore().IsDevLicense() {
		mock.InitMock(clientset, store.GetStore().GetNamespace())
	}

	return nil
}
