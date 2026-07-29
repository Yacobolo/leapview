// Package cli owns command-line adapters for offline Admin operations.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const (
	defaultAuditRetentionDays         = 365
	defaultQueryRetentionDays         = 90
	defaultArchivedAgentRetentionDays = 180
	defaultAuthStateRetentionDays     = 30
)

// Options are the values accepted by offline Admin operations.
type Options struct {
	Apply             bool
	AuditDays         int
	QueryDays         int
	ArchivedAgentDays int
	AuthStateDays     int
	BackupOut         string
	RestoreFrom       string
	RestoreBefore     string
	ConfirmRestore    bool
	DatabaseOnly      bool
}

// Operations are the offline administrative use cases exposed by the CLI.
// Application composition implements this contract because it owns process
// configuration and construction of cross-capability resources.
type Operations interface {
	Initialize(context.Context, string, io.Writer) error
	AcknowledgeInitialCredentials(context.Context) error
	StorageCleanup(context.Context, Options, io.Writer) error
	Maintenance(context.Context, Options, io.Writer) error
	Backup(context.Context, Options, io.Writer) error
	Restore(context.Context, Options, io.Reader, io.Writer) error
}

// Command constructs the offline Admin command tree.
func Command(ctx context.Context, operations Operations) *cobra.Command {
	values := Options{}
	parent := &cobra.Command{Use: "admin", Short: "Administrative utilities"}

	initializeFormat := "json"
	acknowledgeCredentials := false
	initialize := &cobra.Command{
		Use:   "initialize",
		Short: "Initialize one instance administrator and publisher credential",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if operations == nil {
				return fmt.Errorf("Admin CLI operations are required")
			}
			if acknowledgeCredentials {
				return operations.AcknowledgeInitialCredentials(ctx)
			}
			return operations.Initialize(ctx, initializeFormat, command.OutOrStdout())
		},
	}
	initialize.Flags().StringVar(&initializeFormat, "format", "json", "output format (json)")
	initialize.Flags().BoolVar(&acknowledgeCredentials, "acknowledge-credentials", false, "remove the recoverable initialization credential bundle after it has been stored safely")

	storage := &cobra.Command{Use: "storage", Short: "Maintain analytical storage"}
	cleanup := operationCommand(operations, "cleanup", "Reconcile serving-state snapshots and clean DuckLake storage", func(command *cobra.Command) error {
		return operations.StorageCleanup(ctx, values, command.OutOrStdout())
	})
	cleanup.Flags().BoolVar(&values.Apply, "apply", false, "perform destructive cleanup instead of dry-run")
	storage.AddCommand(cleanup)

	maintenance := operationCommand(operations, "maintenance", "Prune bounded operational history", func(command *cobra.Command) error {
		return operations.Maintenance(ctx, values, command.OutOrStdout())
	})
	maintenance.Flags().BoolVar(&values.Apply, "apply", false, "delete rows instead of dry-run")
	maintenance.Flags().IntVar(&values.AuditDays, "audit-days", defaultAuditRetentionDays, "audit event retention in days; 0 disables audit pruning")
	maintenance.Flags().IntVar(&values.QueryDays, "query-days", defaultQueryRetentionDays, "query event retention in days; 0 disables query pruning")
	maintenance.Flags().IntVar(&values.ArchivedAgentDays, "archived-agent-days", defaultArchivedAgentRetentionDays, "archived agent conversation retention in days; 0 disables archived conversation pruning")
	maintenance.Flags().IntVar(&values.AuthStateDays, "auth-state-days", defaultAuthStateRetentionDays, "expired or revoked auth state retention in days; 0 disables auth-state pruning")

	backup := operationCommand(operations, "backup", "Create a consistent LeapView instance backup", func(command *cobra.Command) error {
		return operations.Backup(ctx, values, command.OutOrStdout())
	})
	backup.Flags().StringVar(&values.BackupOut, "out", "", "backup archive output path")
	backup.Flags().BoolVar(&values.DatabaseOnly, "database-only", false, "backup only the platform SQLite database")

	restore := operationCommand(operations, "restore", "Restore LeapView from a validated instance backup", func(command *cobra.Command) error {
		return operations.Restore(ctx, values, command.InOrStdin(), command.OutOrStdout())
	})
	restore.Flags().StringVar(&values.RestoreFrom, "from", "", "backup archive path to restore")
	restore.Flags().StringVar(&values.RestoreBefore, "current-out", "", "path for a backup of the current instance before replacement; - creates and discards a validated temporary checkpoint")
	restore.Flags().BoolVar(&values.ConfirmRestore, "confirm", false, "confirm replacement of the configured LeapView instance")
	restore.Flags().BoolVar(&values.DatabaseOnly, "database-only", false, "restore only the platform SQLite database")

	parent.AddCommand(initialize, storage, maintenance, backup, restore)
	return parent
}

func operationCommand(operations Operations, use, short string, run func(*cobra.Command) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(command *cobra.Command, _ []string) error {
			if operations == nil {
				return fmt.Errorf("Admin CLI operations are required")
			}
			return run(command)
		},
	}
}
