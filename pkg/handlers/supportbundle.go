package handlers

import (
	"net/http"

	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/upstream"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

type UploadSupportBundleResponse struct {
	BundleID string `json:"bundleId"`
	Slug     string `json:"slug"`
}

func UploadSupportBundle(w http.ResponseWriter, r *http.Request) {
	license := store.GetStore().GetLicense()
	if !license.IsSupportBundleUploadSupported() {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("support bundle upload is not enabled for this license"))
		return
	}

	if util.IsAirgap() {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("support bundle upload is not available in airgap mode"))
		return
	}

	uploadURLResp, err := upstream.GetSupportBundleUploadURL(store.GetStore())
	if err != nil {
		logger.Errorf("failed to get support bundle upload url: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := upstream.UploadToS3(uploadURLResp.UploadURL, r.Body, r.ContentLength, r.Header.Get("Content-Type")); err != nil {
		logger.Errorf("failed to upload support bundle to S3: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slug, err := upstream.MarkSupportBundleUploaded(store.GetStore(), uploadURLResp.BundleID)
	if err != nil {
		logger.Errorf("failed to mark support bundle as uploaded: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	JSON(w, http.StatusCreated, UploadSupportBundleResponse{
		BundleID: uploadURLResp.BundleID,
		Slug:     slug,
	})
}
