package cli

import (
	"github.com/Yacobolo/leapview/internal/platform/buildinfo"
	"github.com/spf13/cobra"
)

func versionCommand() *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "version",
		Short: "Report the LeapView build identity",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return buildinfo.Write(command.OutOrStdout(), "leapview", buildinfo.Current(), jsonOutput)
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit machine-readable JSON")
	return command
}
