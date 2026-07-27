package module

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	analyticsducklake "github.com/Yacobolo/leapview/internal/analytics/ducklake"
	analyticsmaterialization "github.com/Yacobolo/leapview/internal/analytics/materialization"
	"github.com/Yacobolo/leapview/internal/analytics/queryaudit"
	queryaudithttp "github.com/Yacobolo/leapview/internal/analytics/queryaudit/http"
	queryauditsqlite "github.com/Yacobolo/leapview/internal/analytics/queryaudit/sqlite"
	"github.com/Yacobolo/leapview/internal/analytics/resource"
	"github.com/Yacobolo/leapview/internal/analytics/resultcache"
	storagemaintenance "github.com/Yacobolo/leapview/internal/servingstate/retention"
	"github.com/prometheus/client_golang/prometheus"
)

type Config struct {
	Database              *sql.DB
	RootDir               string
	CatalogPath           string
	DataPath              string
	MaxConnections        int
	MemoryMaxBytes        int64
	TempMaxBytes          int64
	MaxThreads            int
	TempDir               string
	RuntimeCacheEntries   int
	RuntimeCacheBytes     int64
	WorkspaceCacheEntries int
	WorkspaceCacheBytes   int64
	NodeCacheEntries      int
	NodeCacheBytes        int64
}

type Resources interface {
	resource.Provider
	resource.SessionProvider
}

func NewSurface(environment *analyticsducklake.Environment, cache *resultcache.Pool) *Module {
	return &Module{environment: environment, cache: cache}
}

// NewQueryAuditSurface constructs the analytics-owned control-plane adapter
// without opening the analytical runtime. It is useful to compose API-only
// surfaces and focused tests.
func NewQueryAuditSurface(database *sql.DB) *Module {
	if database == nil {
		return &Module{}
	}
	return &Module{queryAudit: queryauditsqlite.NewRepository(database)}
}

type QueryAuditSurface struct {
	repository queryaudit.Repository
}

func BuildQueryAuditSurface(database *sql.DB) *QueryAuditSurface {
	if database == nil {
		return &QueryAuditSurface{}
	}
	return &QueryAuditSurface{repository: queryauditsqlite.NewRepository(database)}
}

func (s *QueryAuditSurface) Provider() func() (queryaudit.Reader, error) {
	return func() (queryaudit.Reader, error) {
		if s == nil {
			return nil, nil
		}
		return s.repository, nil
	}
}

func (s *QueryAuditSurface) Recorder() queryaudit.Recorder {
	if s == nil {
		return nil
	}
	return s.repository
}

func (s *QueryAuditSurface) Events(workspaceID queryaudithttp.WorkspaceIDNormalizer) http.HandlerFunc {
	if s == nil {
		return NewQueryAuditEvents(nil, workspaceID)
	}
	return NewQueryAuditEvents(s.repository, workspaceID)
}

type Module struct {
	environment *analyticsducklake.Environment
	cache       *resultcache.Pool
	queryAudit  queryaudit.Repository
}

func Build(ctx context.Context, config Config) (*Module, error) {
	environment, err := analyticsducklake.Open(ctx, analyticsducklake.Config{
		RootDir: config.RootDir, CatalogPath: config.CatalogPath, DataPath: config.DataPath,
		MaxConnections: config.MaxConnections, MemoryMaxBytes: config.MemoryMaxBytes,
		TempMaxBytes: config.TempMaxBytes, MaxThreads: config.MaxThreads, TempDir: config.TempDir,
	})
	if err != nil {
		return nil, err
	}
	cache, err := resultcache.New(resultcache.Limits{
		RuntimeEntries: config.RuntimeCacheEntries, RuntimeBytes: config.RuntimeCacheBytes,
		WorkspaceEntries: config.WorkspaceCacheEntries, WorkspaceBytes: config.WorkspaceCacheBytes,
		NodeEntries: config.NodeCacheEntries, NodeBytes: config.NodeCacheBytes,
	})
	if err != nil {
		_ = environment.Close()
		return nil, err
	}
	var queryAudit queryaudit.Repository
	if config.Database != nil {
		queryAudit = queryauditsqlite.NewRepository(config.Database)
	}
	return &Module{environment: environment, cache: cache, queryAudit: queryAudit}, nil
}

func (m *Module) WorkspaceMaterializer() analyticsmaterialization.WorkspaceExecutor {
	if m == nil || m.environment == nil {
		return nil
	}
	return NewWorkspaceMaterializer(m.environment)
}

func (m *Module) RetentionSnapshots() storagemaintenance.SnapshotMaintenance {
	if m == nil {
		return nil
	}
	return m.environment
}

func (m *Module) AdminResources() Resources {
	if m == nil || m.environment == nil {
		return nil
	}
	return m.environment
}

func (m *Module) Collector() prometheus.Collector {
	if m == nil {
		return NewCollector(nil, nil)
	}
	return NewCollector(m.environment, m.cache)
}

func NewWorkspaceMaterializer(environment *analyticsducklake.Environment) analyticsmaterialization.WorkspaceExecutor {
	if environment == nil {
		return nil
	}
	return duckDBWorkspaceMaterializer{environment: environment}
}

func (m *Module) QueryAuditReader() queryaudit.Reader {
	if m == nil {
		return nil
	}
	return m.queryAudit
}

func (m *Module) QueryAuditProvider() func() (queryaudit.Reader, error) {
	return func() (queryaudit.Reader, error) {
		return m.QueryAuditReader(), nil
	}
}

func (m *Module) QueryAuditRecorder() queryaudit.Recorder {
	if m == nil {
		return nil
	}
	return m.queryAudit
}

func (m *Module) QueryAuditEvents(workspaceID queryaudithttp.WorkspaceIDNormalizer) http.HandlerFunc {
	return NewQueryAuditEvents(m.QueryAuditReader(), workspaceID)
}

func NewQueryAuditEvents(reader queryaudit.Reader, workspaceID queryaudithttp.WorkspaceIDNormalizer) http.HandlerFunc {
	handler := queryaudithttp.Handler{
		Reader: func() (queryaudit.Reader, error) {
			return reader, nil
		},
		WorkspaceID: workspaceID,
	}
	return handler.ListQueryEvents
}

func (m *Module) Healthy() error {
	if m == nil || m.environment == nil {
		return nil
	}
	return m.environment.Healthy()
}

func (m *Module) Fatal() <-chan struct{} {
	if m == nil || m.environment == nil {
		return nil
	}
	return m.environment.Fatal()
}

func (m *Module) Close() error {
	if m == nil {
		return nil
	}
	var errs []error
	if m.cache != nil {
		errs = append(errs, m.cache.Close())
	}
	if m.environment != nil {
		errs = append(errs, m.environment.Close())
	}
	return errors.Join(errs...)
}
