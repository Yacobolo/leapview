package duckdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	duckdbdriver "github.com/duckdb/duckdb-go/v2"
	"github.com/flidai/leapview/internal/analytics/connectionbinding"
	"github.com/flidai/leapview/internal/analytics/connectors"
	semanticmodel "github.com/flidai/leapview/internal/analytics/model"
)

type TargetRuntimeSession interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Close() error
}

type TargetRuntimeSessionOpener func(context.Context) (TargetRuntimeSession, error)

func NewIsolatedTargetRuntimeOpener() TargetRuntimeSessionOpener {
	return func(ctx context.Context) (TargetRuntimeSession, error) {
		connector, err := duckdbdriver.NewConnector(":memory:", func(driver.ExecerContext) error {
			return nil
		})
		if err != nil {
			return nil, err
		}
		database := sql.OpenDB(connector)
		database.SetMaxOpenConns(1)
		database.SetMaxIdleConns(1)
		connection, err := database.Conn(ctx)
		if err != nil {
			_ = database.Close()
			return nil, err
		}
		return &isolatedTargetRuntimeSession{connection: connection, database: database}, nil
	}
}

type TargetRuntimeLimits struct {
	MemoryMaxBytes int64
	TempMaxBytes   int64
	MaxThreads     int
}

type TargetRuntimePoolFactoryConfig struct {
	Open       TargetRuntimeSessionOpener
	Limits     TargetRuntimeLimits
	RequireTLS bool
}

type TargetRuntimePoolFactory struct {
	open       TargetRuntimeSessionOpener
	limits     TargetRuntimeLimits
	requireTLS bool
}

var _ connectionbinding.RuntimePoolFactory = (*TargetRuntimePoolFactory)(nil)

func NewTargetRuntimePoolFactory(config TargetRuntimePoolFactoryConfig) (*TargetRuntimePoolFactory, error) {
	if config.Open == nil || config.Limits.MemoryMaxBytes <= 0 ||
		config.Limits.TempMaxBytes <= 0 || config.Limits.MaxThreads <= 0 {
		return nil, fmt.Errorf(
			"%w: target runtime opener and positive resource limits are required",
			connectionbinding.ErrInvalidBinding,
		)
	}
	return &TargetRuntimePoolFactory{
		open: config.Open, limits: config.Limits, requireTLS: config.RequireTLS,
	}, nil
}

func (factory *TargetRuntimePoolFactory) Prepare(
	ctx context.Context,
	binding connectionbinding.TargetBinding,
	snapshot connectionbinding.CredentialSnapshot,
) (connectionbinding.RuntimePool, error) {
	if factory == nil || factory.open == nil {
		return nil, connectionbinding.ErrProviderUnavailable
	}
	if err := validateDatabaseProbeBinding(binding, factory.requireTLS); err != nil {
		return nil, err
	}
	connection, err := ApplyTargetBinding(
		semanticmodel.Connection{Kind: binding.ConnectorKind},
		binding,
		snapshot,
	)
	if err != nil {
		return nil, err
	}
	defer clear(connection.Auth)

	secret, ok, err := compileConnectionSecret(binding.LogicalConnectionID.String(), connection)
	if err != nil || !ok {
		return nil, connectionbinding.ErrInvalidCredentialBundle
	}
	attach, err := compileDatabaseAttach(binding.LogicalConnectionID.String(), connection)
	if err != nil {
		return nil, connectionbinding.ErrInvalidCredentialBundle
	}
	session, err := factory.open(ctx)
	if err != nil {
		return nil, err
	}
	closeOnFailure := true
	defer func() {
		if closeOnFailure {
			_ = session.Close()
		}
	}()
	spec, _ := connectors.LookupConnection(binding.ConnectorKind)
	statements := []string{
		"SET allow_persistent_secrets = false",
		"SET autoinstall_known_extensions = false",
		"SET autoload_known_extensions = false",
		"SET memory_limit = '" + strconv.FormatInt(factory.limits.MemoryMaxBytes, 10) + "B'",
		"SET max_temp_directory_size = '" + strconv.FormatInt(factory.limits.TempMaxBytes, 10) + "B'",
		"SET threads = " + strconv.Itoa(factory.limits.MaxThreads),
		"INSTALL " + spec.RequiredExtension + " FROM core",
		"LOAD " + spec.RequiredExtension,
		secret,
		attach,
		"SET lock_configuration = true",
	}
	for _, statement := range statements {
		if _, err := session.ExecContext(ctx, statement); err != nil {
			return nil, err
		}
	}
	closeOnFailure = false
	return &targetRuntimePool{session: session}, nil
}

func validateDatabaseProbeBinding(binding connectionbinding.TargetBinding, requireTLS bool) error {
	if err := binding.Validate(); err != nil {
		return err
	}
	spec, ok := connectors.LookupConnection(binding.ConnectorKind)
	if !ok || spec.AttachKind != connectors.AttachDatabase ||
		(binding.ConnectorKind != "postgres" && binding.ConnectorKind != "mysql") {
		return fmt.Errorf("%w: connector does not expose a bounded database probe", connectionbinding.ErrInvalidBinding)
	}
	if strings.TrimSpace(binding.Endpoint.Host) == "" || binding.Endpoint.Port <= 0 ||
		strings.TrimSpace(binding.Endpoint.Database) == "" ||
		strings.TrimSpace(binding.Endpoint.SourceIdentity) == "" {
		return fmt.Errorf("%w: database endpoint, port, database, and source identity are required", connectionbinding.ErrInvalidBinding)
	}
	if requireTLS && !secureDatabaseTLSMode(binding.ConnectorKind, binding.Endpoint.TLSMode) {
		return fmt.Errorf("%w: production database probes require verified transport", connectionbinding.ErrInvalidBinding)
	}
	return nil
}

func secureDatabaseTLSMode(kind, mode string) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch kind {
	case "postgres":
		return mode == "require" || mode == "verify-ca" || mode == "verify-full"
	case "mysql":
		return mode == "required" || mode == "verify_ca" || mode == "verify_identity"
	default:
		return false
	}
}

type targetRuntimePool struct {
	mu      sync.Mutex
	session TargetRuntimeSession
}

type isolatedTargetRuntimeSession struct {
	connection *sql.Conn
	database   *sql.DB
	once       sync.Once
	err        error
}

func (session *isolatedTargetRuntimeSession) ExecContext(
	ctx context.Context,
	statement string,
	args ...any,
) (sql.Result, error) {
	if session == nil || session.connection == nil {
		return nil, connectionbinding.ErrProviderUnavailable
	}
	return session.connection.ExecContext(ctx, statement, args...)
}

func (session *isolatedTargetRuntimeSession) Close() error {
	if session == nil {
		return nil
	}
	session.once.Do(func() {
		var errs []error
		if session.connection != nil {
			errs = append(errs, session.connection.Close())
			session.connection = nil
		}
		if session.database != nil {
			errs = append(errs, session.database.Close())
			session.database = nil
		}
		session.err = errors.Join(errs...)
	})
	return session.err
}

func (pool *targetRuntimePool) HealthCheck(ctx context.Context) error {
	if pool == nil {
		return connectionbinding.ErrProviderUnavailable
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.session == nil {
		return connectionbinding.ErrProviderUnavailable
	}
	_, err := pool.session.ExecContext(ctx, "SELECT 1")
	return err
}

func (pool *targetRuntimePool) Close() error {
	if pool == nil {
		return nil
	}
	pool.mu.Lock()
	session := pool.session
	pool.session = nil
	pool.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
}
