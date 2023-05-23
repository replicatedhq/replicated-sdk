package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
)

func RegisterProductionRoutes(r *mux.Router) {
	// license
	r.HandleFunc("/api/v1/license/info", GetLicenseInfo).Methods("GET")
	r.HandleFunc("/api/v1/license/fields", GetLicenseFields).Methods("GET")
	r.HandleFunc("/api/v1/license/fields/{fieldName}", GetLicenseField).Methods("GET")

	// app
	r.HandleFunc("/api/v1/app/info", GetCurrentAppInfo).Methods("GET")
	r.HandleFunc("/api/v1/app/updates", GetAppUpdates).Methods("GET")
	r.HandleFunc("/api/v1/app/history", GetAppHistory).Methods("GET")
}

func JSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		logger.Error(err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
