package cli

import (
	"context"

	workspacecli "github.com/flidai/leapview/internal/workspace/cli"
	"github.com/spf13/cobra"
)

func searchCommand(ctx context.Context, _ *rootOptions) *cobra.Command {
	return workspacecli.SearchCommand(ctx, capabilityAPIClient{})
}
