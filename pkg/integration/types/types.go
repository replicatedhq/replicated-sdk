package types

import (
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
)

type MockData interface {
	GetVersion() string
}

type MockDataVersion struct {
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

func (m *MockDataVersion) GetVersion() string {
	return m.Version
}

type MockDataV1 struct {
	MockDataVersion   `json:",inline" yaml:",inline"`
	AppStatus         appstatetypes.State `json:"appStatus,omitempty" yaml:"appStatus,omitempty"`
	HelmChartURL      string              `json:"helmChartURL,omitempty" yaml:"helmChartURL,omitempty"`
	CurrentRelease    *MockRelease        `json:"currentRelease,omitempty" yaml:"currentRelease,omitempty"`
	DeployedReleases  []MockRelease       `json:"deployedReleases,omitempty" yaml:"deployedReleases,omitempty"`
	AvailableReleases []MockRelease       `json:"availableReleases,omitempty" yaml:"availableReleases,omitempty"`
}

type MockDataV2 struct {
	MockDataVersion   `json:",inline" yaml:",inline"`
	AppStatus         appstatetypes.AppStatus `json:"appStatus,omitempty" yaml:"appStatus,omitempty"`
	HelmChartURL      string                  `json:"helmChartURL,omitempty" yaml:"helmChartURL,omitempty"`
	CurrentRelease    *MockRelease            `json:"currentRelease,omitempty" yaml:"currentRelease,omitempty"`
	DeployedReleases  []MockRelease           `json:"deployedReleases,omitempty" yaml:"deployedReleases,omitempty"`
	AvailableReleases []MockRelease           `json:"availableReleases,omitempty" yaml:"availableReleases,omitempty"`
}

type MockRelease struct {
	VersionLabel         string `json:"versionLabel" yaml:"versionLabel"`
	ReleaseNotes         string `json:"releaseNotes" yaml:"releaseNotes"`
	CreatedAt            string `json:"createdAt" yaml:"createdAt"`
	DeployedAt           string `json:"deployedAt" yaml:"deployedAt"`
	HelmReleaseName      string `json:"helmReleaseName" yaml:"helmReleaseName"`
	HelmReleaseRevision  int    `json:"helmReleaseRevision" yaml:"helmReleaseRevision"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace" yaml:"helmReleaseNamespace"`
	ChannelID            string `json:"channelID" yaml:"channelID"`
	ChannelName          string `json:"channelName" yaml:"channelName"`
	ChannelSequence      int64  `json:"channelSequence" yaml:"channelSequence"`
	ReleaseSequence      int64  `json:"releaseSequence" yaml:"releaseSequence"`
}
