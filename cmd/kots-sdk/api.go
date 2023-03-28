package main

import (
	"encoding/base64"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kots-sdk/pkg/apiserver"
	"github.com/replicatedhq/kots-sdk/pkg/kotsutil"
	"github.com/replicatedhq/kots-sdk/pkg/logger"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
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

			if v.GetString("license-file") == "" && v.GetString("license-base64") == "" {
				return errors.New("--license-file or --license-base64 is required")
			}

			if v.GetString("license-file") != "" && v.GetString("license-base64") != "" {
				return errors.New("only one of --license-file and --license-base64 can be specified")
			}

			var license *kotsv1beta1.License
			if v.GetString("license-file") != "" {
				l, err := kotsutil.LoadLicenseFromPath(v.GetString("license-file"))
				if err != nil {
					return errors.Wrap(err, "failed to parse license from file")
				}
				license = l
			} else {
				decoded, err := base64.StdEncoding.DecodeString(v.GetString("license-base64"))
				if err != nil {
					return errors.Wrap(err, "failed to base64 decode license")
				}
				l, err := kotsutil.LoadLicenseFromBytes(decoded)
				if err != nil {
					return errors.Wrap(err, "failed to parse license from base64")
				}
				license = l
			}

			params := apiserver.APIServerParams{
				License:                license,
				ChannelID:              v.GetString("channel-id"),
				ChannelName:            v.GetString("channel-name"),
				ChannelSequence:        v.GetInt64("channel-sequence"),
				ReleaseSequence:        v.GetInt64("release-sequence"),
				VersionLabel:           v.GetString("version-label"),
				InformersLabelSelector: v.GetString("informers-label-selector"),
			}
			apiserver.Start(params)

			return nil
		},
	}

	cmd.Flags().String("license-file", "", "path to the application license file")
	cmd.Flags().String("license-base64", "", "base64 encoded application license")
	cmd.Flags().String("channel-id", "", "the application channel id")
	cmd.Flags().String("channel-name", "", "the application channel name")
	cmd.Flags().Int64("channel-sequence", -1, "the application upstream channel sequence")
	cmd.Flags().Int64("release-sequence", -1, "the application upstream release sequence")
	cmd.Flags().String("version-label", "", "the application version label")
	cmd.Flags().String("informers-label-selector", "", "the label selector to use for status informers to detect application resources")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}
