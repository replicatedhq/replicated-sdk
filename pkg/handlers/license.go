package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

type LicenseInfo struct {
	LicenseID     string `json:"licenseID"`
	ChannelName   string `json:"channelName"`
	CustomerName  string `json:"customerName"`
	CustomerEmail string `json:"customerEmail"`
	LicenseType   string `json:"licenseType"`
}

func GetLicenseInfo(w http.ResponseWriter, r *http.Request) {
	license := store.GetStore().GetLicense()

	if !util.IsAirgap() {
		l, err := sdklicense.GetLatestLicense(license, store.GetStore().GetReplicatedAppEndpoint())
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license"))
		} else {
			license = l.License
			store.GetStore().SetLicense(license)
		}
	}

	licenseInfo := LicenseInfo{
		LicenseID:     license.Spec.LicenseID,
		ChannelName:   license.Spec.ChannelName,
		CustomerName:  license.Spec.CustomerName,
		CustomerEmail: license.Spec.CustomerEmail,
		LicenseType:   license.Spec.LicenseType,
	}

	JSON(w, http.StatusOK, licenseInfo)
}

func GetLicenseFields(w http.ResponseWriter, r *http.Request) {
	licenseFields := store.GetStore().GetLicenseFields()

	if !util.IsAirgap() {
		fields, err := sdklicense.GetLatestLicenseFields(store.GetStore().GetLicense(), store.GetStore().GetReplicatedAppEndpoint())
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license fields"))
		} else {
			licenseFields = fields
			store.GetStore().SetLicenseFields(licenseFields)
		}
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
		field, err := sdklicense.GetLatestLicenseField(store.GetStore().GetLicense(), store.GetStore().GetReplicatedAppEndpoint(), fieldName)
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license field"))
		} else {
			if field == nil {
				// field might not exist or has been removed
				delete(licenseFields, fieldName)
			} else {
				licenseFields[fieldName] = *field
			}
			store.GetStore().SetLicenseFields(licenseFields)
		}
	}

	if _, ok := licenseFields[fieldName]; !ok {
		logger.Errorf("license field %q not found", fieldName)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	JSON(w, http.StatusOK, licenseFields[fieldName])
}
