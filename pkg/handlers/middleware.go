package handlers

import (
	"net/http"

	"github.com/replicatedhq/replicated-sdk/pkg/handlers/types"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
)

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleOptionsRequest(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func EnforceMockAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !store.GetStore().IsDevLicense() {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func RequireValidLicenseIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		licenseID := r.Header.Get("authorization")
		if licenseID == "" {
			response := types.ErrorResponse{Error: "missing authorization header"}
			JSON(w, http.StatusUnauthorized, response)
			return
		}

		if store.GetStore().GetLicense().Spec.LicenseID != licenseID {
			response := types.ErrorResponse{Error: "license ID is not valid"}
			JSON(w, http.StatusUnauthorized, response)
			return
		}

		next.ServeHTTP(w, r)
	})
}
