package main

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/apiserver"
	"github.com/replicatedhq/replicated-sdk/pkg/config"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func APICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "api",
		Short:        "Starts the API server",
		Long:         ``,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			if v.GetString("log-level") == "debug" {
				logger.SetDebug()
			}

			namespace := v.GetString("namespace")
			configFilePath := v.GetString("config-file")
			integrationLicenseID := v.GetString("integration-license-id")

			if configFilePath == "" && integrationLicenseID == "" {
				return errors.New("either config file or integration license id must be specified")
			}

			var replicatedConfig = new(config.ReplicatedConfig)
			if configFilePath != "" {
				configFile, err := os.ReadFile(configFilePath)
				if err != nil {
					return errors.Wrap(err, "failed to read config file")
				}

				if replicatedConfig, err = config.ParseReplicatedConfig(configFile); err != nil {
					return errors.Wrap(err, "failed to parse config file")
				}
			}

			if replicatedConfig.License == "" && integrationLicenseID == "" {
				return errors.New("either license in the config file or integration license id must be specified")
			}

			if replicatedConfig.License != "" && integrationLicenseID != "" {
				return errors.New("only one of license in the config file or integration license id can be specified")
			}

			params := apiserver.APIServerParams{
				Context:               cmd.Context(),
				LicenseBytes:          []byte(replicatedConfig.License),
				IntegrationLicenseID:  integrationLicenseID,
				LicenseFields:         replicatedConfig.LicenseFields,
				AppName:               replicatedConfig.AppName,
				ChannelID:             replicatedConfig.ChannelID,
				ChannelName:           replicatedConfig.ChannelName,
				ChannelSequence:       replicatedConfig.ChannelSequence,
				ReleaseSequence:       replicatedConfig.ReleaseSequence,
				ReleaseCreatedAt:      replicatedConfig.ReleaseCreatedAt,
				ReleaseNotes:          replicatedConfig.ReleaseNotes,
				VersionLabel:          replicatedConfig.VersionLabel,
				ReplicatedAppEndpoint: replicatedConfig.ReplicatedAppEndpoint,
				StatusInformers:       replicatedConfig.StatusInformers,
				ReplicatedID:          replicatedConfig.ReplicatedID,
				AppID:                 replicatedConfig.AppID,
				Namespace:             namespace,
			}
			apiserver.Start(params)

			return nil
		},
	}

	cmd.Flags().String("config-file", "", "path to the replicated config file")
	cmd.Flags().String("namespace", "", "the namespace where replicated/application is installed")
	cmd.Flags().String("integration-license-id", "", "the id of the license to use")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}
