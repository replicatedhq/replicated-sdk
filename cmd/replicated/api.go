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

			configFilePath := v.GetString("config-file")
			integrationLicenseID := v.GetString("integration-license-id")

			configFile, err := os.ReadFile(configFilePath)
			if err != nil {
				return errors.Wrap(err, "failed to read config file")
			}

			replicatedConfig, err := config.ParseReplicatedConfig(configFile)
			if err != nil {
				return errors.Wrap(err, "failed to parse config file")
			}

			if replicatedConfig.License == "" && integrationLicenseID == "" {
				return errors.New("either a license or integrationLicenseID must be specified in the config file")
			}

			if replicatedConfig.License != "" && integrationLicenseID != "" {
				return errors.New("only one of license or integrationLicenseID should be specified in the config file")
			}

			var license *kotsv1beta1.License
			if replicatedConfig.License != "" {
				if license, err = sdklicense.LoadLicenseFromBytes([]byte(replicatedConfig.License)); err != nil {
					return errors.Wrap(err, "failed to parse license from base64")
				}
			} else if integrationLicenseID != "" {
				if license, err = sdklicense.GetLicenseByID(integrationLicenseID, v.GetString("endpoint")); err != nil {
					return errors.Wrap(err, "failed to get license by id")
				}
				if license.Spec.LicenseType != "dev" {
					return errors.New("--license-id must be a development license")
				}
			}

			params := apiserver.APIServerParams{
				License:                license,
				LicenseFields:          replicatedConfig.LicenseFields,
				AppName:                replicatedConfig.AppName,
				ChannelID:              replicatedConfig.ChannelID,
				ChannelName:            replicatedConfig.ChannelName,
				ChannelSequence:        replicatedConfig.ChannelSequence,
				ReleaseSequence:        replicatedConfig.ReleaseSequence,
				ReleaseCreatedAt:       replicatedConfig.ReleaseCreatedAt,
				ReleaseNotes:           replicatedConfig.ReleaseCreatedAt,
				VersionLabel:           replicatedConfig.VersionLabel,
				InformersLabelSelector: replicatedConfig.InformersLabelSelector,
				Namespace:              v.GetString("namespace"),
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
