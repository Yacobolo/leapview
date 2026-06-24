package cli

import (
	"context"
	"fmt"

	"github.com/Yacobolo/libredash/internal/access"
	accesssqlite "github.com/Yacobolo/libredash/internal/access/sqlite"
	"github.com/Yacobolo/libredash/internal/config"
	"github.com/Yacobolo/libredash/internal/platform"
	"github.com/Yacobolo/libredash/internal/workspace"
	workspacesqlite "github.com/Yacobolo/libredash/internal/workspace/sqlite"
	"github.com/spf13/cobra"
)

func adminCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	parent := &cobra.Command{Use: "admin", Short: "Administrative utilities"}
	bootstrap := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap an owner principal and API token",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.MustLoad()
			store, err := platform.Open(ctx, cfg.DBPath())
			if err != nil {
				return err
			}
			defer store.Close()
			email := cfg.BootstrapEmail
			if email == "" {
				email = "admin@localhost"
			}
			workspaceRepo := workspacesqlite.NewRepository(store.SQLDB())
			if err := workspaceRepo.Ensure(ctx, workspace.EnsureInput{ID: workspace.WorkspaceID(opts.workspaceID), Title: opts.workspaceID}); err != nil {
				return err
			}
			accessRepo := accesssqlite.NewRepository(store.SQLDB())
			if err := accessRepo.BootstrapAdmin(ctx, opts.workspaceID, email); err != nil {
				return err
			}
			principal, err := accessRepo.PrincipalByID(ctx, access.PrincipalIDForEmail(email))
			if err != nil {
				return err
			}
			token, err := accessRepo.CreateAPIToken(ctx, principal.ID, "bootstrap")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), token)
			return nil
		},
	}
	parent.AddCommand(bootstrap)
	return parent
}
