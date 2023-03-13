package main

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kots-sdk",
		Short: "kots-sdk is the software development kit for KOTS",
		Long:  ``,
		Args:  cobra.MinimumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.PersistentFlags().String("log-level", "info", "set the log level")

	cmd.AddCommand(APICmd())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}

func initConfig() {
	viper.SetEnvPrefix("KOTS_SDK")
	viper.AutomaticEnv()
}
