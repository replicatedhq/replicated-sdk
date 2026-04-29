package apiserver

import (
	"context"
	stderrors "errors"
	"log"
	"sync"

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

// bootstrapCritical performs the bootstrap work that must succeed before the
// SDK can serve meaningful responses. It loads, signature-verifies, and
// expiry-checks the license, then populates the in-memory store. It also
// initializes the local appstate operator so app/* endpoints can begin
// reporting status as soon as the pod is Ready.
//
// In production mode (LicenseBytes provided by the chart) this path is
// fully local — no upstream call is made. In integration mode the
// license document is fetched from the Vendor Portal by ID, so an
// unreachable upstream on first boot causes critical to retry. The
// devOffline opt-in (production mode + a dev license) flips the runtime
// airgap flag here so subsequent !util.IsAirgap() gates skip their
// upstream calls; it does not change the license source.
//
// Permanent failures (license parse error, signature invalid, expired,
// devOffline + non-dev license) are returned as backoff.Permanent so the
// retry loop above gives up immediately. Transient failures bubble up
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

	verifiedWrapper, err := loadAndVerifyLicense(params)
	if err != nil {
		return err
	}

	// Expiry is unconditional and has no side effects, so check it
	// before applyDevOfflineGuard — which flips the process-global
	// airgap override on success. Reordering keeps that side effect
	// from landing on a license we are about to reject anyway.
	expired, err := sdklicense.LicenseIsExpired(verifiedWrapper)
	if err != nil {
		return errors.Wrap(err, "failed to check if license is expired")
	}
	if expired {
		return backoff.Permanent(errors.New("License is expired"))
	}

	if err := applyDevOfflineGuard(verifiedWrapper, params.DevOffline); err != nil {
		return err
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

	// Resolve informers BEFORE starting the appstate operator goroutine
	// so a helm.GetRelease error on this attempt doesn't leave a
	// runAppStateMonitor goroutine running that the next retry would
	// duplicate. InitOperator overwrites the package-level `operator`
	// pointer and Operator.Start unconditionally spawns a new goroutine
	// without shutting down the previous monitor, so the start must be
	// the last fallible-or-not step in the function.
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

	appStateOperator := appstate.InitOperator(clientset, params.Namespace)
	appStateOperator.Start()
	appStateOperator.ApplyAppInformers(appstatetypes.AppInformersArgs{
		AppSlug:   store.GetStore().GetAppSlug(),
		Sequence:  store.GetStore().GetReleaseSequence(),
		Informers: informers,
	})

	return nil
}

// loadAndVerifyLicense loads the license from chart-embedded bytes
// (production) or from the upstream Vendor Portal by integration-license
// ID, then signature-verifies it.
func loadAndVerifyLicense(params APIServerParams) (licensewrapper.LicenseWrapper, error) {
	var unverifiedWrapper licensewrapper.LicenseWrapper

	switch {
	case len(params.LicenseBytes) > 0:
		wrapper, err := sdklicense.LoadLicenseFromBytes(params.LicenseBytes)
		if err != nil {
			return licensewrapper.LicenseWrapper{}, backoff.Permanent(errors.Wrap(err, "failed to parse license from base64"))
		}
		unverifiedWrapper = wrapper
	case params.IntegrationLicenseID != "":
		wrapper, err := sdklicense.GetLicenseByID(params.IntegrationLicenseID, params.ReplicatedAppEndpoint)
		if err != nil {
			return licensewrapper.LicenseWrapper{}, errors.Wrap(err, "failed to get license by id")
		}
		if wrapper.GetLicenseType() != "dev" {
			return licensewrapper.LicenseWrapper{}, backoff.Permanent(errors.New("integration license must be a dev license"))
		}
		unverifiedWrapper = wrapper
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

// applyDevOfflineGuard enforces the dev-only contract of the devOffline
// opt-in: when the operator sets replicated.devOffline=true, the loaded
// license must be a dev license, and on success the runtime airgap flag
// is flipped so all !util.IsAirgap() gates skip their upstream calls.
//
// A non-dev license with devOffline=true returns backoff.Permanent so
// the bootstrap retry loop gives up immediately — the install is
// misconfigured and no amount of retrying will fix it. Production
// licenses cannot silently end up running offline this way; operators
// who want true airgap must use the existing isAirgap chart value with
// its accompanying ops requirements.
func applyDevOfflineGuard(wrapper licensewrapper.LicenseWrapper, devOffline bool) error {
	if !devOffline {
		return nil
	}
	if wrapper.GetLicenseType() != "dev" {
		return backoff.Permanent(errors.New("devOffline=true requires a dev license"))
	}
	util.SetAirgapOverride(true)
	log.Println("devOffline enabled: SDK will not call replicated.app for this install")
	return nil
}

// bootstrapBackground performs upstream-dependent bootstrap work whose
// failure must not prevent the SDK from being marked Ready. Errors are
// logged and the call is retried by the caller; handlers continue serving
// from whatever bootstrapCritical placed in the store.
//
// Each step runs independently and accumulates its error rather than
// returning early. This guarantees that a transient failure in one step
// (e.g. GetLatestLicense when upstream is briefly unreachable) does not
// silently disable downstream steps for the pod's lifetime — most
// importantly, heartbeat.Start() always gets a chance to run so the
// instance continues to check in. The steps here are safe to retry
// (heartbeat.Start clears and re-adds its cron entries; Set* store writes
// are last-write-wins). Critical-phase initializers that would leak
// resources on retry (notably appstate.InitOperator) live in
// bootstrapCritical and are retried by their own loop, which terminates
// on the first success.
func bootstrapBackground(params APIServerParams) error {
	ctx := params.Context
	if ctx == nil {
		ctx = context.Background()
	}

	var errs []error

	if !util.IsAirgap() {
		licenseData, err := sdklicense.GetLatestLicense(store.GetStore().GetLicense(), params.ReplicatedAppEndpoint)
		if err != nil {
			errs = append(errs, errors.Wrap(err, "failed to get latest license"))
		} else {
			store.GetStore().SetLicense(licenseData.License)
		}
	}

	// integrationCheckOK distinguishes "we know the mode" from "we don't
	// know yet". The zero value of isIntegrationModeEnabled is false, so
	// without this flag a transient IsEnabled failure would silently fall
	// into the !isIntegrationModeEnabled branch below and call
	// upstream.GetUpdates against the Vendor Portal even when the pod is
	// actually running in integration (dev) mode. We instead skip the
	// updates fetch entirely on this turn and let the heartbeat-driven
	// refresh recover.
	//
	// Clientset is acquired here, scoped to the integration check that
	// needs it. Hoisting GetClientset to the top of the function would
	// gate license sync, heartbeat.Start, and the outdated-version check
	// on a transient k8s API hiccup, contradicting the error-accumulation
	// pattern this function commits to.
	var isIntegrationModeEnabled bool
	integrationCheckOK := false
	clientset, err := k8sutil.GetClientset()
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to get clientset"))
	} else {
		isIntegrationModeEnabled, err = integration.IsEnabled(ctx, clientset, store.GetStore().GetNamespace(), store.GetStore().GetLicense())
		integrationCheckOK = err == nil
		if err != nil {
			errs = append(errs, errors.Wrap(err, "failed to check if integration mode is enabled"))
		}
	}

	if !util.IsAirgap() && integrationCheckOK && !isIntegrationModeEnabled {
		currentCursor := upstreamtypes.ReplicatedCursor{
			ChannelID:       store.GetStore().GetChannelID(),
			ChannelName:     store.GetStore().GetChannelName(),
			ChannelSequence: store.GetStore().GetChannelSequence(),
		}
		updates, err := upstream.GetUpdates(store.GetStore(), store.GetStore().GetLicense(), currentCursor)
		if err != nil {
			errs = append(errs, errors.Wrap(err, "failed to get updates"))
		} else {
			store.GetStore().SetUpdates(updates)
		}
	}

	if err := heartbeat.Start(); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to start heartbeat"))
	}

	// bootstrapBackground is wrapped in a retry loop, so any goroutine
	// launched here would otherwise be re-spawned on every retry and
	// leak. The dev-version check is a one-time observability signal —
	// gate it behind a package-level sync.Once so it runs at most once
	// per process regardless of how many times bootstrapBackground is
	// invoked.
	if !util.IsAirgap() && store.GetStore().IsDevLicense() {
		warnOnOutdatedReplicatedVersionOnce.Do(func() {
			go func() {
				if err := util.WarnOnOutdatedReplicatedVersion(); err != nil {
					logger.Infof("Failed to check if running an outdated replicated version: %v", err)
				}
			}()
		})
	}

	return stderrors.Join(errs...)
}

// warnOnOutdatedReplicatedVersionOnce ensures the dev-mode upstream-version
// warning goroutine is launched at most once per process. bootstrapBackground
// can be invoked multiple times by the retry loop and we don't want to
// spawn a fresh goroutine on every retry.
var warnOnOutdatedReplicatedVersionOnce sync.Once
