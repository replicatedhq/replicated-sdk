package store

import (
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	upstreamtypes "github.com/replicatedhq/replicated-sdk/pkg/upstream/types"
)

type InMemoryStore struct {
	replicatedID          string
	appID                 string
	license               *kotsv1beta1.License
	licenseFields         sdklicensetypes.LicenseFields
	appSlug               string
	appName               string
	channelID             string
	channelName           string
	channelSequence       int64
	releaseSequence       int64
	releaseCreatedAt      string
	releaseNotes          string
	versionLabel          string
	replicatedAppEndpoint string
	namespace             string
	appStatus             appstatetypes.AppStatus
	updates               []upstreamtypes.ChannelRelease
}

type InitInMemoryStoreOptions struct {
	ReplicatedID          string
	AppID                 string
	License               *kotsv1beta1.License
	LicenseFields         sdklicensetypes.LicenseFields
	AppName               string
	ChannelID             string
	ChannelName           string
	ChannelSequence       int64
	ReleaseSequence       int64
	ReleaseCreatedAt      string
	ReleaseNotes          string
	VersionLabel          string
	ReplicatedAppEndpoint string
	Namespace             string
}

func InitInMemory(options InitInMemoryStoreOptions) {
	SetStore(&InMemoryStore{
		replicatedID:          options.ReplicatedID,
		appID:                 options.AppID,
		appSlug:               options.License.Spec.AppSlug,
		license:               options.License,
		licenseFields:         options.LicenseFields,
		appName:               options.AppName,
		channelID:             options.ChannelID,
		channelName:           options.ChannelName,
		channelSequence:       options.ChannelSequence,
		releaseSequence:       options.ReleaseSequence,
		releaseCreatedAt:      options.ReleaseCreatedAt,
		releaseNotes:          options.ReleaseNotes,
		versionLabel:          options.VersionLabel,
		replicatedAppEndpoint: options.ReplicatedAppEndpoint,
		namespace:             options.Namespace,
	})
}

func (s *InMemoryStore) GetReplicatedID() string {
	return s.replicatedID
}

func (s *InMemoryStore) GetAppID() string {
	return s.appID
}

func (s *InMemoryStore) GetLicense() *kotsv1beta1.License {
	return s.license
}

func (s *InMemoryStore) SetLicense(license *kotsv1beta1.License) {
	s.license = license.DeepCopy()
}

func (s *InMemoryStore) GetLicenseFields() sdklicensetypes.LicenseFields {
	return s.licenseFields
}

func (s *InMemoryStore) SetLicenseFields(licenseFields sdklicensetypes.LicenseFields) {
	// copy by value not reference
	if licenseFields == nil {
		s.licenseFields = nil
		return
	}
	if s.licenseFields == nil {
		s.licenseFields = sdklicensetypes.LicenseFields{}
	}
	for k, v := range licenseFields {
		s.licenseFields[k] = v
	}
}

func (s *InMemoryStore) IsDevLicense() bool {
	return s.license.Spec.LicenseType == "dev"
}

func (s *InMemoryStore) GetAppSlug() string {
	return s.appSlug
}

func (s *InMemoryStore) GetAppName() string {
	return s.appName
}

func (s *InMemoryStore) GetChannelID() string {
	return s.channelID
}

func (s *InMemoryStore) GetChannelName() string {
	return s.channelName
}

func (s *InMemoryStore) GetChannelSequence() int64 {
	return s.channelSequence
}

func (s *InMemoryStore) GetReleaseSequence() int64 {
	return s.releaseSequence
}

func (s *InMemoryStore) GetReleaseCreatedAt() string {
	return s.releaseCreatedAt
}

func (s *InMemoryStore) GetReleaseNotes() string {
	return s.releaseNotes
}

func (s *InMemoryStore) GetVersionLabel() string {
	return s.versionLabel
}

func (s *InMemoryStore) GetReplicatedAppEndpoint() string {
	return s.replicatedAppEndpoint
}

func (s *InMemoryStore) GetNamespace() string {
	return s.namespace
}

func (s *InMemoryStore) GetAppStatus() appstatetypes.AppStatus {
	return s.appStatus
}

func (s *InMemoryStore) SetAppStatus(status appstatetypes.AppStatus) {
	s.appStatus = status
}

func (s *InMemoryStore) GetUpdates() []upstreamtypes.ChannelRelease {
	return s.updates
}

func (s *InMemoryStore) SetUpdates(updates []upstreamtypes.ChannelRelease) {
	s.updates = updates
}
