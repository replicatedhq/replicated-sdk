package store

import (
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/leader"
	licensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
	licensewrapper "github.com/replicatedhq/kotskinds/pkg/licensewrapper"
)

var (
	store Store

	_ Store = (*InMemoryStore)(nil)
)

type Store interface {
	GetReplicatedID() string
	GetAppID() string
	GetLicense() licensewrapper.LicenseWrapper
	SetLicense(license licensewrapper.LicenseWrapper)
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
	// Leader election
	GetLeaderElector() leader.LeaderElector
	SetLeaderElector(elector leader.LeaderElector)
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
