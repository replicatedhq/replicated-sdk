package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/integration"
	integrationtypes "github.com/replicatedhq/replicated-sdk/pkg/integration/types"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
)

type GetIntegrationStatusResponse struct {
	IsEnabled bool `json:"isEnabled"`
}

func PostIntegrationMockData(w http.ResponseWriter, r *http.Request) {
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

	if !isIntegrationModeEnabled {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	mockDataRequest := integrationtypes.MockData{}
	if err := json.NewDecoder(r.Body).Decode(&mockDataRequest); err != nil {
		logger.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := integration.SetMockData(r.Context(), clientset, store.GetStore().GetNamespace(), mockDataRequest); err != nil {
		logger.Errorf("failed to update mock data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func GetIntegrationMockData(w http.ResponseWriter, r *http.Request) {
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

	if !isIntegrationModeEnabled {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	mockData, err := integration.GetMockData(r.Context(), clientset, store.GetStore().GetNamespace())
	if err != nil {
		logger.Errorf("failed to get mock data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if mockData == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	JSON(w, http.StatusOK, mockData)
}

func GetIntegrationStatus(w http.ResponseWriter, r *http.Request) {
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

	response := GetIntegrationStatusResponse{
		IsEnabled: isIntegrationModeEnabled,
	}

	JSON(w, http.StatusOK, response)
}
