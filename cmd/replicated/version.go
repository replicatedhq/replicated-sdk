package main

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/replicatedhq/replicated-sdk/pkg/buildversion"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type VersionOutput struct {
	Version string `json:"version"`
}

func VersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the current version and exit",
		Long:  `Print the current version and exit`,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			output := v.GetString("output")

			versionOutput := VersionOutput{
				Version: buildversion.Version(),
			}

			if output != "json" && output != "" {
				return errors.Errorf("output format %s not supported (allowed formats are: json)", output)
			}

			if output == "json" {
				// marshal JSON
				outputJSON, err := json.Marshal(versionOutput)
				if err != nil {
					return errors.Wrap(err, "failed to marshal version output")
				}
				fmt.Println(string(outputJSON))
				return nil
			}

			// print basic version info
			fmt.Printf("Replicated %s\n", buildversion.Version())

			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "", "output format (currently supported: json)")

	return cmd
}
