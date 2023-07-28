package main

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/apiserver"
	"github.com/replicatedhq/replicated-sdk/pkg/config"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
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

			var err error
			var replicatedSDKConfig = new(config.ReplicatedSDKConfig)
			if configFilePath != "" {
				configFile, err := os.ReadFile(configFilePath)
				if err != nil {
					return errors.Wrap(err, "failed to read config file")
				}

				if replicatedSDKConfig, err = config.ParseReplicatedSDKConfig(configFile); err != nil {
					return errors.Wrap(err, "failed to parse config file")
				}
			}

			if replicatedSDKConfig.License == "" && integrationLicenseID == "" {
				return errors.New("either license in the config file or integration license id must be specified")
			}

			if replicatedSDKConfig.License != "" && integrationLicenseID != "" {
				return errors.New("only one of license in the config file or integration license id can be specified")
			}

			var license *kotsv1beta1.License
			if replicatedSDKConfig.License != "" {
				if license, err = sdklicense.LoadLicenseFromBytes([]byte(replicatedSDKConfig.License)); err != nil {
					return errors.Wrap(err, "failed to parse license from base64")
				}
			} else if integrationLicenseID != "" {
				if license, err = sdklicense.GetLicenseByID(integrationLicenseID, replicatedSDKConfig.ReplicatedAppEndpoint); err != nil {
					return errors.Wrap(err, "failed to get license by id for integration license id")
				}
				if license.Spec.LicenseType != "dev" {
					return errors.New("integration license must be a dev license")
				}
			}

			params := apiserver.APIServerParams{
				Context:               cmd.Context(),
				License:               license,
				LicenseFields:         replicatedSDKConfig.LicenseFields,
				AppName:               replicatedSDKConfig.AppName,
				ChannelID:             replicatedSDKConfig.ChannelID,
				ChannelName:           replicatedSDKConfig.ChannelName,
				ChannelSequence:       replicatedSDKConfig.ChannelSequence,
				ReleaseSequence:       replicatedSDKConfig.ReleaseSequence,
				ReleaseCreatedAt:      replicatedSDKConfig.ReleaseCreatedAt,
				ReleaseNotes:          replicatedSDKConfig.ReleaseNotes,
				VersionLabel:          replicatedSDKConfig.VersionLabel,
				ReplicatedAppEndpoint: replicatedSDKConfig.ReplicatedAppEndpoint,
				StatusInformers:       replicatedSDKConfig.StatusInformers,
				Namespace:             namespace,
			}
			apiserver.Start(params)

			return nil
		},
	}

	cmd.Flags().String("config-file", "", "path to the replicated sdk config file")
	cmd.Flags().String("namespace", "", "the namespace where the sdk/application is installed")
	cmd.Flags().String("integration-license-id", "", "the id of the license to use")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}
