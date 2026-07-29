package composectl

import (
	"context"

	"github.com/flidai/leapview/internal/platform/buildinfo"
	"github.com/spf13/cobra"
)

func Command(ctx context.Context, controller *Controller) *cobra.Command {
	root := &cobra.Command{
		Use:           "leapviewctl",
		Short:         "Operate one Docker Compose LeapView instance",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	root.SetIn(controller.stdin)
	root.SetOut(controller.stdout)
	root.SetErr(controller.stderr)

	versionJSON := false
	version := &cobra.Command{
		Use:   "version",
		Short: "Report the leapviewctl build identity",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return buildinfo.Write(command.OutOrStdout(), "leapviewctl", buildinfo.Current(), versionJSON)
		},
	}
	version.Flags().BoolVar(&versionJSON, "json", false, "emit machine-readable JSON")

	initOptions := InitOptions{Environment: defaultEnvironment}
	initialize := &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration and one-time credentials",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return controller.Initialize(ctx, initOptions)
		},
	}
	initialize.Flags().StringVar(&initOptions.AdminEmail, "admin-email", "", "initial platform administrator email")
	initialize.Flags().StringVar(&initOptions.Domain, "domain", "", "canonical public application hostname")
	initialize.Flags().StringVar(&initOptions.Environment, "environment", defaultEnvironment, "instance environment")
	initialize.Flags().StringVar(&initOptions.Image, "image", "", "immutable LeapView image reference")
	initialize.Flags().BoolVar(&initOptions.NoHTTPS, "no-https", false, "use a trusted external HTTPS proxy instead of the Caddy overlay")

	start := &cobra.Command{
		Use:   "start",
		Short: "Start the instance and wait for health",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return controller.Start(ctx)
		},
	}
	status := &cobra.Command{
		Use:   "status",
		Short: "Show Compose and application health",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return controller.Status(ctx)
		},
	}
	logs := &cobra.Command{
		Use:                "logs [compose log arguments...]",
		Short:              "Show Compose service logs",
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return controller.Logs(ctx, args)
		},
	}
	firstLogin := &cobra.Command{
		Use:   "first-login",
		Short: "Print and delete the one-time credential file",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return controller.FirstLogin()
		},
	}
	backup := &cobra.Command{
		Use:   "backup [archive-name]",
		Short: "Create a validated full-instance backup",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return controller.Backup(ctx, name)
		},
	}
	restore := &cobra.Command{
		Use:   "restore <archive>",
		Short: "Restore a validated full-instance backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return controller.Restore(ctx, args[0])
		},
	}
	upgrade := &cobra.Command{
		Use:   "upgrade <image-digest>",
		Short: "Upgrade with paired image and state rollback",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return controller.Upgrade(ctx, args[0])
		},
	}
	rollbackConfirmed := false
	rollback := &cobra.Command{
		Use:   "rollback",
		Short: "Restore the previous paired image and state checkpoint",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return controller.Rollback(ctx, rollbackConfirmed)
		},
	}
	rollback.Flags().BoolVar(&rollbackConfirmed, "confirm", false, "confirm that post-upgrade state will be discarded")

	root.AddCommand(version, initialize, start, status, logs, firstLogin, backup, restore, upgrade, rollback)
	return root
}
