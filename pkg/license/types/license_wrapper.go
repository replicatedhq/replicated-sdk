package types

import (
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
)

// LicenseWrapper holds either a v1beta1 or v1beta2 license (never both).
// Exactly one field will be non-nil.
type LicenseWrapper struct {
	V1 *kotsv1beta1.License
	V2 *kotsv1beta2.License
}

// IsV1 returns true if this wrapper contains a v1beta1 license
func (w LicenseWrapper) IsV1() bool {
	return w.V1 != nil
}

// IsV2 returns true if this wrapper contains a v1beta2 license
func (w LicenseWrapper) IsV2() bool {
	return w.V2 != nil
}

// GetAppSlug returns the app slug from whichever version is present
func (w LicenseWrapper) GetAppSlug() string {
	if w.V1 != nil {
		return w.V1.Spec.AppSlug
	}
	if w.V2 != nil {
		return w.V2.Spec.AppSlug
	}
	return ""
}

// GetLicenseID returns the license ID from whichever version is present
func (w LicenseWrapper) GetLicenseID() string {
	if w.V1 != nil {
		return w.V1.Spec.LicenseID
	}
	if w.V2 != nil {
		return w.V2.Spec.LicenseID
	}
	return ""
}

// GetLicenseType returns the license type from whichever version is present
func (w LicenseWrapper) GetLicenseType() string {
	if w.V1 != nil {
		return w.V1.Spec.LicenseType
	}
	if w.V2 != nil {
		return w.V2.Spec.LicenseType
	}
	return ""
}

// GetEndpoint returns the endpoint from whichever version is present
func (w LicenseWrapper) GetEndpoint() string {
	if w.V1 != nil {
		return w.V1.Spec.Endpoint
	}
	if w.V2 != nil {
		return w.V2.Spec.Endpoint
	}
	return ""
}

// GetChannelID returns the channel ID from whichever version is present
func (w LicenseWrapper) GetChannelID() string {
	if w.V1 != nil {
		return w.V1.Spec.ChannelID
	}
	if w.V2 != nil {
		return w.V2.Spec.ChannelID
	}
	return ""
}

// GetChannelName returns the channel name from whichever version is present
func (w LicenseWrapper) GetChannelName() string {
	if w.V1 != nil {
		return w.V1.Spec.ChannelName
	}
	if w.V2 != nil {
		return w.V2.Spec.ChannelName
	}
	return ""
}

// GetCustomerName returns the customer name from whichever version is present
func (w LicenseWrapper) GetCustomerName() string {
	if w.V1 != nil {
		return w.V1.Spec.CustomerName
	}
	if w.V2 != nil {
		return w.V2.Spec.CustomerName
	}
	return ""
}

// GetCustomerEmail returns the customer email from whichever version is present
func (w LicenseWrapper) GetCustomerEmail() string {
	if w.V1 != nil {
		return w.V1.Spec.CustomerEmail
	}
	if w.V2 != nil {
		return w.V2.Spec.CustomerEmail
	}
	return ""
}

// GetLicenseSequence returns the license sequence from whichever version is present
func (w LicenseWrapper) GetLicenseSequence() int64 {
	if w.V1 != nil {
		return w.V1.Spec.LicenseSequence
	}
	if w.V2 != nil {
		return w.V2.Spec.LicenseSequence
	}
	return 0
}

// IsAirgapSupported returns whether airgap is supported from whichever version is present
func (w LicenseWrapper) IsAirgapSupported() bool {
	if w.V1 != nil {
		return w.V1.Spec.IsAirgapSupported
	}
	if w.V2 != nil {
		return w.V2.Spec.IsAirgapSupported
	}
	return false
}

// IsGitOpsSupported returns whether GitOps is supported from whichever version is present
func (w LicenseWrapper) IsGitOpsSupported() bool {
	if w.V1 != nil {
		return w.V1.Spec.IsGitOpsSupported
	}
	if w.V2 != nil {
		return w.V2.Spec.IsGitOpsSupported
	}
	return false
}

// IsIdentityServiceSupported returns whether identity service is supported from whichever version is present
func (w LicenseWrapper) IsIdentityServiceSupported() bool {
	if w.V1 != nil {
		return w.V1.Spec.IsIdentityServiceSupported
	}
	if w.V2 != nil {
		return w.V2.Spec.IsIdentityServiceSupported
	}
	return false
}

// IsGeoaxisSupported returns whether geoaxis is supported from whichever version is present
func (w LicenseWrapper) IsGeoaxisSupported() bool {
	if w.V1 != nil {
		return w.V1.Spec.IsGeoaxisSupported
	}
	if w.V2 != nil {
		return w.V2.Spec.IsGeoaxisSupported
	}
	return false
}

// IsSnapshotSupported returns whether snapshots are supported from whichever version is present
func (w LicenseWrapper) IsSnapshotSupported() bool {
	if w.V1 != nil {
		return w.V1.Spec.IsSnapshotSupported
	}
	if w.V2 != nil {
		return w.V2.Spec.IsSnapshotSupported
	}
	return false
}

// IsSupportBundleUploadSupported returns whether support bundle upload is supported from whichever version is present
func (w LicenseWrapper) IsSupportBundleUploadSupported() bool {
	if w.V1 != nil {
		return w.V1.Spec.IsSupportBundleUploadSupported
	}
	if w.V2 != nil {
		return w.V2.Spec.IsSupportBundleUploadSupported
	}
	return false
}

// IsSemverRequired returns whether semver is required from whichever version is present
func (w LicenseWrapper) IsSemverRequired() bool {
	if w.V1 != nil {
		return w.V1.Spec.IsSemverRequired
	}
	if w.V2 != nil {
		return w.V2.Spec.IsSemverRequired
	}
	return false
}
