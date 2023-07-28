package config

import (
	"github.com/pkg/errors"
	appstatetypes "github.com/replicatedhq/replicated-sdk/pkg/appstate/types"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"gopkg.in/yaml.v2"
)

type ReplicatedSDKConfig struct {
	License               string                               `yaml:"license"`
	LicenseFields         sdklicensetypes.LicenseFields        `yaml:"licenseFields"`
	AppName               string                               `yaml:"appName"`
	ChannelID             string                               `yaml:"channelID"`
	ChannelName           string                               `yaml:"channelName"`
	ChannelSequence       int64                                `yaml:"channelSequence"`
	ReleaseSequence       int64                                `yaml:"releaseSequence"`
	ReleaseCreatedAt      string                               `yaml:"releaseCreatedAt"`
	ReleaseNotes          string                               `yaml:"releaseNotes"`
	VersionLabel          string                               `yaml:"versionLabel"`
	ReplicatedAppEndpoint string                               `yaml:"replicatedAppEndpoint"`
	StatusInformers       []appstatetypes.StatusInformerString `yaml:"statusInformers"`
}

func ParseReplicatedSDKConfig(config []byte) (*ReplicatedSDKConfig, error) {
	var rc ReplicatedSDKConfig
	err := yaml.Unmarshal(config, &rc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config file")
	}
	return &rc, nil
}
