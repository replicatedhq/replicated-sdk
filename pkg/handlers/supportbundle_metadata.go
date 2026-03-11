package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/k8sutil"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/supportbundle"
)

type SupportBundleMetadataRequest struct {
	Data map[string]string `json:"data"`
}

func PostSupportBundleMetadata(w http.ResponseWriter, r *http.Request) {
	request := SupportBundleMetadataRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logger.Errorf("failed to decode support bundle metadata request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request body"))
		return
	}

	if request.Data == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("data is required"))
		return
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get clientset"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	namespace := store.GetStore().GetNamespace()

	if err := supportbundle.SaveMetadata(r.Context(), clientset, namespace, request.Data, true); err != nil {
		logger.Errorf("failed to save support bundle metadata: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	JSON(w, http.StatusOK, "")
}

func PatchSupportBundleMetadata(w http.ResponseWriter, r *http.Request) {
	request := SupportBundleMetadataRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logger.Errorf("failed to decode support bundle metadata request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request body"))
		return
	}

	if request.Data == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("data is required"))
		return
	}

	clientset, err := k8sutil.GetClientset()
	if err != nil {
		logger.Error(errors.Wrap(err, "failed to get clientset"))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	namespace := store.GetStore().GetNamespace()

	if err := supportbundle.SaveMetadata(r.Context(), clientset, namespace, request.Data, false); err != nil {
		logger.Errorf("failed to save support bundle metadata: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	JSON(w, http.StatusOK, "")
}
