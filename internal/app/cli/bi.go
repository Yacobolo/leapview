package cli

import (
	"context"

	analyticscli "github.com/Yacobolo/leapview/internal/analytics/cli"
	dashboardcli "github.com/Yacobolo/leapview/internal/dashboard/cli"
	workspacecli "github.com/Yacobolo/leapview/internal/workspace/cli"
	"github.com/spf13/cobra"
)

func workspacesCommand(ctx context.Context, _ *rootOptions) *cobra.Command {
	return workspacecli.WorkspacesCommand(ctx, capabilityAPIClient{})
}

func dashboardsCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	return dashboardcli.Command(ctx, capabilityAPIClient{}, opts.workspaceID)
}

func semanticModelsCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	return analyticscli.Command(ctx, capabilityAPIClient{}, opts.workspaceID)
}

func addTargetTokenFlags(command *cobra.Command, opts *rootOptions) {
	command.Flags().StringVar(&opts.target, "target", "", "LeapView server URL")
	command.Flags().StringVar(&opts.token, "token", "", "API token")
}
