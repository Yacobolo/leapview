package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yacobolo/libredash/internal/access"
	accesssqlite "github.com/Yacobolo/libredash/internal/access/sqlite"
	"github.com/Yacobolo/libredash/internal/config"
	"github.com/Yacobolo/libredash/internal/instancelock"
	"github.com/Yacobolo/libredash/internal/platform"
	servingstate "github.com/Yacobolo/libredash/internal/servingstate"
	servingstatesqlite "github.com/Yacobolo/libredash/internal/servingstate/sqlite"
	storagemaintenance "github.com/Yacobolo/libredash/internal/storage/maintenance"
	"github.com/spf13/cobra"
)

func adminCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	parent := &cobra.Command{Use: "admin", Short: "Administrative utilities"}
	initializeFormat := "json"
	initialize := &cobra.Command{
		Use:   "initialize",
		Short: "Initialize one instance administrator and publisher credential",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAdminInitialize(ctx, initializeFormat, cmd.OutOrStdout())
		},
	}
	initialize.Flags().StringVar(&initializeFormat, "format", "json", "output format (json)")
	storage := &cobra.Command{Use: "storage", Short: "Maintain analytical storage"}
	cleanup := &cobra.Command{
		Use:   "cleanup",
		Short: "Reconcile serving-state snapshots and clean DuckLake storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdminStorageCleanup(ctx, opts, cmd.OutOrStdout())
		},
	}
	cleanup.Flags().BoolVar(&opts.apply, "apply", false, "perform destructive cleanup instead of dry-run")
	storage.AddCommand(cleanup)
	maintenance := &cobra.Command{
		Use:   "maintenance",
		Short: "Prune bounded operational history",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdminMaintenance(ctx, opts, cmd.OutOrStdout())
		},
	}
	maintenance.Flags().BoolVar(&opts.apply, "apply", false, "delete rows instead of dry-run")
	maintenance.Flags().IntVar(&opts.auditDays, "audit-days", defaultAuditRetentionDays, "audit event retention in days; 0 disables audit pruning")
	maintenance.Flags().IntVar(&opts.queryDays, "query-days", defaultQueryRetentionDays, "query event retention in days; 0 disables query pruning")
	maintenance.Flags().IntVar(&opts.archivedAgentDays, "archived-agent-days", defaultArchivedAgentRetentionDays, "archived agent conversation retention in days; 0 disables archived conversation pruning")
	maintenance.Flags().IntVar(&opts.authStateDays, "auth-state-days", defaultAuthStateRetentionDays, "expired or revoked auth state retention in days; 0 disables auth-state pruning")
	backup := &cobra.Command{
		Use:   "backup",
		Short: "Create a consistent LibreDash instance backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdminBackup(ctx, opts, cmd.OutOrStdout())
		},
	}
	backup.Flags().StringVar(&opts.backupOut, "out", "", "backup archive output path")
	backup.Flags().BoolVar(&opts.databaseOnly, "database-only", false, "backup only the platform SQLite database")
	restore := &cobra.Command{
		Use:   "restore",
		Short: "Restore LibreDash from a validated instance backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdminRestore(ctx, opts, cmd.OutOrStdout())
		},
	}
	restore.Flags().StringVar(&opts.restoreFrom, "from", "", "backup archive path to restore")
	restore.Flags().StringVar(&opts.restoreBefore, "current-out", "", "path for a backup of the current instance before replacement")
	restore.Flags().BoolVar(&opts.confirmRestore, "confirm", false, "confirm replacement of the configured LibreDash instance")
	restore.Flags().BoolVar(&opts.databaseOnly, "database-only", false, "restore only the platform SQLite database")
	parent.AddCommand(initialize, storage, maintenance, backup, restore)
	return parent
}

var errInstanceAlreadyInitialized = errors.New("LibreDash instance is already initialized")

type initialInstanceCredentials struct {
	Email                   string `json:"email"`
	TemporaryPassword       string `json:"temporaryPassword"`
	PublisherToken          string `json:"publisherToken"`
	PublisherTokenExpiresAt string `json:"publisherTokenExpiresAt"`
}

func runAdminInitialize(ctx context.Context, format string, out io.Writer) error {
	if format != "json" {
		return fmt.Errorf("admin initialize supports only --format json")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	email, err := initialAdminEmail(cfg)
	if err != nil {
		return err
	}
	lock, err := instancelock.Acquire(cfg.HomeDir)
	if err != nil {
		return err
	}
	defer lock.Release()
	store, err := platform.Open(ctx, cfg.DBPath())
	if err != nil {
		return err
	}
	defer store.Close()
	environment := serveEnvironment(cfg.Production, "", cfg.Environment)
	if err := store.BindInstanceEnvironment(ctx, string(environment)); err != nil {
		return err
	}
	repo := accesssqlite.NewRepository(store.SQLDB())
	var result initialInstanceCredentials
	err = repo.RunAuditedMutationBatch(ctx, func(txRepo access.Repository) ([]access.AuditEventInput, error) {
		sqliteRepo, ok := txRepo.(*accesssqlite.Repository)
		if !ok {
			return nil, fmt.Errorf("initialize access transaction is unavailable")
		}
		inserted, err := sqliteRepo.InsertPlatformSettingIfMissing(ctx, "instance.initialized", time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return nil, err
		}
		if !inserted {
			return nil, errInstanceAlreadyInitialized
		}
		created, err := txRepo.CreateLocalUser(ctx, access.LocalUserInput{Email: email, DisplayName: email, MustChange: true})
		if err != nil {
			return nil, err
		}
		principal, err := txRepo.SetPlatformRole(ctx, access.PlatformRoleInput{PrincipalID: created.Principal.ID, Email: email, DisplayName: email, Role: access.RolePlatformAdmin})
		if err != nil {
			return nil, err
		}
		expires := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
		token, _, err := txRepo.CreateAPITokenWithMetadata(ctx, access.APITokenInput{
			PrincipalID: principal.ID,
			Name:        "initial-publisher",
			Privileges: []access.Privilege{
				access.PrivilegeUseWorkspace, access.PrivilegeViewItem, access.PrivilegeQueryData,
				access.PrivilegeRefreshData, access.PrivilegeDeploy, access.PrivilegeActivateDeployment,
				access.PrivilegeViewData, access.PrivilegeIngestData,
			},
			ExpiresAt: expires,
		})
		if err != nil {
			return nil, err
		}
		result = initialInstanceCredentials{Email: email, TemporaryPassword: created.Password, PublisherToken: token, PublisherTokenExpiresAt: expires.Format(time.RFC3339)}
		return []access.AuditEventInput{{PrincipalID: principal.ID, Action: "instance.initialized", TargetType: "instance", TargetID: string(environment), Privilege: access.PrivilegeManagePlatform, Status: "success"}}, nil
	})
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}

const (
	defaultAuditRetentionDays         = 365
	defaultQueryRetentionDays         = 90
	defaultArchivedAgentRetentionDays = 180
	defaultAuthStateRetentionDays     = 30
)

func initialAdminEmail(cfg config.Config) (string, error) {
	email := strings.TrimSpace(cfg.BootstrapEmail)
	if email == "" {
		if cfg.Production {
			return "", fmt.Errorf("production instance initialization requires LIBREDASH_BOOTSTRAP_ADMIN_EMAIL")
		}
		email = "admin@localhost"
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address == "" {
		return "", fmt.Errorf("instance initialization requires a valid LIBREDASH_BOOTSTRAP_ADMIN_EMAIL")
	}
	return parsed.Address, nil
}

func runAdminBackup(ctx context.Context, opts *rootOptions, out io.Writer) error {
	if opts.backupOut == "" {
		return fmt.Errorf("admin backup requires --out")
	}
	cfg := config.MustLoad()
	lock, err := instancelock.Acquire(cfg.HomeDir)
	if err != nil {
		return err
	}
	defer lock.Release()
	if opts.databaseOnly {
		store, err := platform.Open(ctx, cfg.DBPath())
		if err != nil {
			return err
		}
		defer store.Close()
		if err := store.Backup(ctx, opts.backupOut); err != nil {
			return err
		}
		fmt.Fprintf(out, "database backup written: %s\n", opts.backupOut)
		return nil
	}
	if err := validateFullInstanceArchiveLayout(cfg); err != nil {
		return err
	}
	if err := platform.BackupInstance(ctx, platform.InstanceBackupOptions{
		HomeDir: cfg.HomeDir,
		DBPath:  cfg.DBPath(),
		OutPath: opts.backupOut,
	}); err != nil {
		return err
	}
	fmt.Fprintf(out, "instance backup written: %s\n", opts.backupOut)
	return nil
}

func runAdminRestore(ctx context.Context, opts *rootOptions, out io.Writer) error {
	if opts.restoreFrom == "" {
		return fmt.Errorf("admin restore requires --from")
	}
	if !opts.confirmRestore {
		return fmt.Errorf("admin restore requires --confirm")
	}
	cfg := config.MustLoad()
	lock, err := instancelock.Acquire(cfg.HomeDir)
	if err != nil {
		return err
	}
	defer lock.Release()
	expectedEnvironment, err := restoreTargetEnvironment(ctx, cfg)
	if err != nil {
		return err
	}
	if opts.databaseOnly {
		if err := platform.ValidateDatabaseInstanceEnvironment(ctx, opts.restoreFrom, string(expectedEnvironment)); err != nil {
			return err
		}
		if err := platform.Restore(ctx, cfg.DBPath(), opts.restoreFrom, opts.restoreBefore); err != nil {
			return err
		}
		fmt.Fprintf(out, "database restored from: %s\n", opts.restoreFrom)
		if opts.restoreBefore != "" {
			fmt.Fprintf(out, "previous database backup: %s\n", opts.restoreBefore)
		}
		return nil
	}
	if err := validateFullInstanceArchiveLayout(cfg); err != nil {
		return err
	}
	if err := platform.RestoreInstance(ctx, platform.InstanceRestoreOptions{
		TargetHomeDir:        cfg.HomeDir,
		BackupPath:           opts.restoreFrom,
		CurrentBackupOut:     opts.restoreBefore,
		ExpectedEnvironment:  string(expectedEnvironment),
		PreserveRelativeFile: instancelock.FileName,
	}); err != nil {
		return err
	}
	fmt.Fprintf(out, "instance restored from: %s\n", opts.restoreFrom)
	if opts.restoreBefore != "" {
		fmt.Fprintf(out, "previous instance backup: %s\n", opts.restoreBefore)
	}
	return nil
}

func restoreTargetEnvironment(ctx context.Context, cfg config.Config) (servingstate.Environment, error) {
	if _, err := os.Stat(cfg.DBPath()); err == nil {
		store, err := platform.Open(ctx, cfg.DBPath())
		if err != nil {
			return "", err
		}
		environment, environmentErr := offlineInstanceEnvironment(ctx, store, cfg)
		closeErr := store.Close()
		if environmentErr != nil {
			return "", environmentErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		return environment, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return serveEnvironment(cfg.Production, "", cfg.Environment), nil
}

func validateFullInstanceArchiveLayout(cfg config.Config) error {
	homeAbs, err := filepath.Abs(cfg.HomeDir)
	if err != nil {
		return err
	}
	paths := map[string]string{"DuckLake catalog": cfg.DuckLakeCatalogPath(), "DuckLake data": cfg.DuckLakeDataDir(), "artifact": cfg.ArtifactDir(), "runtime": cfg.RuntimeDir()}
	if cfg.ManagedDataBackend == "local" || cfg.ManagedDataBackend == "" {
		paths["managed-data"] = cfg.ManagedDataDir
	}
	for label, path := range paths {
		pathAbs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(homeAbs, pathAbs)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("full instance backup/restore requires %s path inside LIBREDASH_HOME; got %s outside %s", label, path, cfg.HomeDir)
		}
	}
	return nil
}

func runAdminStorageCleanup(ctx context.Context, opts *rootOptions, out io.Writer) error {
	cfg := config.MustLoad()
	lock, err := acquireDestructiveMaintenanceLock(cfg, opts.apply)
	if err != nil {
		return err
	}
	defer lock.Release()
	store, err := platform.Open(ctx, cfg.DBPath())
	if err != nil {
		return err
	}
	defer store.Close()
	repo := servingstatesqlite.NewRepository(store.SQLDB())
	environment, err := offlineInstanceEnvironment(ctx, store, cfg)
	if err != nil {
		return err
	}
	_, err = storagemaintenance.Run(ctx, repo, storagemaintenance.Options{
		Environment: environment,
		RootDir:     cfg.HomeDir,
		CatalogPath: cfg.DuckLakeCatalogPath(),
		DataPath:    cfg.DuckLakeDataDir(),
		DryRun:      !opts.apply,
		Out:         out,
	})
	if err != nil {
		return fmt.Errorf("storage cleanup: %w", err)
	}
	return nil
}

func offlineInstanceEnvironment(ctx context.Context, store *platform.Store, cfg config.Config) (servingstate.Environment, error) {
	bound, err := store.InstanceEnvironment(ctx)
	if err == nil {
		if requested := strings.TrimSpace(cfg.Environment); requested != "" && requested != bound {
			return "", fmt.Errorf("LibreDash instance is bound to environment %q, not %q", bound, requested)
		}
		return servingstate.Environment(bound), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read instance environment: %w", err)
	}
	environment := serveEnvironment(cfg.Production, "", cfg.Environment)
	if err := store.BindInstanceEnvironment(ctx, string(environment)); err != nil {
		return "", err
	}
	return environment, nil
}

func runAdminMaintenance(ctx context.Context, opts *rootOptions, out io.Writer) error {
	if opts.auditDays < 0 || opts.queryDays < 0 || opts.archivedAgentDays < 0 || opts.authStateDays < 0 {
		return fmt.Errorf("retention days must be zero or greater")
	}
	cfg := config.MustLoad()
	lock, err := acquireDestructiveMaintenanceLock(cfg, opts.apply)
	if err != nil {
		return err
	}
	defer lock.Release()
	store, err := platform.Open(ctx, cfg.DBPath())
	if err != nil {
		return err
	}
	defer store.Close()
	result, err := store.PruneOperationalHistory(ctx, platform.OperationalRetentionOptions{
		AuditEventsMaxAge:             days(opts.auditDays),
		QueryEventsMaxAge:             days(opts.queryDays),
		ArchivedAgentConversationsAge: days(opts.archivedAgentDays),
		AuthStateMaxAge:               days(opts.authStateDays),
		DryRun:                        !opts.apply,
	})
	if err != nil {
		return fmt.Errorf("operational maintenance: %w", err)
	}
	mode := "dry-run"
	if opts.apply {
		mode = "apply"
	}
	fmt.Fprintf(out, "mode: %s\n", mode)
	fmt.Fprintf(out, "audit events: %d\n", result.AuditEventsDeleted)
	fmt.Fprintf(out, "query events: %d\n", result.QueryEventsDeleted)
	fmt.Fprintf(out, "archived agent conversations: %d\n", result.ArchivedAgentConversationsDeleted)
	fmt.Fprintf(out, "expired oauth states: %d\n", result.ExpiredOAuthStatesDeleted)
	fmt.Fprintf(out, "stale sessions: %d\n", result.StaleSessionsDeleted)
	fmt.Fprintf(out, "stale api tokens: %d\n", result.StaleAPITokensDeleted)
	fmt.Fprintf(out, "stale service principal secrets: %d\n", result.StaleServicePrincipalSecretsDeleted)
	return nil
}

func acquireDestructiveMaintenanceLock(cfg config.Config, apply bool) (*instancelock.Lock, error) {
	if !apply {
		return nil, nil
	}
	return instancelock.Acquire(cfg.HomeDir)
}

func days(value int) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value) * 24 * time.Hour
}
