package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/config"
	"github.com/replicatedhq/replicated-sdk/pkg/helm"
	"github.com/replicatedhq/replicated-sdk/pkg/integration"
	integrationtypes "github.com/replicatedhq/replicated-sdk/pkg/integration/types"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/meta"
	"github.com/replicatedhq/replicated-sdk/pkg/meta/types"
	"github.com/replicatedhq/replicated-sdk/pkg/report"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
	helmrelease "helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

type GetCurrentAppInfoResponse struct {
	InstanceID      string              `json:"instanceID"`
	AppSlug         string              `json:"appSlug"`
	AppName         string              `json:"appName"`
	AppStatus       appstatetypes.State `json:"appStatus"`
	HelmChartURL    string              `json:"helmChartURL,omitempty"`
	CurrentRelease  AppRelease          `json:"currentRelease"`
	ChannelID       string              `json:"channelID"`
	ChannelName     string              `json:"channelName"`
	ChannelSequence int64               `json:"channelSequence"`
	ReleaseSequence int64               `json:"releaseSequence"`
}

type GetCurrentAppStatusResponse struct {
	AppStatus appstatetypes.AppStatus `json:"appStatus"`
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

type SendCustomAppMetricsRequest struct {
	Data CustomAppMetricsData `json:"data"`
}

type CustomAppMetricsData map[string]interface{}

type SendAppInstanceTagsRequest struct {
	Data types.InstanceTagData `json:"data"`
}

func GetCurrentAppInfo(w http.ResponseWriter, r *http.Request) {
	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get clientset"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	isIntegrationModeEnabled, err := integration.IsEnabled(r.Context(), clientset, store.GetStore().GetNamespace(), store.GetStore().GetLicense())
	if err != nil {
		logger.Errorf("failed to check if integration mode is enabled: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isIntegrationModeEnabled {
		mockData, err := integration.GetMockData(r.Context(), clientset, store.GetStore().GetNamespace())
		if err != nil {
			logger.Errorf("failed to get mock data: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := GetCurrentAppInfoResponse{
			InstanceID: store.GetStore().GetAppID(),
			AppSlug:    store.GetStore().GetAppSlug(),
			AppName:    store.GetStore().GetAppName(),
		}

		switch mockData := mockData.(type) {
		case *integrationtypes.MockDataV1:
			response.AppStatus = mockData.AppStatus
			response.HelmChartURL = mockData.HelmChartURL
			if mockData.CurrentRelease != nil {
				response.CurrentRelease = mockReleaseToAppRelease(*mockData.CurrentRelease)
			}
			response.ChannelID = store.GetStore().GetChannelID()
			response.ChannelName = store.GetStore().GetChannelName()
			response.ChannelSequence = store.GetStore().GetChannelSequence()
			response.ReleaseSequence = store.GetStore().GetReleaseSequence()

		case *integrationtypes.MockDataV2:
			response.AppStatus = mockData.AppStatus.State
			response.HelmChartURL = mockData.HelmChartURL
			if mockData.CurrentRelease != nil {
				response.CurrentRelease = mockReleaseToAppRelease(*mockData.CurrentRelease)
				response.ChannelID = mockData.CurrentRelease.ChannelID
				response.ChannelName = mockData.CurrentRelease.ChannelName
				response.ChannelSequence = mockData.CurrentRelease.ChannelSequence
				response.ReleaseSequence = mockData.CurrentRelease.ReleaseSequence
			}
		default:
			logger.Errorf("unknown mock data type: %T", mockData)
		}

		w.Header().Set(MockDataHeader, "true")

		JSON(w, http.StatusOK, response)
		return
	}

	response := GetCurrentAppInfoResponse{
		InstanceID:   store.GetStore().GetAppID(),
		AppSlug:      store.GetStore().GetAppSlug(),
		AppName:      store.GetStore().GetAppName(),
		AppStatus:    store.GetStore().GetAppStatus().State,
		HelmChartURL: helm.GetParentChartURL(),
		CurrentRelease: AppRelease{
			VersionLabel: store.GetStore().GetVersionLabel(),
			CreatedAt:    store.GetStore().GetReleaseCreatedAt(),
			ReleaseNotes: store.GetStore().GetReleaseNotes(),
		},
		ChannelID:       store.GetStore().GetChannelID(),
		ChannelName:     store.GetStore().GetChannelName(),
		ChannelSequence: store.GetStore().GetChannelSequence(),
		ReleaseSequence: store.GetStore().GetReleaseSequence(),
	}

	if helm.IsHelmManaged() {
		helmRelease, err := helm.GetRelease(helm.GetReleaseName())
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get helm release"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if helmRelease != nil {
			response.CurrentRelease.HelmReleaseName = helmRelease.Name
			response.CurrentRelease.HelmReleaseRevision = helmRelease.Version
			response.CurrentRelease.HelmReleaseNamespace = helmRelease.Namespace
			response.CurrentRelease.DeployedAt = helmRelease.Info.LastDeployed.Format(time.RFC3339)
		}
	}

	JSON(w, http.StatusOK, response)
}

func GetCurrentAppStatus(w http.ResponseWriter, r *http.Request) {
	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get clientset"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	isIntegrationModeEnabled, err := integration.IsEnabled(r.Context(), clientset, store.GetStore().GetNamespace(), store.GetStore().GetLicense())
	if err != nil {
		logger.Errorf("failed to check if integration mode is enabled: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isIntegrationModeEnabled {
		response := GetCurrentAppStatusResponse{}

		mockData, err := integration.GetMockData(r.Context(), clientset, store.GetStore().GetNamespace())
		if err != nil {
			logger.Errorf("failed to get mock data: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		switch mockData := mockData.(type) {
		case *integrationtypes.MockDataV1:
			logger.Errorf("app status is not supported in v1 mock data")
			w.WriteHeader(http.StatusInternalServerError)
			return
		case *integrationtypes.MockDataV2:
			response.AppStatus = mockData.AppStatus
		default:
			logger.Errorf("unknown mock data type: %T", mockData)
		}

		w.Header().Set(MockDataHeader, "true")

		JSON(w, http.StatusOK, response)
		return
	}

	response := GetCurrentAppStatusResponse{
		AppStatus: store.GetStore().GetAppStatus(),
	}

	JSON(w, http.StatusOK, response)
}

func GetAppUpdates(w http.ResponseWriter, r *http.Request) {
	if util.IsAirgap() {
		JSON(w, http.StatusOK, []upstreamtypes.ChannelRelease{})
		return
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get clientset"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	isIntegrationModeEnabled, err := integration.IsEnabled(r.Context(), clientset, store.GetStore().GetNamespace(), store.GetStore().GetLicense())
	if err != nil {
		logger.Errorf("failed to check if integration mode is enabled: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isIntegrationModeEnabled {
		mockData, err := integration.GetMockData(r.Context(), clientset, store.GetStore().GetNamespace())
		if err != nil {
			logger.Errorf("failed to get mock data: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := []upstreamtypes.ChannelRelease{}
		var avalableReleases []integrationtypes.MockRelease

		switch mockData := mockData.(type) {
		case *integrationtypes.MockDataV1:
			avalableReleases = mockData.AvailableReleases
		case *integrationtypes.MockDataV2:
			avalableReleases = mockData.AvailableReleases
		default:
			logger.Errorf("unknown mock data type: %T", mockData)
		}

		for _, mockRelease := range avalableReleases {
			response = append(response, upstreamtypes.ChannelRelease{
				VersionLabel: mockRelease.VersionLabel,
				CreatedAt:    mockRelease.CreatedAt,
				ReleaseNotes: mockRelease.ReleaseNotes,
			})
		}

		w.Header().Set(MockDataHeader, "true")

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
	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get clientset"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	isIntegrationModeEnabled, err := integration.IsEnabled(r.Context(), clientset, store.GetStore().GetNamespace(), store.GetStore().GetLicense())
	if err != nil {
		logger.Errorf("failed to check if integration mode is enabled: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isIntegrationModeEnabled {
		mockData, err := integration.GetMockData(r.Context(), clientset, store.GetStore().GetNamespace())
		if err != nil {
			logger.Errorf("failed to get mock data: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var deployedReleases []integrationtypes.MockRelease

		switch mockData := mockData.(type) {
		case *integrationtypes.MockDataV1:
			deployedReleases = mockData.DeployedReleases
		case *integrationtypes.MockDataV2:
			deployedReleases = mockData.DeployedReleases
		default:
			logger.Errorf("unknown mock data type: %T", mockData)
		}

		response := GetAppHistoryResponse{
			Releases: []AppRelease{},
		}
		for _, mockRelease := range deployedReleases {
			response.Releases = append(response.Releases, mockReleaseToAppRelease(mockRelease))
		}

		w.Header().Set(MockDataHeader, "true")

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
		if unstructured.GetName() != util.GetReplicatedSecretName() {
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

func mockReleaseToAppRelease(mockRelease integrationtypes.MockRelease) AppRelease {
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

var testClientSet kubernetes.Interface

func SetTestClientSet(clientset kubernetes.Interface) {
	testClientSet = clientset
}

func SendCustomAppMetrics(w http.ResponseWriter, r *http.Request) {
	request := SendCustomAppMetricsRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logger.Error(errors.Wrap(err, "decode request"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := validateCustomAppMetricsData(request.Data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	var clientset kubernetes.Interface
	if testClientSet != nil {
		clientset = testClientSet
	} else {
		var err error
		clientset, err = k8sutil.GetClientset()
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get clientset"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	overwrite := true
	if r.Method == http.MethodPatch {
		overwrite = false
	}

	if err := report.SendCustomAppMetrics(clientset, store.GetStore(), request.Data, overwrite); err != nil {
		logger.Error(errors.Wrap(err, "set application data"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	JSON(w, http.StatusOK, "")
}

func DeleteCustomAppMetricsKey(w http.ResponseWriter, r *http.Request) {
	key, ok := mux.Vars(r)["key"]

	if !ok || key == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "key cannot be empty")
		logger.Errorf("cannot delete custom metrics key - key cannot be empty")
		return
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get clientset"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{key: nil}

	if err := report.SendCustomAppMetrics(clientset, store.GetStore(), data, false); err != nil {
		logger.Error(errors.Wrapf(err, "failed to delete custom merics key: %s", key))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	JSON(w, http.StatusNoContent, "")
}

func validateCustomAppMetricsData(data CustomAppMetricsData) error {
	if len(data) == 0 {
		return errors.New("no data provided")
	}

	for key, val := range data {
		valType := reflect.TypeOf(val)
		if valType == nil {
			return errors.Errorf("%s value is nil, only scalar values are allowed", key)
		}

		switch valType.Kind() {
		case reflect.Slice:
			return errors.Errorf("%s value is an array, only scalar values are allowed", key)
		case reflect.Array:
			return errors.Errorf("%s value is an array, only scalar values are allowed", key)
		case reflect.Map:
			return errors.Errorf("%s value is a map, only scalar values are allowed", key)
		}
	}

	return nil
}

func SendAppInstanceTags(w http.ResponseWriter, r *http.Request) {
	request := SendAppInstanceTagsRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		t, ok := err.(*json.UnmarshalTypeError)
		if ok {
			logger.Errorf("failed to decode instance-tag request: %s value is not a string", t.Field)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "%v not supported, only string values are allowed on instance-tags", t.Value)
			return
		}

		logger.Error(errors.Wrap(err, "decode request"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get clientset"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := meta.SaveInstanceTag(r.Context(), clientset, store.GetStore().GetNamespace(), request.Data); err != nil {
		logger.Errorf("failed to save instance tags: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := report.SendInstanceData(clientset, store.GetStore()); err != nil {
		logger.Errorf("failed to send instance data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	JSON(w, http.StatusOK, "")
}
