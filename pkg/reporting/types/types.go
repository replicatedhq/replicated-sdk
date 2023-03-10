package types

type ReportingInfo struct {
	InstanceID string         `json:"instance_id"`
	ClusterID  string         `json:"cluster_id"`
	Downstream DownstreamInfo `json:"downstream"`
	AppStatus  string         `json:"app_status"`
	K8sVersion string         `json:"k8s_version"`
}

type DownstreamInfo struct {
	ChannelID       string `json:"channel_id"`
	ChannelName     string `json:"channel_name"`
	ChannelSequence int64  `json:"channel_sequence"`
	ReleaseSequence int64  `json:"release_sequence"`
	Status          string `json:"status"`
}
