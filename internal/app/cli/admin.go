package cli

import (
	"context"
	"io"

	admincli "github.com/Yacobolo/leapview/internal/admin/cli"
	"github.com/Yacobolo/leapview/internal/app/adminoffline"
	"github.com/Yacobolo/leapview/internal/app/config"
	"github.com/spf13/cobra"
)

func adminCommand(ctx context.Context, _ *rootOptions) *cobra.Command {
	return admincli.Command(ctx, adminoffline.Operations{})
}

// The small delegations below preserve focused application-composition tests
// while all offline adapter construction and orchestration lives outside the
// CLI composition root.

type initialInstanceCredentials = adminoffline.InitialInstanceCredentials

func runAdminInitialize(ctx context.Context, format string, out io.Writer) error {
	return (adminoffline.Operations{}).Initialize(ctx, format, out)
}

func acknowledgeInitialCredentials(ctx context.Context) error {
	return (adminoffline.Operations{}).AcknowledgeInitialCredentials(ctx)
}

func runAdminStorageCleanup(ctx context.Context, opts *rootOptions, out io.Writer) error {
	return (adminoffline.Operations{}).StorageCleanup(ctx, adminOptions(opts), out)
}

func runAdminMaintenance(ctx context.Context, opts *rootOptions, out io.Writer) error {
	return (adminoffline.Operations{}).Maintenance(ctx, adminOptions(opts), out)
}

func runAdminBackup(ctx context.Context, opts *rootOptions, out io.Writer) error {
	return (adminoffline.Operations{}).Backup(ctx, adminOptions(opts), out)
}

func runAdminRestore(ctx context.Context, opts *rootOptions, in io.Reader, out io.Writer) error {
	return (adminoffline.Operations{}).Restore(ctx, adminOptions(opts), in, out)
}

func adminOptions(opts *rootOptions) admincli.Options {
	return admincli.Options{
		Apply:             opts.apply,
		AuditDays:         opts.auditDays,
		QueryDays:         opts.queryDays,
		ArchivedAgentDays: opts.archivedAgentDays,
		AuthStateDays:     opts.authStateDays,
		BackupOut:         opts.backupOut,
		RestoreFrom:       opts.restoreFrom,
		RestoreBefore:     opts.restoreBefore,
		ConfirmRestore:    opts.confirmRestore,
		DatabaseOnly:      opts.databaseOnly,
	}
}

func initialCredentialRecoveryPath(homeDir string) string {
	return adminoffline.InitialCredentialRecoveryPath(homeDir)
}

func writeInitialCredentialRecovery(path string, contents []byte) error {
	return adminoffline.WriteInitialCredentialRecovery(path, contents)
}

func readInitialCredentialRecovery(path string) ([]byte, error) {
	return adminoffline.ReadInitialCredentialRecovery(path)
}

func writeAll(out io.Writer, contents []byte) error {
	return adminoffline.WriteAll(out, contents)
}

func fullInstanceDerivedPaths(cfg config.Config) ([]string, error) {
	return adminoffline.FullInstanceDerivedPaths(cfg)
}
