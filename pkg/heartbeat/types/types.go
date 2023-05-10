package types

import (
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
)

type HeartbeatInfo struct {
	InstanceID      string                       `json:"instance_id"`
	ClusterID       string                       `json:"cluster_id"`
	ChannelID       string                       `json:"channel_id"`
	ChannelName     string                       `json:"channel_name"`
	ChannelSequence int64                        `json:"channel_sequence"`
	ReleaseSequence int64                        `json:"release_sequence"`
	AppStatus       string                       `json:"app_status"`
	ResourceStates  appstatetypes.ResourceStates `json:"resource_states"`
	K8sVersion      string                       `json:"k8s_version"`
}
