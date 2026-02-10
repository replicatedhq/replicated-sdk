package replicatedclient

import "time"

// State represents the application state.
type State string

const (
	StateReady       State = "ready"
	StateUpdating    State = "updating"
	StateDegraded    State = "degraded"
	StateUnavailable State = "unavailable"
	StateMissing     State = "missing"
)

// AppInfo is the response from GET /api/v1/app/info.
type AppInfo struct {
	InstanceID      string   `json:"instanceID"`
	AppSlug         string   `json:"appSlug"`
	AppName         string   `json:"appName"`
	AppStatus       State    `json:"appStatus"`
	HelmChartURL    string   `json:"helmChartURL,omitempty"`
	CurrentRelease  Release  `json:"currentRelease"`
	ChannelID       string   `json:"channelID"`
	ChannelName     string   `json:"channelName"`
	ChannelSequence int64    `json:"channelSequence"`
	ReleaseSequence int64    `json:"releaseSequence"`
}

// Release describes a deployed or available application release.
type Release struct {
	VersionLabel         string `json:"versionLabel"`
	ReleaseNotes         string `json:"releaseNotes"`
	CreatedAt            string `json:"createdAt"`
	DeployedAt           string `json:"deployedAt"`
	HelmReleaseName      string `json:"helmReleaseName,omitempty"`
	HelmReleaseRevision  int    `json:"helmReleaseRevision,omitempty"`
	HelmReleaseNamespace string `json:"helmReleaseNamespace,omitempty"`
}

// ResourceState describes the state of a single Kubernetes resource.
type ResourceState struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	State     State  `json:"state"`
}

// AppStatus contains the full application status with resource-level detail.
type AppStatus struct {
	AppSlug        string          `json:"appSlug"`
	ResourceStates []ResourceState `json:"resourceStates"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	State          State           `json:"state"`
	Sequence       int64           `json:"sequence"`
}

// AppStatusResponse is the response from GET /api/v1/app/status.
type AppStatusResponse struct {
	AppStatus AppStatus `json:"appStatus"`
}

// AppHistoryResponse is the response from GET /api/v1/app/history.
type AppHistoryResponse struct {
	Releases []Release `json:"releases"`
}

// ChannelRelease describes an available upstream release.
type ChannelRelease struct {
	VersionLabel string `json:"versionLabel"`
	CreatedAt    string `json:"createdAt"`
	ReleaseNotes string `json:"releaseNotes"`
}

// LicenseInfo is the response from GET /api/v1/license/info.
type LicenseInfo struct {
	LicenseID                      string      `json:"licenseID"`
	AppSlug                        string      `json:"appSlug"`
	ChannelName                    string      `json:"channelName"`
	CustomerID                     string      `json:"customerID"`
	CustomerName                   string      `json:"customerName"`
	CustomerEmail                  string      `json:"customerEmail"`
	LicenseType                    string      `json:"licenseType"`
	ChannelID                      string      `json:"channelID"`
	LicenseSequence                int64       `json:"licenseSequence"`
	IsAirgapSupported              bool        `json:"isAirgapSupported"`
	IsGitOpsSupported              bool        `json:"isGitOpsSupported"`
	IsIdentityServiceSupported     bool        `json:"isIdentityServiceSupported"`
	IsGeoaxisSupported             bool        `json:"isGeoaxisSupported"`
	IsSnapshotSupported            bool        `json:"isSnapshotSupported"`
	IsSupportBundleUploadSupported bool        `json:"isSupportBundleUploadSupported"`
	IsSemverRequired               bool        `json:"isSemverRequired"`
	Endpoint                       string      `json:"endpoint"`
	Entitlements                   interface{} `json:"entitlements,omitempty"`
}

// LicenseFieldSignature contains version-specific signatures for a license field.
type LicenseFieldSignature struct {
	V1 string `json:"v1,omitempty"`
	V2 string `json:"v2,omitempty"`
}

// LicenseField describes a single custom license field.
type LicenseField struct {
	Name        string                `json:"name,omitempty"`
	Title       string                `json:"title,omitempty"`
	Description string                `json:"description,omitempty"`
	Value       interface{}           `json:"value,omitempty"`
	ValueType   string                `json:"valueType,omitempty"`
	IsHidden    bool                  `json:"isHidden,omitempty"`
	Signature   LicenseFieldSignature `json:"signature,omitempty"`
}

// LicenseFields is a map of field name to LicenseField.
type LicenseFields map[string]LicenseField

// CustomAppMetricsData holds custom application metrics (scalar values only).
type CustomAppMetricsData map[string]interface{}

// SendCustomAppMetricsRequest is the request body for POST/PATCH /api/v1/app/custom-metrics.
type SendCustomAppMetricsRequest struct {
	Data CustomAppMetricsData `json:"data"`
}

// InstanceTagData holds instance tag information.
type InstanceTagData struct {
	Force bool              `json:"force"`
	Tags  map[string]string `json:"tags"`
}

// SendAppInstanceTagsRequest is the request body for POST /api/v1/app/instance-tags.
type SendAppInstanceTagsRequest struct {
	Data InstanceTagData `json:"data"`
}

// IntegrationStatusResponse is the response from GET /api/v1/integration/status.
type IntegrationStatusResponse struct {
	IsEnabled bool `json:"isEnabled"`
}

// HealthzResponse is the response from GET /healthz.
type HealthzResponse struct {
	Version string `json:"version"`
}

// ErrorResponse is the standard error response returned by the API.
type ErrorResponse struct {
	Error string `json:"error,omitempty"`
}
