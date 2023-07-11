package main

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/apiserver"
	"github.com/replicatedhq/replicated-sdk/pkg/config"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func APICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Starts the API server",
		Long:  ``,
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

			var err error
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

			var license *kotsv1beta1.License
			if replicatedConfig.License != "" {
				if license, err = sdklicense.LoadLicenseFromBytes([]byte(replicatedConfig.License)); err != nil {
					return errors.Wrap(err, "failed to parse license from base64")
				}
			} else if integrationLicenseID != "" {
				if license, err = sdklicense.GetLicenseByID(integrationLicenseID, replicatedConfig.ReplicatedAppEndpoint); err != nil {
					return errors.Wrap(err, "failed to get license by id for integration license id")
				}
				if license.Spec.LicenseType != "dev" {
					return errors.New("integration license must be a dev license")
				}
			}

			params := apiserver.APIServerParams{
				License:               license,
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
				Namespace:             namespace,
			}
			apiserver.Start(params)

			return nil
		},
	}

	cmd.Flags().String("config-file", "", "path to the replicated config file")
	cmd.Flags().String("namespace", "", "the namespace where the sdk/application is installed")
	cmd.Flags().String("integration-license-id", "", "the id of the license to use")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}
