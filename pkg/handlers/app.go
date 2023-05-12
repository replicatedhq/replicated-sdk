package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/helm"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	helmrelease "helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
)

type GetCurrentAppInfoResponse struct {
	AppSlug        string     `json:"appSlug"`
	AppName        string     `json:"appName"`
	CurrentRelease AppRelease `json:"currentRelease"`
}

type GetAppHistoryResponse struct {
	Releases []AppRelease `json:"releases"`
}

type AppRelease struct {
	VersionLabel         string `json:"versionLabel"`
	ChannelID            string `json:"channelID"`
	ChannelName          string `json:"channelName"`
	ChannelSequence      int64  `json:"channelSequence"`
	ReleaseSequence      int64  `json:"releaseSequence"`
	IsRequired           bool   `json:"isRequired"`
	CreatedAt            string `json:"createdAt"`
	ReleaseNotes         string `json:"releaseNotes"`
	HelmReleaseName      string `json:"helmReleaseName,omitempty"`
	HelmReleaseRevision  int    `json:"helmReleaseRevision,omitempty"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace,omitempty"`
}

func GetCurrentAppInfo(w http.ResponseWriter, r *http.Request) {
	response := GetCurrentAppInfoResponse{
		AppSlug: store.GetStore().GetAppSlug(),
		AppName: store.GetStore().GetAppName(),
		CurrentRelease: AppRelease{
			VersionLabel:         store.GetStore().GetVersionLabel(),
			ChannelID:            store.GetStore().GetChannelID(),
			ChannelName:          store.GetStore().GetChannelName(),
			ChannelSequence:      store.GetStore().GetChannelSequence(),
			ReleaseSequence:      store.GetStore().GetReleaseSequence(),
			IsRequired:           store.GetStore().GetReleaseIsRequired(),
			CreatedAt:            store.GetStore().GetReleaseCreatedAt(),
			ReleaseNotes:         store.GetStore().GetReleaseNotes(),
			HelmReleaseName:      helm.GetReleaseName(),
			HelmReleaseRevision:  helm.GetReleaseRevision(),
			HelmReleaseNamespace: helm.GetReleaseNamespace(),
		},
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

func GetAppHistory(w http.ResponseWriter, r *http.Request) {
	if !helm.IsHelmManaged() {
		err := errors.New("app history is only available in Helm mode")
		logger.Error(err)
		JSON(w, http.StatusBadRequest, err)
		return
	}

	helmHistory, err := helm.GetReleaseHistory()
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to list helm releases"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// sort in descending order
	sort.Slice(helmHistory, func(i, j int) bool {
		return helmHistory[i].Version > helmHistory[j].Version
	})

	response := GetAppHistoryResponse{
		Releases: []AppRelease{},
	}
	for _, helmRelease := range helmHistory {
		appRelease := helmReleaseToAppRelease(helmRelease)
		if appRelease != nil {
			response.Releases = append(response.Releases, *appRelease)
		}
	}

	JSON(w, http.StatusOK, response)
}

func helmReleaseToAppRelease(helmRelease *helmrelease.Release) *AppRelease {
	// find the replicated secret in the helm release and get the info from it
	for _, doc := range strings.Split(helmRelease.Manifest, "\n---\n") {
		if doc == "" {
			continue
		}

		unstructured := &unstructured.Unstructured{}
		_, gvk, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(doc), nil, unstructured)
		if err != nil {
			logger.Infof("error decoding document: %v", err.Error())
			continue
		}

		if gvk.Group != "" || gvk.Version != "v1" || gvk.Kind != "Secret" {
			continue
		}
		if unstructured.GetName() != "replicated" {
			continue
		}

		data, ok := unstructured.Object["stringData"].(map[string]interface{})
		if !ok {
			// try data
			data, ok = unstructured.Object["data"].(map[string]interface{})
			if !ok {
				return nil
			}
		}

		appRelease := &AppRelease{
			HelmReleaseName:      helmRelease.Name,
			HelmReleaseRevision:  helmRelease.Version,
			HelmReleaseNamespace: helmRelease.Namespace,
		}

		appRelease.ChannelID = data["REPLICATED_CHANNEL_ID"].(string)
		appRelease.ChannelName = data["REPLICATED_CHANNEL_NAME"].(string)
		appRelease.ChannelSequence, _ = strconv.ParseInt(data["REPLICATED_CHANNEL_SEQUENCE"].(string), 10, 64)
		appRelease.ReleaseSequence, _ = strconv.ParseInt(data["REPLICATED_RELEASE_SEQUENCE"].(string), 10, 64)
		appRelease.IsRequired, _ = strconv.ParseBool(data["REPLICATED_RELEASE_IS_REQUIRED"].(string))
		appRelease.CreatedAt = data["REPLICATED_RELEASE_CREATED_AT"].(string)
		appRelease.ReleaseNotes = data["REPLICATED_RELEASE_NOTES"].(string)
		appRelease.VersionLabel = data["REPLICATED_VERSION_LABEL"].(string)

		return appRelease
	}

	logger.Debugf("replicated secret not found in helm release %s revision %d", helmRelease.Name, helmRelease.Version)

	return nil
}
