package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func RegisterDevModeRoutes(r *mux.Router) {
	for _, route := range routeMap {
		routeaPath := route
		r.HandleFunc(routeaPath, func(w http.ResponseWriter, r *http.Request) {
			devModeData, err := getDevModeSecretData()
			if err != nil {
				logger.Errorf("failed to get dev mode secret data: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			mockData := devModeData["REPLICATED_MOCK_DATA"]
			if len(mockData) == 0 {
				// no mock data return 200
				logger.Debug("failed to get dev mode secret data")
				JSON(w, http.StatusOK, nil)
				return
			}

			var mockResponseMap map[string]interface{}
			if err := json.Unmarshal(mockData, &mockResponseMap); err != nil {
				logger.Errorf("failed to unmarshal replicated mock data: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			response := mockResponseMap[routeaPath]
			if response == nil {
				// no mock data return 200
				logger.Debug("failed to get dev mode mock response")
				JSON(w, http.StatusOK, nil)
				return
			}

			JSON(w, http.StatusOK, response)
		})
	}
}

func getDevModeSecretData() (map[string][]byte, error) {
	clientset, err := k8sutil.GetClientset()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get clientset")
	}

	secret, err := clientset.CoreV1().Secrets(store.GetStore().GetNamespace()).Get(context.TODO(), "replicated-dev", metav1.GetOptions{})
	if err != nil && !kuberneteserrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to get secret replicated-dev")
	}

	return secret.Data, nil
}
