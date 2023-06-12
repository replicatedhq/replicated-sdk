package main

import (
	"encoding/base64"
	"os"
	"strings"

	"github.com/pkg/errors"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/replicated-sdk/pkg/apiserver"
	sdklicense "github.com/replicatedhq/replicated-sdk/pkg/license"
	sdklicensetypes "github.com/replicatedhq/replicated-sdk/pkg/license/types"
	"github.com/replicatedhq/replicated-sdk/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
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

			licenseID := v.GetString("license-id")
			licenseFile := v.GetString("license-file")
			licenseBase64 := v.GetString("license-base64")

			if licenseID == "" && licenseFile == "" && licenseBase64 == "" {
				return errors.New("--license-file or --license-base64 or --license-id is required")
			}

			if (licenseFile != "" && (licenseBase64 != "" || licenseID != "")) ||
				(licenseBase64 != "" && (licenseID != "" || licenseFile != "")) ||
				(licenseID != "" && (licenseBase64 != "" || licenseFile != "")) {
				return errors.New("only one of --license-file, --license-base64 or --license-id can be specified")
			}

			var err error
			var license *kotsv1beta1.License
			switch {
			case licenseID != "":
				license, err = sdklicense.GetLicenseByID(licenseID, v.GetString("endpoint"))
				if err != nil {
					return errors.Wrap(err, "failed to get license by id")
				}
				if license.Spec.LicenseType != "dev" {
					return errors.New("--license-id must be a development license")
				}
			case licenseFile != "":
				license, err = sdklicense.LoadLicenseFromPath(licenseFile)
				if err != nil {
					return errors.Wrap(err, "failed to parse license from file")
				}
			case licenseBase64 != "":
				decoded, err := base64.StdEncoding.DecodeString(licenseBase64)
				if err != nil {
					return errors.Wrap(err, "failed to base64 decode license")
				}
				license, err = sdklicense.LoadLicenseFromBytes(decoded)
				if err != nil {
					return errors.Wrap(err, "failed to parse license from base64")
				}
			}

			licenseFieldsFile := v.GetString("license-fields-file")
			licenseFieldsBase64 := v.GetString("license-fields-base64")
			if licenseFieldsFile != "" && licenseFieldsBase64 != "" {
				return errors.New("only one of --license-fields-file or --license-fields-base64 can be specified")
			}

			var licenseFields sdklicensetypes.LicenseFields
			if licenseFieldsFile != "" {
				b, err := os.ReadFile(licenseFieldsFile)
				if err != nil {
					return errors.Wrap(err, "failed to read license file")
				}
				if err := yaml.Unmarshal(b, &licenseFields); err != nil {
					return errors.Wrap(err, "failed to unmarshal license fields from file")
				}
			} else if licenseFieldsBase64 != "" {
				decoded, err := base64.StdEncoding.DecodeString(licenseFieldsBase64)
				if err != nil {
					return errors.Wrap(err, "failed to base64 decode license fields")
				}
				if err := yaml.Unmarshal(decoded, &licenseFields); err != nil {
					return errors.Wrap(err, "failed to unmarshal decoded license fields")
				}
			}

			params := apiserver.APIServerParams{
				License:                license,
				LicenseFields:          licenseFields,
				AppName:                v.GetString("app-name"),
				ChannelID:              v.GetString("channel-id"),
				ChannelName:            v.GetString("channel-name"),
				ChannelSequence:        v.GetInt64("channel-sequence"),
				ReleaseSequence:        v.GetInt64("release-sequence"),
				ReleaseIsRequired:      v.GetBool("release-is-required"),
				ReleaseCreatedAt:       v.GetString("release-created-at"),
				ReleaseNotes:           v.GetString("release-notes"),
				VersionLabel:           v.GetString("version-label"),
				InformersLabelSelector: v.GetString("informers-label-selector"),
				Namespace:              v.GetString("namespace"),
			}
			apiserver.Start(params)

			return nil
		},
	}

	cmd.Flags().String("license-id", "", "the application development license id")
	cmd.Flags().String("license-file", "", "path to the application license file")
	cmd.Flags().String("license-base64", "", "base64 encoded application license")
	cmd.Flags().String("license-fields-file", "", "path to the application license fields file")
	cmd.Flags().String("license-fields-base64", "", "base64 encoded application license fields")
	cmd.Flags().String("app-name", "", "the application name")
	cmd.Flags().String("channel-id", "", "the application channel id")
	cmd.Flags().String("channel-name", "", "the application channel name")
	cmd.Flags().Int64("channel-sequence", -1, "the application upstream channel sequence")
	cmd.Flags().Int64("release-sequence", -1, "the application upstream release sequence")
	cmd.Flags().Bool("release-is-required", false, "if the application release is required")
	cmd.Flags().String("release-created-at", "", "when the application release was created")
	cmd.Flags().String("release-notes", "", "the application release notes")
	cmd.Flags().String("version-label", "", "the application version label")
	cmd.Flags().String("informers-label-selector", "", "the label selector to use for status informers to detect application resources")
	cmd.Flags().String("namespace", "", "the namespace where the sdk/application is installed")
	cmd.Flags().String("endpoint", "", "the replicated api endpoint")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}
