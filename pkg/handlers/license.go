package handlers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/replicatedhq/replicated-sdk/pkg/store"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

type LicenseInfo struct {
	LicenseID                      string                                          `json:"licenseID"`
	AppSlug                        string                                          `json:"appSlug"`
	ChannelName                    string                                          `json:"channelName"`
	CustomerName                   string                                          `json:"customerName"`
	CustomerEmail                  string                                          `json:"customerEmail"`
	LicenseType                    string                                          `json:"licenseType"`
	ChannelID                      string                                          `json:"channelID"`
	LicenseSequence                int64                                           `json:"licenseSequence"`
	IsAirgapSupported              bool                                            `json:"isAirgapSupported"`
	IsGitOpsSupported              bool                                            `json:"isGitOpsSupported"`
	IsIdentityServiceSupported     bool                                            `json:"isIdentityServiceSupported"`
	IsGeoaxisSupported             bool                                            `json:"isGeoaxisSupported"`
	IsSnapshotSupported            bool                                            `json:"isSnapshotSupported"`
	IsSupportBundleUploadSupported bool                                            `json:"isSupportBundleUploadSupported"`
	IsSemverRequired               bool                                            `json:"isSemverRequired"`
	Endpoint                       string                                          `json:"endpoint"`
	Entitlements                   map[string]kotsv1beta1.EntitlementField `json:"entitlements,omitempty"`
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

func licenseInfoFromWrapper(wrapper licensewrapper.LicenseWrapper) LicenseInfo {
	// Convert EntitlementFieldWrapper map to EntitlementField map for JSON serialization
	var entitlements map[string]kotsv1beta1.EntitlementField
	wrappedEntitlements := wrapper.GetEntitlements()
	if wrappedEntitlements != nil {
		entitlements = make(map[string]kotsv1beta1.EntitlementField, len(wrappedEntitlements))
		for key, wrapped := range wrappedEntitlements {
			// Both v1beta1 and v1beta2 EntitlementField have identical structure
			// Use the unwrapped field directly (V1 or V2)
			if wrapped.V1 != nil {
				entitlements[key] = *wrapped.V1
			} else if wrapped.V2 != nil {
				// v1beta2 EntitlementField is structurally identical to v1beta1
				// Safe to convert for JSON serialization
				v2Field := *wrapped.V2
				entitlements[key] = kotsv1beta1.EntitlementField{
					Title:       v2Field.Title,
					Description: v2Field.Description,
					Value: kotsv1beta1.EntitlementValue{
						Type:    kotsv1beta1.Type(v2Field.Value.Type),
						IntVal:  v2Field.Value.IntVal,
						StrVal:  v2Field.Value.StrVal,
						BoolVal: v2Field.Value.BoolVal,
					},
					ValueType: v2Field.ValueType,
					IsHidden:  v2Field.IsHidden,
					Signature: kotsv1beta1.EntitlementFieldSignature{
						V1: v2Field.Signature.V2, // Note: using V2 signature in V1 field for consistency
					},
				}
			}
		}
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
		Entitlements:                   entitlements,
	}
}
