package types

import (
	"encoding/base64"
	"encoding/json"

	"github.com/pkg/errors"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	"github.com/replicatedhq/replicated-sdk/pkg/util"
)

type Distribution int64

const (
	UnknownDistribution Distribution = iota
	AKS
	DigitalOcean
	EKS
	GKE
	K0s
	K3s
	Kind
	Kurl
	MicroK8s
	Minikube
	OpenShift
	RKE2
	Tanzu
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
	K8sDistribution string                       `json:"k8s_distribution"`
}

func (d Distribution) String() string {
	switch d {
	case AKS:
		return "aks"
	case DigitalOcean:
		return "digital-ocean"
	case EKS:
		return "eks"
	case GKE:
		return "gke"
	case K0s:
		return "k0s"
	case K3s:
		return "k3s"
	case Kind:
		return "kind"
	case Kurl:
		return "kurl"
	case MicroK8s:
		return "microk8s"
	case Minikube:
		return "minikube"
	case OpenShift:
		return "openshift"
	case RKE2:
		return "rke2"
	case Tanzu:
		return "tanzu"
	}
	return "unknown"
}

type InstanceReport struct {
	Events []InstanceReportEvent `json:"events"`
}

type InstanceReportEvent struct {
	ReportedAt                int64  `json:"reported_at"`
	LicenseID                 string `json:"license_id"`
	InstanceID                string `json:"instance_id"`
	ClusterID                 string `json:"cluster_id"`
	AppStatus                 string `json:"app_status,omitempty"`
	ResourceStates            string `json:"resource_states,omitempty"`
	K8sVersion                string `json:"k8s_version"`
	K8sDistribution           string `json:"k8s_distribution,omitempty"`
	DownstreamChannelID       string `json:"downstream_channel_id,omitempty"`
	DownstreamChannelSequence int64  `json:"downstream_channel_sequence"`
	DownstreamChannelName     string `json:"downstream_channel_name,omitempty"`
}

func (r *InstanceReport) Encode() ([]byte, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal instance report")
	}
	compressedData, err := util.GzipData(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to gzip instance report")
	}
	encodedData := base64.StdEncoding.EncodeToString(compressedData)

	return []byte(encodedData), nil
}

func DecodeInstanceReport(encodedData []byte) (*InstanceReport, error) {
	decodedData, err := base64.StdEncoding.DecodeString(string(encodedData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode instance report")
	}
	decompressedData, err := util.GunzipData(decodedData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to gunzip instance report")
	}

	instanceReport := InstanceReport{}
	if err := json.Unmarshal(decompressedData, &instanceReport); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal instance report")
	}

	return &instanceReport, nil
}
