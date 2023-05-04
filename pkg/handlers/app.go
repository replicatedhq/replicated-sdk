package handlers

import (
	"net/http"

	"github.com/pkg/errors"
	sdklicense "github.com/replicatedhq/kots-sdk/pkg/license"
	"github.com/replicatedhq/kots-sdk/pkg/logger"
	"github.com/replicatedhq/kots-sdk/pkg/store"
	"github.com/replicatedhq/kots-sdk/pkg/upstream"
	upstreamtypes "github.com/replicatedhq/kots-sdk/pkg/upstream/types"
	"github.com/replicatedhq/kots-sdk/pkg/util"
)

type GetCurrentAppInfoResponse struct {
	AppSlug         string `json:"appSlug"`
	VersionLabel    string `json:"versionLabel"`
	ChannelID       string `json:"channelId"`
	ChannelName     string `json:"channelName"`
	ChannelSequence int64  `json:"channelSequence"`
	ReleaseSequence int64  `json:"releaseSequence"`
}

func GetCurrentAppInfo(w http.ResponseWriter, r *http.Request) {
	response := GetCurrentAppInfoResponse{
		AppSlug:         store.GetStore().GetAppSlug(),
		VersionLabel:    store.GetStore().GetVersionLabel(),
		ChannelID:       store.GetStore().GetChannelID(),
		ChannelName:     store.GetStore().GetChannelName(),
		ChannelSequence: store.GetStore().GetChannelSequence(),
		ReleaseSequence: store.GetStore().GetReleaseSequence(),
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
