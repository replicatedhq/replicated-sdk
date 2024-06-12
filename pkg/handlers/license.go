package handlers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

type LicenseInfo struct {
	LicenseID                      string                                  `json:"licenseID"`
	AppSlug                        string                                  `json:"appSlug"`
	ChannelName                    string                                  `json:"channelName"`
	CustomerName                   string                                  `json:"customerName"`
	CustomerEmail                  string                                  `json:"customerEmail"`
	LicenseType                    string                                  `json:"licenseType"`
	ChannelID                      string                                  `json:"channelID"`
	LicenseSequence                int64                                   `json:"licenseSequence"`
	IsAirgapSupported              bool                                    `json:"isAirgapSupported"`
	IsGitOpsSupported              bool                                    `json:"isGitOpsSupported"`
	IsIdentityServiceSupported     bool                                    `json:"isIdentityServiceSupported"`
	IsGeoaxisSupported             bool                                    `json:"isGeoaxisSupported"`
	IsSnapshotSupported            bool                                    `json:"isSnapshotSupported"`
	IsSupportBundleUploadSupported bool                                    `json:"isSupportBundleUploadSupported"`
	IsSemverRequired               bool                                    `json:"isSemverRequired"`
	Endpoint                       string                                  `json:"endpoint"`
	Entitlements                   map[string]kotsv1beta1.EntitlementField `json:"entitlements"`
}

func GetLicenseInfo(w http.ResponseWriter, r *http.Request) {
	license := store.GetStore().GetLicense()

	if !util.IsAirgap() {
		l, err := sdklicense.GetLatestLicense(license, store.GetStore().GetReplicatedAppEndpoint())
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license"))
			JSONCached(w, http.StatusOK, licenseInfoFromLicense(license))
			return
		}

		license = l.License
		store.GetStore().SetLicense(license)
	}

	JSON(w, http.StatusOK, licenseInfoFromLicense(license))
}

func GetLicenseFields(w http.ResponseWriter, r *http.Request) {
	licenseFields := store.GetStore().GetLicenseFields()

	if !util.IsAirgap() {
		fields, err := sdklicense.GetLatestLicenseFields(store.GetStore().GetLicense(), store.GetStore().GetReplicatedAppEndpoint())
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license fields"))
			JSONCached(w, http.StatusOK, licenseFields)
			return
		}

		licenseFields = fields
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
		field, err := sdklicense.GetLatestLicenseField(store.GetStore().GetLicense(), store.GetStore().GetReplicatedAppEndpoint(), fieldName)
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license field"))
			if lf, ok := licenseFields[fieldName]; !ok {
				JSONCached(w, http.StatusNotFound, fmt.Sprintf("license field %q not found", fieldName))
			} else {
				JSONCached(w, http.StatusOK, lf)
			}
			return
		}

		if field == nil {
			// field might not exist or has been removed
			delete(licenseFields, fieldName)
		} else {
			licenseFields[fieldName] = *field
		}
		store.GetStore().SetLicenseFields(licenseFields)
	}

	if _, ok := licenseFields[fieldName]; !ok {
		JSON(w, http.StatusNotFound, fmt.Sprintf("license field %q not found", fieldName))
		return
	}

	JSON(w, http.StatusOK, licenseFields[fieldName])
}

func licenseInfoFromLicense(license *kotsv1beta1.License) LicenseInfo {
	return LicenseInfo{
		LicenseID:                      license.Spec.LicenseID,
		AppSlug:                        license.Spec.AppSlug,
		ChannelName:                    license.Spec.ChannelName,
		CustomerName:                   license.Spec.CustomerName,
		CustomerEmail:                  license.Spec.CustomerEmail,
		LicenseType:                    license.Spec.LicenseType,
		ChannelID:                      license.Spec.ChannelID,
		LicenseSequence:                license.Spec.LicenseSequence,
		IsAirgapSupported:              license.Spec.IsAirgapSupported,
		IsGitOpsSupported:              license.Spec.IsGitOpsSupported,
		IsIdentityServiceSupported:     license.Spec.IsIdentityServiceSupported,
		IsGeoaxisSupported:             license.Spec.IsGeoaxisSupported,
		IsSnapshotSupported:            license.Spec.IsSnapshotSupported,
		IsSupportBundleUploadSupported: license.Spec.IsSupportBundleUploadSupported,
		IsSemverRequired:               license.Spec.IsSemverRequired,
		Endpoint:                       license.Spec.Endpoint,
		Entitlements:                   license.Spec.Entitlements,
	}
}
