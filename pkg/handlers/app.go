package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/helm"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/mock"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
	types "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	helmrelease "helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
)

type GetCurrentAppInfoResponse struct {
	AppSlug        string     `json:"appSlug"`
	AppName        string     `json:"appName"`
	HelmChartURL   string     `json:"helmChartURL,omitempty"`
	CurrentRelease AppRelease `json:"currentRelease"`
}

type GetAppHistoryResponse struct {
	Releases []AppRelease `json:"releases"`
}

type AppRelease struct {
	VersionLabel         string `json:"versionLabel"`
	ChannelID            string `json:"channelID"`
	ChannelName          string `json:"channelName"`
	IsRequired           bool   `json:"isRequired"`
	CreatedAt            string `json:"createdAt"`
	ReleaseNotes         string `json:"releaseNotes"`
	HelmReleaseName      string `json:"helmReleaseName,omitempty"`
	HelmReleaseRevision  int    `json:"helmReleaseRevision,omitempty"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace,omitempty"`
}

func GetCurrentAppInfo(w http.ResponseWriter, r *http.Request) {
	if store.GetStore().IsDevLicense() {
		hasMocks, err := mock.MustGetMock().HasMocks()
		if err != nil {
			logger.Errorf("failed to check if mocks exist: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if hasMocks {
			mockCurrentRelease, err := mock.MustGetMock().GetCurrentRelease()
			if err != nil {
				logger.Errorf("failed to get mock current release: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			response := GetCurrentAppInfoResponse{
				AppSlug: store.GetStore().GetAppSlug(),
				AppName: store.GetStore().GetAppName(),
			}

			if mockCurrentRelease != nil {
				response.CurrentRelease = mockReleaseToAppRelease(*mockCurrentRelease)
			}

			JSON(w, http.StatusOK, response)
			return
		}
	}

	response := GetCurrentAppInfoResponse{
		AppSlug:      store.GetStore().GetAppSlug(),
		AppName:      store.GetStore().GetAppName(),
		HelmChartURL: helm.GetParentChartURL(),
		CurrentRelease: AppRelease{
			VersionLabel:         store.GetStore().GetVersionLabel(),
			ChannelID:            store.GetStore().GetChannelID(),
			ChannelName:          store.GetStore().GetChannelName(),
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
	if store.GetStore().IsDevLicense() {
		hasMocks, err := mock.MustGetMock().HasMocks()
		if err != nil {
			logger.Errorf("failed to check if mocks exist: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if hasMocks {
			mockAvailableReleases, err := mock.MustGetMock().GetAvailableReleases()
			if err != nil {
				logger.Errorf("failed to get available mock releases: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			response := []types.ChannelRelease{}
			for _, mockRelease := range mockAvailableReleases {
				response = append(response, types.ChannelRelease{
					VersionLabel: mockRelease.VersionLabel,
					IsRequired:   mockRelease.IsRequired,
					CreatedAt:    time.Now().Format(time.RFC3339),
					ReleaseNotes: mockRelease.ReleaseNotes,
				})
			}

			JSON(w, http.StatusOK, response)
			return
		}
	}

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
	if store.GetStore().IsDevLicense() {
		hasMocks, err := mock.MustGetMock().HasMocks()
		if err != nil {
			logger.Errorf("failed to check if mocks exist: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if hasMocks {
			mockReleases, err := mock.MustGetMock().GetDeployedReleases()
			if err != nil {
				logger.Errorf("failed to get mock releases: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			response := GetAppHistoryResponse{
				Releases: []AppRelease{},
			}
			for _, mockRelease := range mockReleases {
				appRelease := mockReleaseToAppRelease(mockRelease)
				response.Releases = append(response.Releases, appRelease)
			}

			JSON(w, http.StatusOK, response)
			return
		}
	}

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
			return nil
		}

		appRelease := &AppRelease{
			HelmReleaseName:      helmRelease.Name,
			HelmReleaseRevision:  helmRelease.Version,
			HelmReleaseNamespace: helmRelease.Namespace,
		}

		appRelease.ChannelID = data["REPLICATED_CHANNEL_ID"].(string)
		appRelease.ChannelName = data["REPLICATED_CHANNEL_NAME"].(string)
		appRelease.IsRequired, _ = strconv.ParseBool(data["REPLICATED_RELEASE_IS_REQUIRED"].(string))
		appRelease.CreatedAt = data["REPLICATED_RELEASE_CREATED_AT"].(string)
		appRelease.ReleaseNotes = data["REPLICATED_RELEASE_NOTES"].(string)
		appRelease.VersionLabel = data["REPLICATED_VERSION_LABEL"].(string)

		return appRelease
	}

	logger.Debugf("replicated secret not found in helm release %s revision %d", helmRelease.Name, helmRelease.Version)

	return nil
}

func mockReleaseToAppRelease(mockRelease mock.MockRelease) AppRelease {
	appRelease := AppRelease{
		VersionLabel: mockRelease.VersionLabel,
		ChannelID:    mockRelease.ChannelID,
		ChannelName:  mockRelease.ChannelName,
		IsRequired:   mockRelease.IsRequired,
		CreatedAt:    time.Now().Format(time.RFC3339),
		ReleaseNotes: mockRelease.ReleaseNotes,
	}

	if helm.IsHelmManaged() {
		appRelease.HelmReleaseName = helm.GetReleaseName()
		appRelease.HelmReleaseRevision = helm.GetReleaseRevision()
		appRelease.HelmReleaseNamespace = helm.GetReleaseNamespace()
	}

	return appRelease
}
