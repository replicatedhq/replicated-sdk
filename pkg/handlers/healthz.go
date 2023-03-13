package handlers

import (
	"net/http"

	"github.com/replicatedhq/kots-sdk/pkg/buildversion"
)

type HealthzResponse struct {
	Version string `json:"version"`
}

func Healthz(w http.ResponseWriter, r *http.Request) {

	healthzResponse := HealthzResponse{
		Version: buildversion.Version(),
	}

	JSON(w, http.StatusOK, healthzResponse)
}
