package store

import (
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
)

var (
	store Store

	_ Store = (*InMemoryStore)(nil)
)

type Store interface {
	GetReplicatedID() string
	GetAppID() string
	GetLicense() *kotsv1beta1.License
	SetLicense(license *kotsv1beta1.License)
	GetLicenseFields() sdklicensetypes.LicenseFields
	SetLicenseFields(licenseFields sdklicensetypes.LicenseFields)
	IsDevLicense() bool
	GetAppSlug() string
	GetAppName() string
	GetChannelID() string
	GetChannelName() string
	GetChannelSequence() int64
	GetReleaseSequence() int64
	GetReleaseCreatedAt() string
	GetReleaseNotes() string
	GetVersionLabel() string
	GetReplicatedAppEndpoint() string
	GetNamespace() string
	GetAppStatus() appstatetypes.AppStatus
	SetAppStatus(status appstatetypes.AppStatus)
	GetUpdates() []upstreamtypes.ChannelRelease
	SetUpdates(updates []upstreamtypes.ChannelRelease)
}

func SetStore(s Store) {
	store = s
}

func GetStore() Store {
	if store == nil {
		return &InMemoryStore{}
	}
	return store
}
