package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
)

var (
	routeMap = map[string]string{
		"GetLicenseInfo":    "/api/v1/license/info",
		"GetLicenseFields":  "/api/v1/license/fields",
		"GetLicenseField":   "/api/v1/license/fields/{fieldName}",
		"GetCurrentAppInfo": "/api/v1/app/info",
		"GetAppUpdates":     "/api/v1/app/updates",
		"GetAppHistory":     "/api/v1/app/history",
	}
)

func RegisterProductionRoutes(r *mux.Router) {
	// license
	r.HandleFunc(routeMap["GetLicenseInfo"], GetLicenseInfo).Methods("GET")
	r.HandleFunc(routeMap["GetLicenseFields"], GetLicenseFields).Methods("GET")
	r.HandleFunc(routeMap["GetLicenseField"], GetLicenseField).Methods("GET")

	// app
	r.HandleFunc(routeMap["GetCurrentAppInfo"], GetCurrentAppInfo).Methods("GET")
	r.HandleFunc(routeMap["GetAppUpdates"], GetAppUpdates).Methods("GET")
	r.HandleFunc(routeMap["GetAppHistory"], GetAppHistory).Methods("GET")
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
