package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/mock"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
)

func PostMockData(w http.ResponseWriter, r *http.Request) {
	mockDataRequest := mock.MockData{}
	if err := json.NewDecoder(r.Body).Decode(&mockDataRequest); err != nil {
		logger.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := mock.MustGetMock().InsertMockData(mockDataRequest); err != nil {
		logger.Errorf("failed to update mock data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func GetMockData(w http.ResponseWriter, r *http.Request) {
	if !store.GetStore().IsDevLicense() {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	mockData, err := mock.MustGetMock().GetMockData()
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

func DeleteMockData(w http.ResponseWriter, r *http.Request) {
	if err := mock.MustGetMock().DeleteMockData(); err != nil {
		logger.Errorf("failed to delete mock data: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
