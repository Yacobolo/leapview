// Package adminoffline composes offline administrative operations from
// capability-owned adapters and application configuration.
package adminoffline

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

	"github.com/Yacobolo/leapview/internal/access"
	accesssqlite "github.com/Yacobolo/leapview/internal/access/sqlite"
	admincli "github.com/Yacobolo/leapview/internal/admin/cli"
	adminsqlite "github.com/Yacobolo/leapview/internal/admin/sqlite"
	analyticsducklake "github.com/Yacobolo/leapview/internal/analytics/ducklake"
	"github.com/Yacobolo/leapview/internal/app/config"
	"github.com/Yacobolo/leapview/internal/platform"
	"github.com/Yacobolo/leapview/internal/platform/filesystem"
	"github.com/Yacobolo/leapview/internal/platform/locking"
	servingstate "github.com/Yacobolo/leapview/internal/servingstate"
	storagemaintenance "github.com/Yacobolo/leapview/internal/servingstate/retention"
	servingstatesqlite "github.com/Yacobolo/leapview/internal/servingstate/sqlite"
)

// Operations implements the Admin capability's offline operation contract.
type Operations struct{}

func (Operations) Initialize(ctx context.Context, format string, out io.Writer) error {
	return runAdminInitialize(ctx, format, out)
}

func (Operations) AcknowledgeInitialCredentials(ctx context.Context) error {
	return acknowledgeInitialCredentials(ctx)
}

func (Operations) StorageCleanup(ctx context.Context, values admincli.Options, out io.Writer) error {
	return runAdminStorageCleanup(ctx, values, out)
}

func (Operations) Maintenance(ctx context.Context, values admincli.Options, out io.Writer) error {
	return runAdminMaintenance(ctx, values, out)
}

func (Operations) Backup(ctx context.Context, values admincli.Options, out io.Writer) error {
	return runAdminBackup(ctx, values, out)
}

func (Operations) Restore(ctx context.Context, values admincli.Options, in io.Reader, out io.Writer) error {
	return runAdminRestore(ctx, values, in, out)
}

var errInstanceAlreadyInitialized = errors.New("LeapView instance is already initialized")

const (
	instanceInitializedSetting        = "instance.initialized"
	initialCredentialRecoveryFileName = ".initial-credentials.json"
)

// InitialInstanceCredentials are the one-time credentials returned by
// initialization and evaluation bootstrap.
type InitialInstanceCredentials struct {
	Email                   string `json:"email"`
	TemporaryPassword       string `json:"temporaryPassword"`
	PublisherToken          string `json:"publisherToken"`
	PublisherTokenExpiresAt string `json:"publisherTokenExpiresAt"`
}

type initialInstanceCredentials = InitialInstanceCredentials

func runAdminInitialize(ctx context.Context, format string, out io.Writer) error {
	if format != "json" {
		return fmt.Errorf("admin initialize supports only --format json")
	}
	cfg, err := config.Load()
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
	environment := configuredEnvironment(cfg)
	if err := store.BindInstanceEnvironment(ctx, string(environment)); err != nil {
		return err
	}
	recoveryPath := initialCredentialRecoveryPath(cfg.HomeDir)
	if _, err := store.GetSetting(ctx, instanceInitializedSetting); err == nil {
		credentials, readErr := readInitialCredentialRecovery(recoveryPath)
		if readErr == nil {
			return writeAll(out, credentials)
		}
		if os.IsNotExist(readErr) {
			return errInstanceAlreadyInitialized
		}
		return readErr
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err := os.Remove(recoveryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale initialization credentials: %w", err)
	}
	email, err := initialAdminEmail(cfg)
	if err != nil {
		return err
	}
	repo := accesssqlite.NewRepository(store.SQLDB())
	var result initialInstanceCredentials
	var encodedResult []byte
	err = repo.RunAuditedMutationBatch(ctx, func(txRepo access.Repository) ([]access.AuditEventInput, error) {
		sqliteRepo, ok := txRepo.(*accesssqlite.Repository)
		if !ok {
			return nil, fmt.Errorf("initialize access transaction is unavailable")
		}
		inserted, err := sqliteRepo.InsertPlatformSettingIfMissing(ctx, instanceInitializedSetting, time.Now().UTC().Format(time.RFC3339))
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
			Name:        access.APITokenNameInitialPublisher,
			Privileges: []access.Privilege{
				access.PrivilegeUseWorkspace, access.PrivilegeViewItem, access.PrivilegeQueryData,
				access.PrivilegeRefreshData, access.PrivilegeDeploy, access.PrivilegeActivateDeployment,
				access.PrivilegeViewData, access.PrivilegeIngestData, access.PrivilegeViewAudit,
			},
			ExpiresAt: expires,
		})
		if err != nil {
			return nil, err
		}
		result = initialInstanceCredentials{Email: email, TemporaryPassword: created.Password, PublisherToken: token, PublisherTokenExpiresAt: expires.Format(time.RFC3339)}
		encodedResult, err = json.Marshal(result)
		if err != nil {
			return nil, err
		}
		encodedResult = append(encodedResult, '\n')
		if err := writeInitialCredentialRecovery(recoveryPath, encodedResult); err != nil {
			return nil, err
		}
		return []access.AuditEventInput{{PrincipalID: principal.ID, Action: "instance.initialized", TargetType: "instance", TargetID: string(environment), Privilege: access.PrivilegeManagePlatform, Status: "success"}}, nil
	})
	if err != nil {
		_ = os.Remove(recoveryPath)
		return err
	}
	return writeAll(out, encodedResult)
}

func acknowledgeInitialCredentials(ctx context.Context) error {
	cfg, err := config.Load()
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
	if _, err := offlineInstanceEnvironment(ctx, store, cfg); err != nil {
		return err
	}
	if _, err := store.GetSetting(ctx, instanceInitializedSetting); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("LeapView instance has not been initialized")
		}
		return err
	}
	if err := os.Remove(initialCredentialRecoveryPath(cfg.HomeDir)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("acknowledge initialization credentials: %w", err)
	}
	return nil
}

func initialCredentialRecoveryPath(homeDir string) string {
	return filepath.Join(homeDir, initialCredentialRecoveryFileName)
}

// InitialCredentialRecoveryPath returns the protected recovery bundle path.
func InitialCredentialRecoveryPath(homeDir string) string {
	return initialCredentialRecoveryPath(homeDir)
}

func readInitialCredentialRecovery(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("initialization credential recovery file %q must be a private regular file", path)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var credentials initialInstanceCredentials
	if err := json.Unmarshal(contents, &credentials); err != nil || credentials.Email == "" || credentials.TemporaryPassword == "" || credentials.PublisherToken == "" {
		return nil, fmt.Errorf("initialization credential recovery file %q is invalid", path)
	}
	return contents, nil
}

// ReadInitialCredentialRecovery reads and validates a protected recovery
// bundle.
func ReadInitialCredentialRecovery(path string) ([]byte, error) {
	return readInitialCredentialRecovery(path)
}

func writeInitialCredentialRecovery(path string, contents []byte) error {
	if err := securefs.EnsurePrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".initial-credentials-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(securefs.PrivateFileMode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(contents); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return err
	}
	return nil
}

// WriteInitialCredentialRecovery atomically writes a protected recovery
// bundle.
func WriteInitialCredentialRecovery(path string, contents []byte) error {
	return writeInitialCredentialRecovery(path, contents)
}

func writeAll(out io.Writer, contents []byte) error {
	written, err := out.Write(contents)
	if err == nil && written != len(contents) {
		return io.ErrShortWrite
	}
	return err
}

// WriteAll writes a complete credential response or returns an error.
func WriteAll(out io.Writer, contents []byte) error {
	return writeAll(out, contents)
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
			return "", fmt.Errorf("production instance initialization requires LEAPVIEW_BOOTSTRAP_ADMIN_EMAIL")
		}
		email = "admin@localhost"
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address == "" {
		return "", fmt.Errorf("instance initialization requires a valid LEAPVIEW_BOOTSTRAP_ADMIN_EMAIL")
	}
	return parsed.Address, nil
}

func runAdminBackup(ctx context.Context, opts admincli.Options, out io.Writer) error {
	if opts.BackupOut == "" {
		return fmt.Errorf("admin backup requires --out")
	}
	backupPath := opts.BackupOut
	stream := backupPath == "-"
	cfg := config.MustLoad()
	if stream && opts.DatabaseOnly {
		var err error
		backupPath, err = unusedTemporaryPathIn(filepath.Dir(cfg.HomeDir), "leapview-backup-*.db")
		if err != nil {
			return err
		}
		defer os.Remove(backupPath)
	}
	lock, err := instancelock.Acquire(cfg.HomeDir)
	if err != nil {
		return err
	}
	defer lock.Release()
	if opts.DatabaseOnly {
		store, err := platform.Open(ctx, cfg.DBPath())
		if err != nil {
			return err
		}
		defer store.Close()
		if err := store.Backup(ctx, backupPath); err != nil {
			return err
		}
		if stream {
			return copyFile(out, backupPath)
		}
		fmt.Fprintf(out, "database backup written: %s\n", backupPath)
		return nil
	}
	if err := validateFullInstanceArchiveLayout(cfg); err != nil {
		return err
	}
	derivedPaths, err := fullInstanceDerivedPaths(cfg)
	if err != nil {
		return err
	}
	backupOptions := platform.InstanceBackupOptions{
		HomeDir:              cfg.HomeDir,
		DBPath:               cfg.DBPath(),
		OutPath:              backupPath,
		ExcludeRelativePaths: derivedPaths,
	}
	if stream {
		return platform.BackupInstanceToWriter(ctx, backupOptions, out)
	}
	if err := platform.BackupInstance(ctx, backupOptions); err != nil {
		return err
	}
	fmt.Fprintf(out, "instance backup written: %s\n", backupPath)
	return nil
}

func runAdminRestore(ctx context.Context, opts admincli.Options, in io.Reader, out io.Writer) error {
	if opts.RestoreFrom == "" {
		return fmt.Errorf("admin restore requires --from")
	}
	if !opts.ConfirmRestore {
		return fmt.Errorf("admin restore requires --confirm")
	}
	cfg := config.MustLoad()
	restorePath := opts.RestoreFrom
	restoreLabel := restorePath
	stream := restorePath == "-"
	if stream {
		restoreLabel = "stdin"
	}
	if stream && opts.DatabaseOnly {
		if in == nil {
			return fmt.Errorf("admin restore --from - requires standard input")
		}
		var err error
		restorePath, err = copyReaderToTemporaryFile(in, filepath.Dir(cfg.HomeDir), "leapview-restore-*.db")
		if err != nil {
			return err
		}
		defer os.Remove(restorePath)
	}
	if stream && !opts.DatabaseOnly && in == nil {
		return fmt.Errorf("admin restore --from - requires standard input")
	}
	restoreBefore := opts.RestoreBefore
	if restoreBefore == "-" {
		var err error
		restoreBefore, err = unusedTemporaryPathIn(filepath.Dir(cfg.HomeDir), platform.InstanceRestoreCheckpointPattern)
		if err != nil {
			return err
		}
		defer os.Remove(restoreBefore)
	}
	lock, err := instancelock.Acquire(cfg.HomeDir)
	if err != nil {
		return err
	}
	defer lock.Release()
	expectedEnvironment, err := restoreTargetEnvironment(ctx, cfg)
	if err != nil {
		return err
	}
	if opts.DatabaseOnly {
		if err := platform.ValidateDatabaseInstanceEnvironment(ctx, restorePath, string(expectedEnvironment)); err != nil {
			return err
		}
		if err := platform.Restore(ctx, cfg.DBPath(), restorePath, restoreBefore); err != nil {
			return err
		}
		fmt.Fprintf(out, "database restored from: %s\n", restoreLabel)
		if restoreBefore != "" && opts.RestoreBefore != "-" {
			fmt.Fprintf(out, "previous database backup: %s\n", restoreBefore)
		}
		return nil
	}
	if err := validateFullInstanceArchiveLayout(cfg); err != nil {
		return err
	}
	derivedPaths, err := fullInstanceDerivedPaths(cfg)
	if err != nil {
		return err
	}
	restoreOptions := platform.InstanceRestoreOptions{
		TargetHomeDir:        cfg.HomeDir,
		BackupPath:           restorePath,
		CurrentBackupOut:     restoreBefore,
		DiscardCurrentBackup: opts.RestoreBefore == "-",
		ExpectedEnvironment:  string(expectedEnvironment),
		PreserveRelativeFile: instancelock.FileName,
		ResetRelativePaths:   derivedPaths,
	}
	if stream {
		restoreOptions.BackupPath = ""
		err = platform.RestoreInstanceFromReader(ctx, restoreOptions, in)
	} else {
		err = platform.RestoreInstance(ctx, restoreOptions)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "instance restored from: %s\n", restoreLabel)
	if restoreBefore != "" && opts.RestoreBefore != "-" {
		fmt.Fprintf(out, "previous instance backup: %s\n", restoreBefore)
	}
	return nil
}

func unusedTemporaryPathIn(directory, pattern string) (string, error) {
	if err := os.MkdirAll(directory, securefs.PrivateDirMode); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}

func copyFile(out io.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(out, file)
	return err
}

func copyReaderToTemporaryFile(in io.Reader, directory, pattern string) (string, error) {
	if err := os.MkdirAll(directory, securefs.PrivateDirMode); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", err
	}
	path := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(securefs.PrivateFileMode); err != nil {
		_ = file.Close()
		return "", err
	}
	if _, err := io.Copy(file, in); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	cleanup = false
	return path, nil
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
	return configuredEnvironment(cfg), nil
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
			return fmt.Errorf("full instance backup/restore requires %s path inside LEAPVIEW_HOME; got %s outside %s", label, path, cfg.HomeDir)
		}
	}
	return nil
}

func fullInstanceDerivedPaths(cfg config.Config) ([]string, error) {
	homeAbs, err := filepath.Abs(cfg.HomeDir)
	if err != nil {
		return nil, err
	}
	managedDataAbs, err := filepath.Abs(cfg.ManagedDataDir)
	if err != nil {
		return nil, err
	}
	backend := strings.TrimSpace(cfg.ManagedDataBackend)
	var derivedPath string
	switch backend {
	case "", "local":
		derivedPath = filepath.Join(managedDataAbs, "objects", "revisions")
	case "s3":
		derivedPath = filepath.Join(managedDataAbs, "runtime")
	default:
		return nil, fmt.Errorf("unsupported managed-data backend %q", backend)
	}
	relative, err := filepath.Rel(homeAbs, derivedPath)
	if err != nil {
		return nil, err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		if backend == "s3" {
			return nil, nil
		}
		return nil, fmt.Errorf("local managed-data derived path %s is outside %s", derivedPath, cfg.HomeDir)
	}
	return []string{filepath.ToSlash(relative)}, nil
}

// FullInstanceDerivedPaths returns regenerable instance paths excluded from
// backups.
func FullInstanceDerivedPaths(cfg config.Config) ([]string, error) {
	return fullInstanceDerivedPaths(cfg)
}

func runAdminStorageCleanup(ctx context.Context, opts admincli.Options, out io.Writer) error {
	cfg := config.MustLoad()
	lock, err := acquireDestructiveMaintenanceLock(cfg, opts.Apply)
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
	snapshots, err := analyticsducklake.Open(ctx, analyticsducklake.Config{
		RootDir: cfg.HomeDir, CatalogPath: cfg.DuckLakeCatalogPath(), DataPath: cfg.DuckLakeDataDir(),
	})
	if err != nil {
		return err
	}
	defer snapshots.Close()
	_, err = storagemaintenance.Run(ctx, repo, storagemaintenance.Options{
		Environment: string(environment),
		Snapshots:   snapshots,
		CatalogPath: cfg.DuckLakeCatalogPath(),
		DataPath:    cfg.DuckLakeDataDir(),
		DryRun:      !opts.Apply,
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
			return "", fmt.Errorf("LeapView instance is bound to environment %q, not %q", bound, requested)
		}
		return servingstate.Environment(bound), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read instance environment: %w", err)
	}
	environment := configuredEnvironment(cfg)
	if err := store.BindInstanceEnvironment(ctx, string(environment)); err != nil {
		return "", err
	}
	return environment, nil
}

func runAdminMaintenance(ctx context.Context, opts admincli.Options, out io.Writer) error {
	if opts.AuditDays < 0 || opts.QueryDays < 0 || opts.ArchivedAgentDays < 0 || opts.AuthStateDays < 0 {
		return fmt.Errorf("retention days must be zero or greater")
	}
	cfg := config.MustLoad()
	lock, err := acquireDestructiveMaintenanceLock(cfg, opts.Apply)
	if err != nil {
		return err
	}
	defer lock.Release()
	store, err := platform.Open(ctx, cfg.DBPath())
	if err != nil {
		return err
	}
	defer store.Close()
	result, err := adminsqlite.PruneOperationalHistory(ctx, store.SQLDB(), adminsqlite.RetentionOptions{
		AuditEventsMaxAge:             days(opts.AuditDays),
		QueryEventsMaxAge:             days(opts.QueryDays),
		ArchivedAgentConversationsAge: days(opts.ArchivedAgentDays),
		AuthStateMaxAge:               days(opts.AuthStateDays),
		DryRun:                        !opts.Apply,
	})
	if err != nil {
		return fmt.Errorf("operational maintenance: %w", err)
	}
	mode := "dry-run"
	if opts.Apply {
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

func configuredEnvironment(cfg config.Config) servingstate.Environment {
	if value := strings.TrimSpace(cfg.Environment); value != "" {
		return servingstate.NormalizeEnvironment(servingstate.Environment(value))
	}
	if cfg.Production {
		return servingstate.Environment("prod")
	}
	return servingstate.DefaultEnvironment
}
