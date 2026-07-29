package cli

import (
	"context"

	admincli "github.com/Yacobolo/leapview/internal/admin/cli"
	"github.com/Yacobolo/leapview/internal/app/adminoffline"
	"github.com/spf13/cobra"
)

func adminCommand(ctx context.Context, _ *rootOptions) *cobra.Command {
	return admincli.Command(ctx, adminoffline.Operations{})
}
