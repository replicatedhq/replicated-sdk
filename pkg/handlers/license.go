package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	sdklicense "github.com/replicatedhq/kots-sdk/pkg/license"
	sdklicensetypes "github.com/replicatedhq/kots-sdk/pkg/license/types"
	"github.com/replicatedhq/kots-sdk/pkg/logger"
	"github.com/replicatedhq/kots-sdk/pkg/store"
	"github.com/replicatedhq/kots-sdk/pkg/util"
)

type LicenseInfo struct {
	LicenseID           string `json:"licenseID"`
	ChannelID           string `json:"channelID"`
	ChannelName         string `json:"channelName"`
	CustomerName        string `json:"customerName"`
	CustomerEmail       string `json:"customerEmail"`
	LicenseType         string `json:"licenseType"`
	IsAirgapSupported   bool   `json:"isAirgapSupported"`
	IsGitOpsSupported   bool   `json:"isGitOpsSupported"`
	IsSnapshotSupported bool   `json:"isSnapshotSupported"`
}

func GetLicenseInfo(w http.ResponseWriter, r *http.Request) {
	license := store.GetStore().GetLicense()

	if !util.IsAirgap() {
		l, err := sdklicense.GetLatestLicense(license)
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license fields"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		license = l.License

		// update the store
		store.GetStore().SetLicense(license)
	}

	licenseInfo := LicenseInfo{
		LicenseID:           license.Spec.LicenseID,
		ChannelID:           license.Spec.ChannelID,
		ChannelName:         license.Spec.ChannelName,
		CustomerName:        license.Spec.CustomerName,
		CustomerEmail:       license.Spec.CustomerEmail,
		LicenseType:         license.Spec.LicenseType,
		IsAirgapSupported:   license.Spec.IsAirgapSupported,
		IsGitOpsSupported:   license.Spec.IsGitOpsSupported,
		IsSnapshotSupported: license.Spec.IsSnapshotSupported,
	}

	JSON(w, http.StatusOK, licenseInfo)
}

func GetLicenseFields(w http.ResponseWriter, r *http.Request) {
	licenseFields := store.GetStore().GetLicenseFields()

	if !util.IsAirgap() {
		fields, err := sdklicense.GetLatestLicenseFields(store.GetStore().GetLicense())
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license fields"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		licenseFields = fields

		// update the store
		store.GetStore().SetLicenseFields(licenseFields)
	}

	JSON(w, http.StatusOK, licenseFields)
}

func GetLicenseField(w http.ResponseWriter, r *http.Request) {
	fieldName := mux.Vars(r)["fieldName"]

	licenseFields := store.GetStore().GetLicenseFields()
	if licenseFields == nil {
		licenseFields = sdklicensetypes.LicenseFields{}
	}

	if !util.IsAirgap() {
		field, err := sdklicense.GetLatestLicenseField(store.GetStore().GetLicense(), fieldName)
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license field"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if field == nil {
			// field might not exist or has been removed
			delete(licenseFields, fieldName)
		} else {
			licenseFields[fieldName] = *field
		}

		// update the store
		store.GetStore().SetLicenseFields(licenseFields)
	}

	if _, ok := licenseFields[fieldName]; !ok {
		logger.Errorf("license field %q not found", fieldName)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	JSON(w, http.StatusOK, licenseFields[fieldName])
}
