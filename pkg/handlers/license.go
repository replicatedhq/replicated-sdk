package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	sdklicense "github.com/replicatedhq/kots-sdk/pkg/license"
	"github.com/replicatedhq/kots-sdk/pkg/logger"
	"github.com/replicatedhq/kots-sdk/pkg/store"
	"github.com/replicatedhq/kots-sdk/pkg/util"
)

func GetLicenseFields(w http.ResponseWriter, r *http.Request) {
	license := store.GetStore().GetLicense()

	if !util.IsAirgap() {
		licenseData, err := sdklicense.GetLatestLicense(license)
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		license = licenseData.License

		// update the store
		store.GetStore().SetLicense(license)
	}

	JSON(w, http.StatusOK, license.Spec.Entitlements)
}

func GetLicenseField(w http.ResponseWriter, r *http.Request) {
	license := store.GetStore().GetLicense()

	if !util.IsAirgap() {
		licenseData, err := sdklicense.GetLatestLicense(license)
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		license = licenseData.License

		// update the store
		store.GetStore().SetLicense(license)
	}

	fieldName := mux.Vars(r)["fieldName"]

	entitlement, ok := license.Spec.Entitlements[fieldName]
	if !ok {
		logger.Error(errors.Errorf("no entitlement named %q found", fieldName))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	JSON(w, http.StatusOK, entitlement)
}
