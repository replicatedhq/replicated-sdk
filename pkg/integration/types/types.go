package types

import (
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
)

type MockData struct {
	AppStatus         appstatetypes.State `json:"appStatus,omitempty" yaml:"appStatus,omitempty"`
	HelmChartURL      string              `json:"helmChartURL,omitempty" yaml:"helmChartURL,omitempty"`
	CurrentRelease    *MockRelease        `json:"currentRelease,omitempty" yaml:"currentRelease,omitempty"`
	DeployedReleases  []MockRelease       `json:"deployedReleases,omitempty" yaml:"deployedReleases,omitempty"`
	AvailableReleases []MockRelease       `json:"availableReleases,omitempty" yaml:"availableReleases,omitempty"`
}

type MockRelease struct {
	VersionLabel         string `json:"versionLabel" yaml:"versionLabel"`
	ReleaseNotes         string `json:"releaseNotes" yaml:"releaseNotes"`
	CreatedAt            string `json:"createdAt" yaml:"createdAt"`
	DeployedAt           string `json:"deployedAt" yaml:"deployedAt"`
	HelmReleaseName      string `json:"helmReleaseName" yaml:"helmReleaseName"`
	HelmReleaseRevision  int    `json:"helmReleaseRevision" yaml:"helmReleaseRevision"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace" yaml:"helmReleaseNamespace"`
}
