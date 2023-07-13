package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/config"
	"github.com/replicatedhq/replicated-sdk/pkg/helm"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/mock"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
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
	ReleaseNotes         string `json:"releaseNotes"`
	CreatedAt            string `json:"createdAt"`
	DeployedAt           string `json:"deployedAt"`
	HelmReleaseName      string `json:"helmReleaseName,omitempty"`
	HelmReleaseRevision  int    `json:"helmReleaseRevision,omitempty"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace,omitempty"`
}

func GetCurrentAppInfo(w http.ResponseWriter, r *http.Request) {
	isMockEnabled, err := mock.MustGetMock().IsMockEnabled(r.Context(), store.GetStore().GetLicense())
	if err != nil {
		logger.Errorf("failed to check if mock is enabled: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isMockEnabled {
		response := GetCurrentAppInfoResponse{
			AppSlug: store.GetStore().GetAppSlug(),
			AppName: store.GetStore().GetAppName(),
		}

		mockCurrentRelease, err := mock.MustGetMock().GetCurrentRelease(r.Context())
		if err != nil {
			logger.Errorf("failed to get mock current release: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if mockCurrentRelease != nil {
			response.CurrentRelease = mockReleaseToAppRelease(*mockCurrentRelease)
		}

		mockHelmChartURL, err := mock.MustGetMock().GetHelmChartURL(r.Context())
		if err != nil {
			logger.Errorf("failed to get mock helm chart url: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		response.HelmChartURL = mockHelmChartURL

		JSON(w, http.StatusOK, response)
		return
	}

	response := GetCurrentAppInfoResponse{
		AppSlug:      store.GetStore().GetAppSlug(),
		AppName:      store.GetStore().GetAppName(),
		HelmChartURL: helm.GetParentChartURL(),
		CurrentRelease: AppRelease{
			VersionLabel: store.GetStore().GetVersionLabel(),
			CreatedAt:    store.GetStore().GetReleaseCreatedAt(),
			ReleaseNotes: store.GetStore().GetReleaseNotes(),
		},
	}

	if helm.IsHelmManaged() {
		helmRelease, err := helm.GetRelease(helm.GetReleaseName())
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get helm release"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response.CurrentRelease.HelmReleaseName = helmRelease.Name
		response.CurrentRelease.HelmReleaseRevision = helmRelease.Version
		response.CurrentRelease.HelmReleaseNamespace = helmRelease.Namespace
		response.CurrentRelease.DeployedAt = helmRelease.Info.LastDeployed.Format(time.RFC3339)
	}

	JSON(w, http.StatusOK, response)
}

func GetAppUpdates(w http.ResponseWriter, r *http.Request) {
	isMockEnabled, err := mock.MustGetMock().IsMockEnabled(r.Context(), store.GetStore().GetLicense())
	if err != nil {
		logger.Errorf("failed to check if mock is enabled: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isMockEnabled {
		mockAvailableReleases, err := mock.MustGetMock().GetAvailableReleases(r.Context())
		if err != nil {
			logger.Errorf("failed to get available mock releases: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := []upstreamtypes.ChannelRelease{}
		for _, mockRelease := range mockAvailableReleases {
			response = append(response, upstreamtypes.ChannelRelease{
				VersionLabel: mockRelease.VersionLabel,
				CreatedAt:    mockRelease.CreatedAt,
				ReleaseNotes: mockRelease.ReleaseNotes,
			})
		}

		JSON(w, http.StatusOK, response)
		return
	}

	license := store.GetStore().GetLicense()
	updates := store.GetStore().GetUpdates()

	licenseData, err := sdklicense.GetLatestLicense(license, store.GetStore().GetReplicatedAppEndpoint())
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get latest license"))
		JSONCached(w, http.StatusOK, updates)
		return
	}

	license = licenseData.License
	store.GetStore().SetLicense(license)

	currentCursor := upstreamtypes.ReplicatedCursor{
		ChannelID:       store.GetStore().GetChannelID(),
		ChannelName:     store.GetStore().GetChannelName(),
		ChannelSequence: store.GetStore().GetChannelSequence(),
	}
	us, err := upstream.GetUpdates(store.GetStore(), license, currentCursor)
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get updates"))
		JSONCached(w, http.StatusOK, updates)
		return
	}

	updates = us
	store.GetStore().SetUpdates(updates)

	JSON(w, http.StatusOK, updates)
}

func GetAppHistory(w http.ResponseWriter, r *http.Request) {
	isMockEnabled, err := mock.MustGetMock().IsMockEnabled(r.Context(), store.GetStore().GetLicense())
	if err != nil {
		logger.Errorf("failed to check if mock is enabled: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isMockEnabled {
		mockReleases, err := mock.MustGetMock().GetDeployedReleases(r.Context())
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
			DeployedAt:           helmRelease.Info.LastDeployed.Format(time.RFC3339),
			HelmReleaseName:      helmRelease.Name,
			HelmReleaseRevision:  helmRelease.Version,
			HelmReleaseNamespace: helmRelease.Namespace,
		}

		configFile, ok := data["config.yaml"]
		if ok {
			replicatedConfig, err := config.ParseReplicatedConfig([]byte(configFile.(string)))
			if err != nil {
				logger.Infof("failed to parse config file: %v", err)
				continue
			}
			appRelease.VersionLabel = replicatedConfig.VersionLabel
			appRelease.ReleaseNotes = replicatedConfig.ReleaseNotes
			appRelease.CreatedAt = replicatedConfig.ReleaseCreatedAt
		}

		return appRelease
	}

	logger.Debugf("replicated secret not found in helm release %s revision %d", helmRelease.Name, helmRelease.Version)

	return nil
}

func mockReleaseToAppRelease(mockRelease mock.MockRelease) AppRelease {
	appRelease := AppRelease{
		VersionLabel:         mockRelease.VersionLabel,
		ReleaseNotes:         mockRelease.ReleaseNotes,
		CreatedAt:            mockRelease.CreatedAt,
		DeployedAt:           mockRelease.DeployedAt,
		HelmReleaseName:      mockRelease.HelmReleaseName,
		HelmReleaseRevision:  mockRelease.HelmReleaseRevision,
		HelmReleaseNamespace: mockRelease.HelmReleaseNamespace,
	}

	return appRelease
}
