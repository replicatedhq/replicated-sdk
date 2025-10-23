package store

import (
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	licensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
)

var (
	store Store

	_ Store = (*InMemoryStore)(nil)
)

type Store interface {
	GetReplicatedID() string
	GetAppID() string
	GetLicense() licensetypes.LicenseWrapper
	SetLicense(license licensetypes.LicenseWrapper)
	GetLicenseFields() licensetypes.LicenseFields
	SetLicenseFields(licenseFields licensetypes.LicenseFields)
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
	GetReleaseImages() []string
	GetNamespace() string
	GetAppStatus() appstatetypes.AppStatus
	SetAppStatus(status appstatetypes.AppStatus)
	// Pod image tracking
	SetPodImages(namespace string, podUID string, images []appstatetypes.ImageInfo)
	DeletePodImages(namespace string, podUID string)
	GetRunningImages() map[string][]string
	GetUpdates() []upstreamtypes.ChannelRelease
	SetUpdates(updates []upstreamtypes.ChannelRelease)
	GetReportAllImages() bool
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
