package handlers

import (
	"net/http"
	"os"
	"strconv"

	"github.com/pkg/errors"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

type GetCurrentAppInfoResponse struct {
	AppSlug              string `json:"appSlug"`
	AppName              string `json:"appName"`
	VersionLabel         string `json:"versionLabel"`
	ChannelID            string `json:"channelId"`
	ChannelName          string `json:"channelName"`
	ChannelSequence      int64  `json:"channelSequence"`
	ReleaseSequence      int64  `json:"releaseSequence"`
	HelmReleaseName      string `json:"helmReleaseName,omitempty"`
	HelmReleaseRevision  int64  `json:"helmReleaseRevision,omitempty"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace,omitempty"`
}

func GetCurrentAppInfo(w http.ResponseWriter, r *http.Request) {
	var helmReleaseRevision int64
	if os.Getenv("HELM_RELEASE_REVISION") != "" {
		hr, err := strconv.ParseInt(os.Getenv("HELM_RELEASE_REVISION"), 10, 64)
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to parse helm revision"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		helmReleaseRevision = hr
	}

	response := GetCurrentAppInfoResponse{
		AppSlug:              store.GetStore().GetAppSlug(),
		AppName:              store.GetStore().GetAppName(),
		VersionLabel:         store.GetStore().GetVersionLabel(),
		ChannelID:            store.GetStore().GetChannelID(),
		ChannelName:          store.GetStore().GetChannelName(),
		ChannelSequence:      store.GetStore().GetChannelSequence(),
		ReleaseSequence:      store.GetStore().GetReleaseSequence(),
		HelmReleaseName:      os.Getenv("HELM_RELEASE_NAME"),
		HelmReleaseRevision:  helmReleaseRevision,
		HelmReleaseNamespace: os.Getenv("HELM_RELEASE_NAMESPACE"),
	}

	JSON(w, http.StatusOK, response)
}

func GetAppUpdates(w http.ResponseWriter, r *http.Request) {
	license := store.GetStore().GetLicense()

	if !util.IsAirgap() {
		licenseData, err := sdklicense.GetLatestLicense(store.GetStore().GetLicense())
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		license = licenseData.License

		// update the store
		store.GetStore().SetLicense(license)
	}

	currentCursor := upstreamtypes.ReplicatedCursor{
		ChannelID:       store.GetStore().GetChannelID(),
		ChannelName:     store.GetStore().GetChannelName(),
		ChannelSequence: store.GetStore().GetChannelSequence(),
	}
	updates, err := upstream.ListPendingChannelReleases(license, currentCursor)
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to list pending channel releases"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	JSON(w, http.StatusOK, updates)
}
