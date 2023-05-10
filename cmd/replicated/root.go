package main

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replicated",
		Short: "replicated is the software development kit for Replicated",
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
	cmd.AddCommand(VersionCmd())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}

func initConfig() {
	viper.SetEnvPrefix("REPLICATED")
	viper.AutomaticEnv()
}
