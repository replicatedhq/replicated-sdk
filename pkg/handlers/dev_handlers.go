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

func RegisterDeveloperModeRoutes(r *mux.Router) {
	devModeData, err := getDevModeSecretData()
	if err != nil {
		logger.Errorf("failed to get dev data: %v", err)
		return
	}

	mockData := devModeData["REPLICATED_MOCK_DATA"]
	if mockData == nil {
		return
	}

	var mockResponseMap map[string]interface{}
	if err := json.Unmarshal(mockData, &mockResponseMap); err != nil {
		logger.Errorf("failed to unmarshal dev mode routes: %v", err)
		return
	}

	for urlPath, response := range mockResponseMap {
		r.HandleFunc(urlPath, func(w http.ResponseWriter, r *http.Request) {
			resp, err := json.Marshal(response)
			if err != nil {
				logger.Errorf("failed to marshal dev mode response: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(resp)
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
