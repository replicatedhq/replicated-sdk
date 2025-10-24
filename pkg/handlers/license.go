package handlers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

type LicenseInfo struct {
	LicenseID                      string                                   `json:"licenseID"`
	AppSlug                        string                                   `json:"appSlug"`
	ChannelName                    string                                   `json:"channelName"`
	CustomerName                   string                                   `json:"customerName"`
	CustomerEmail                  string                                   `json:"customerEmail"`
	LicenseType                    string                                   `json:"licenseType"`
	ChannelID                      string                                   `json:"channelID"`
	LicenseSequence                int64                                    `json:"licenseSequence"`
	IsAirgapSupported              bool                                     `json:"isAirgapSupported"`
	IsGitOpsSupported              bool                                     `json:"isGitOpsSupported"`
	IsIdentityServiceSupported     bool                                     `json:"isIdentityServiceSupported"`
	IsGeoaxisSupported             bool                                     `json:"isGeoaxisSupported"`
	IsSnapshotSupported            bool                                     `json:"isSnapshotSupported"`
	IsSupportBundleUploadSupported bool                                     `json:"isSupportBundleUploadSupported"`
	IsSemverRequired               bool                                     `json:"isSemverRequired"`
	Endpoint                       string                                   `json:"endpoint"`
	V1Entitlements                 *map[string]kotsv1beta1.EntitlementField `json:"v1Entitlements,omitempty"`
	V2Entitlements                 *map[string]kotsv1beta2.EntitlementField `json:"v2Entitlements,omitempty"`
}

func GetLicenseInfo(w http.ResponseWriter, r *http.Request) {
	wrapper := store.GetStore().GetLicense()

	if !util.IsAirgap() {
		l, err := sdklicense.GetLatestLicense(wrapper, store.GetStore().GetReplicatedAppEndpoint())
		if err != nil {
			logger.Error(errors.Wrap(err, "failed to get latest license"))
			JSONCached(w, http.StatusOK, licenseInfoFromWrapper(wrapper))
			return
		}

		wrapper = l.License
		store.GetStore().SetLicense(wrapper)
	}

	JSON(w, http.StatusOK, licenseInfoFromWrapper(wrapper))
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

func licenseInfoFromWrapper(wrapper sdklicensetypes.LicenseWrapper) LicenseInfo {
	// Return native entitlements based on version - no conversion
	var v1Entitlements *map[string]kotsv1beta1.EntitlementField
	var v2Entitlements *map[string]kotsv1beta2.EntitlementField

	if wrapper.V1 != nil {
		v1Entitlements = &wrapper.V1.Spec.Entitlements
	} else if wrapper.V2 != nil {
		v2Entitlements = &wrapper.V2.Spec.Entitlements
	}

	return LicenseInfo{
		LicenseID:                      wrapper.GetLicenseID(),
		AppSlug:                        wrapper.GetAppSlug(),
		ChannelName:                    wrapper.GetChannelName(),
		CustomerName:                   wrapper.GetCustomerName(),
		CustomerEmail:                  wrapper.GetCustomerEmail(),
		LicenseType:                    wrapper.GetLicenseType(),
		ChannelID:                      wrapper.GetChannelID(),
		LicenseSequence:                wrapper.GetLicenseSequence(),
		IsAirgapSupported:              wrapper.IsAirgapSupported(),
		IsGitOpsSupported:              wrapper.IsGitOpsSupported(),
		IsIdentityServiceSupported:     wrapper.IsIdentityServiceSupported(),
		IsGeoaxisSupported:             wrapper.IsGeoaxisSupported(),
		IsSnapshotSupported:            wrapper.IsSnapshotSupported(),
		IsSupportBundleUploadSupported: wrapper.IsSupportBundleUploadSupported(),
		IsSemverRequired:               wrapper.IsSemverRequired(),
		Endpoint:                       wrapper.GetEndpoint(),
		V1Entitlements:                 v1Entitlements,
		V2Entitlements:                 v2Entitlements,
	}
}
