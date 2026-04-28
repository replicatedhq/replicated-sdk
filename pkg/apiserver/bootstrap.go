package apiserver

import (
	"context"
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
	"k8s.io/client-go/kubernetes"
)

// bootstrapCritical performs the bootstrap work that must succeed before the
// SDK can serve meaningful responses. It loads, signature-verifies, and
// expiry-checks the license, then populates the in-memory store. It also
// initializes the local appstate operator so app/* endpoints can begin
// reporting status as soon as the pod is Ready.
//
// In production mode (LicenseBytes provided by the chart), this path is
// fully local. In integration mode it consults the upstream Vendor Portal
// with cache fallback: a previously-cached license satisfies critical even
// when replicated.app is unreachable, so subsequent boots after one
// successful sync survive offline conditions. First-boot offline (no
// cache, unreachable upstream) returns backoff.Permanent — the SDK refuses
// to start rather than silently run with empty data.
//
// Permanent failures (license parse error, signature invalid, expired,
// first-boot-offline-with-no-cache) are returned as backoff.Permanent so
// the retry loop above gives up immediately. Transient failures bubble up
// unwrapped and the caller will retry.
func bootstrapCritical(params APIServerParams) error {
	clientset, err := k8sutil.GetClientset()
	if err != nil {
		return errors.Wrap(err, "failed to get clientset")
	}

	replicatedID, appID := params.ReplicatedID, params.AppID
	if replicatedID == "" || appID == "" {
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

	reportAllImages := params.ReportAllImages
	if !reportAllImages {
		distribution := report.GetDistribution(clientset)
		if distribution == reporttypes.EmbeddedCluster {
			reportAllImages = true
			log.Println("Detected Embedded Cluster installation, enabling reportAllImages")
		}
	}

	verifiedWrapper, err := loadAndVerifyLicense(params, clientset)
	if err != nil {
		return err
	}

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
		ReadOnlyMode:          params.ReadOnlyMode,
	})

	appStateOperator := appstate.InitOperator(clientset, params.Namespace)
	appStateOperator.Start()

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

	return nil
}

// loadAndVerifyLicense loads the license from chart-embedded bytes
// (production) or from the upstream Vendor Portal by integration-license
// ID with cache fallback, then signature-verifies it.
//
// Integration mode goes through SyncLicenseByID: a successful upstream
// call writes through to the cache; an upstream failure with a populated
// cache transparently uses the cached license; an upstream failure with
// no cache returns a permanent error so the pod refuses to start with no
// usable license.
func loadAndVerifyLicense(params APIServerParams, clientset kubernetes.Interface) (licensewrapper.LicenseWrapper, error) {
	var unverifiedWrapper licensewrapper.LicenseWrapper

	switch {
	case len(params.LicenseBytes) > 0:
		wrapper, err := sdklicense.LoadLicenseFromBytes(params.LicenseBytes)
		if err != nil {
			return licensewrapper.LicenseWrapper{}, backoff.Permanent(errors.Wrap(err, "failed to parse license from base64"))
		}
		unverifiedWrapper = wrapper
	case params.IntegrationLicenseID != "":
		ctx := params.Context
		if ctx == nil {
			ctx = context.Background()
		}
		data, _, err := sdklicense.SyncLicenseByID(ctx, clientset, params.Namespace, params.IntegrationLicenseID, params.ReplicatedAppEndpoint)
		if err != nil {
			return licensewrapper.LicenseWrapper{}, backoff.Permanent(errors.Wrap(err, "integration mode requires either reachable upstream or a previously-cached license; neither was available"))
		}
		if data.License.GetLicenseType() != "dev" {
			return licensewrapper.LicenseWrapper{}, backoff.Permanent(errors.New("integration license must be a dev license"))
		}
		unverifiedWrapper = data.License
	default:
		return licensewrapper.LicenseWrapper{}, backoff.Permanent(errors.New("no license source configured: either LicenseBytes or IntegrationLicenseID is required"))
	}

	if err := unverifiedWrapper.VerifySignature(); err != nil {
		if licensewrappertypes.IsLicenseDataValidationError(err) {
			// Non-fatal: license data outside the signature was changed,
			// but the data inside the signature was still valid; the
			// wrapper has been updated to use that data instead.
			log.Println(err.Error())
		} else {
			return licensewrapper.LicenseWrapper{}, backoff.Permanent(errors.Wrap(err, "failed to verify license signature"))
		}
	}

	return unverifiedWrapper, nil
}

// bootstrapBackground performs upstream-dependent bootstrap work whose
// failure must not prevent the SDK from being marked Ready in the default
// configuration. The caller decides how to interpret returned errors:
//
//   - In default (resilient) mode, errors are logged and ignored; handlers
//     continue serving from whatever bootstrapCritical placed in the store.
//   - With requireUpstreamOnStartup=true, the caller treats any error here
//     as fatal and the pod will not be marked Ready.
//
// Upstream calls go through the cache-aware Sync* wrappers so a successful
// refresh writes through to the cache and an upstream failure transparently
// falls back to cached data when available. The in-memory store ends up
// populated either way.
func bootstrapBackground(params APIServerParams) error {
	clientset, err := k8sutil.GetClientset()
	if err != nil {
		return errors.Wrap(err, "failed to get clientset")
	}

	ctx := params.Context
	if ctx == nil {
		ctx = context.Background()
	}

	if !util.IsAirgap() {
		licenseData, _, err := sdklicense.SyncLatestLicense(ctx, clientset, params.Namespace, store.GetStore().GetLicense(), params.ReplicatedAppEndpoint)
		if err != nil {
			return errors.Wrap(err, "failed to get latest license")
		}
		store.GetStore().SetLicense(licenseData.License)
	}

	isIntegrationModeEnabled, err := integration.IsEnabled(ctx, clientset, store.GetStore().GetNamespace(), store.GetStore().GetLicense())
	if err != nil {
		return errors.Wrap(err, "failed to check if integration mode is enabled")
	}

	if !util.IsAirgap() && !isIntegrationModeEnabled {
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

	if err := heartbeat.Start(); err != nil {
		return errors.Wrap(err, "failed to start heartbeat")
	}

	if !util.IsAirgap() && store.GetStore().IsDevLicense() {
		go func() {
			if err := util.WarnOnOutdatedReplicatedVersion(); err != nil {
				logger.Infof("Failed to check if running an outdated replicated version: %v", err)
			}
		}()
	}

	return nil
}
