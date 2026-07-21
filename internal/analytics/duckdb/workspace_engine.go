package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yacobolo/leapview/internal/analytics/connectors"
	analyticsducklake "github.com/Yacobolo/leapview/internal/analytics/ducklake"
	analyticsmaterialize "github.com/Yacobolo/leapview/internal/analytics/materialize"
	semanticmodel "github.com/Yacobolo/leapview/internal/analytics/model"
	semanticquery "github.com/Yacobolo/leapview/internal/analytics/query"
	"github.com/Yacobolo/leapview/internal/dataquery"
	"github.com/Yacobolo/leapview/internal/workload"
)

type workspaceEngine struct {
	db      *analyticsducklake.Environment
	sources *SourceRuntime
}

func (e *workspaceEngine) Close() error {
	if e == nil || e.db == nil {
		return nil
	}
	return e.db.Close()
}

func openWorkspaceEngine(ctx context.Context, config WorkspaceRuntimeConfig, budget EngineBudget) (Engine, error) {
	layout := analyticsducklake.NewLayout(config.DBDir)
	if config.CatalogPath != "" {
		layout.CatalogPath = config.CatalogPath
	}
	if config.DuckLakeDataPath != "" {
		layout.DataPath = config.DuckLakeDataPath
	}
	duckConfig := analyticsducklake.Config{RootDir: config.DBDir, CatalogPath: layout.CatalogPath, DataPath: layout.DataPath, SnapshotID: config.SnapshotID, MaxReaders: workspaceReaderCount(config.MaxReaders), MemoryBytes: budget.MemoryBytes, TempBytes: budget.TempBytes, Threads: budget.Threads, TempDir: budget.TempDir}
	var db *analyticsducklake.Environment
	var err error
	if config.SnapshotID > 0 {
		db, err = analyticsducklake.OpenSnapshot(ctx, duckConfig)
	} else {
		db, err = analyticsducklake.Open(ctx, duckConfig)
	}
	if err != nil {
		return nil, err
	}
	for _, model := range config.Models {
		if err := loadExtensions(ctx, db.SQLDB(), RequiredExtensions(model)); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	allowed, err := workspaceAllowedDirectories(config, layout, budget.TempDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	disableExternal := config.SnapshotID > 0 || !workspaceRequiresRemoteAccess(config.Models)
	if err := db.LockConfiguration(ctx, allowed, disableExternal); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &workspaceEngine{db: db, sources: NewSourceRuntime(db)}, nil
}

func workspaceRequiresRemoteAccess(models map[string]*semanticmodel.Model) bool {
	for _, model := range models {
		for _, connection := range model.Connections {
			if strings.TrimSpace(connection.Host) != "" || isRemoteLocation(connection.Path) || isRemoteLocation(connection.Scope) || isRemoteLocation(connection.Root) {
				return true
			}
		}
		for _, source := range model.Sources {
			if isRemoteLocation(source.Path) {
				return true
			}
		}
	}
	return false
}

func isRemoteLocation(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !connectors.IsLocalPath(value)
}

func workspaceAllowedDirectories(config WorkspaceRuntimeConfig, layout analyticsducklake.Layout, tempDir string) ([]string, error) {
	paths := []string{filepath.Dir(layout.CatalogPath), layout.DataPath, tempDir}
	if config.SnapshotID <= 0 {
		for _, model := range config.Models {
			for _, connection := range model.Connections {
				for _, candidate := range []string{connection.Root, connection.Scope} {
					if connectors.IsLocalPath(candidate) {
						paths = append(paths, candidate)
					}
				}
				if connectors.IsLocalPath(connection.Path) && strings.TrimSpace(connection.Path) != "" {
					paths = append(paths, filepath.Dir(connection.Path))
				}
				if dataPath, ok := connection.Options["data_path"]; ok {
					candidate := fmt.Sprint(dataPath)
					if connectors.IsLocalPath(candidate) {
						paths = append(paths, candidate)
					}
				}
			}
			for _, source := range model.Sources {
				if strings.TrimSpace(source.Path) == "" {
					continue
				}
				resolved, err := ResolveSourcePath(model, source)
				if err != nil {
					return nil, fmt.Errorf("derive source access policy: %w", err)
				}
				if connectors.IsLocalPath(resolved) {
					paths = append(paths, filepath.Dir(resolved))
				}
			}
		}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" || !connectors.IsLocalPath(path) {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve allowed DuckDB directory %q: %w", path, err)
		}
		absolute = filepath.Clean(absolute)
		if _, ok := seen[absolute]; ok {
			continue
		}
		seen[absolute] = struct{}{}
		result = append(result, absolute)
	}
	return result, nil
}

type lazyWorkspaceDatabase struct {
	handle  *Handle
	path    string
	readers int
}

func withWorkspaceEngine[T any](ctx context.Context, handle *Handle, fn func(context.Context, *workspaceEngine) (T, error)) (T, error) {
	var zero T
	var admission workload.Lease
	if handle.descriptor.RequireAdmission {
		if _, _, ok := workload.Current(ctx); !ok {
			admitter, found := workload.FromContext(ctx)
			if !found {
				return zero, ErrUnadmitted
			}
			var err error
			admission, err = admitter.Acquire(ctx, workload.Request{Class: workload.Interactive, WorkspaceID: handle.descriptor.WorkspaceID, Operation: "analytics.execute"})
			if err != nil {
				return zero, err
			}
			defer admission.Release()
			ctx = admission.Context()
		}
	}
	lease, err := handle.Acquire(ctx)
	if err != nil {
		return zero, err
	}
	defer lease.Release()
	engine, ok := lease.Engine().(*workspaceEngine)
	if !ok {
		return zero, fmt.Errorf("unexpected DuckDB workspace engine %T", lease.Engine())
	}
	value, err := fn(lease.Context(), engine)
	err = classifyResourceError(err)
	if err == nil {
		if rows, ok := any(value).(semanticquery.Rows); ok {
			var bytes int64
			for _, row := range rows {
				bytes += dataquery.EstimateRowBytes(row)
			}
			handle.pool.observeResult(len(rows), bytes)
		}
	}
	if reason, ok := ResourceExhaustedReasonOf(err); ok {
		handle.pool.recordExhaustion(string(reason))
	} else if reason, ok := dataquery.ResultLimitReasonOf(err); ok {
		handle.pool.recordExhaustion("result_" + string(reason))
	}
	return value, err
}

func (d lazyWorkspaceDatabase) Close() error         { return nil }
func (d lazyWorkspaceDatabase) Path() string         { return d.path }
func (d lazyWorkspaceDatabase) ReadConcurrency() int { return max(1, d.readers) }
func (d lazyWorkspaceDatabase) Exec(ctx context.Context, statement string) error {
	_, err := withWorkspaceEngine(ctx, d.handle, func(ctx context.Context, e *workspaceEngine) (struct{}, error) {
		return struct{}{}, e.db.Exec(ctx, statement)
	})
	return err
}
func (d lazyWorkspaceDatabase) Query(ctx context.Context, plan semanticquery.Plan) (semanticquery.Rows, error) {
	return withWorkspaceEngine(ctx, d.handle, func(ctx context.Context, e *workspaceEngine) (semanticquery.Rows, error) {
		return e.db.Query(ctx, plan)
	})
}
func (d lazyWorkspaceDatabase) Count(ctx context.Context, plan semanticquery.Plan) (int, error) {
	return withWorkspaceEngine(ctx, d.handle, func(ctx context.Context, e *workspaceEngine) (int, error) { return e.db.Count(ctx, plan) })
}
func (d lazyWorkspaceDatabase) FloatBounds(ctx context.Context, plan semanticquery.Plan, column string) (semanticquery.FloatBounds, error) {
	return withWorkspaceEngine(ctx, d.handle, func(ctx context.Context, e *workspaceEngine) (semanticquery.FloatBounds, error) {
		return e.db.FloatBounds(ctx, plan, column)
	})
}
func (d lazyWorkspaceDatabase) Histogram(ctx context.Context, plan semanticquery.Plan, spec semanticquery.HistogramSpec) ([]semanticquery.HistogramBin, error) {
	return withWorkspaceEngine(ctx, d.handle, func(ctx context.Context, e *workspaceEngine) ([]semanticquery.HistogramBin, error) {
		return e.db.Histogram(ctx, plan, spec)
	})
}
func (d lazyWorkspaceDatabase) Distribution(ctx context.Context, plan semanticquery.Plan, spec semanticquery.DistributionSpec) (semanticquery.Rows, error) {
	return withWorkspaceEngine(ctx, d.handle, func(ctx context.Context, e *workspaceEngine) (semanticquery.Rows, error) {
		return e.db.Distribution(ctx, plan, spec)
	})
}

type lazyWorkspaceSources struct{ handle *Handle }

func (s lazyWorkspaceSources) PrepareSourceRuntime(ctx context.Context, model *semanticmodel.Model) error {
	_, err := withWorkspaceEngine(ctx, s.handle, func(ctx context.Context, e *workspaceEngine) (struct{}, error) {
		return struct{}{}, e.sources.PrepareSourceRuntime(ctx, model)
	})
	return err
}
func (s lazyWorkspaceSources) PlanModelTable(ctx context.Context, model *semanticmodel.Model, name string, table semanticmodel.Table) (analyticsmaterialize.ModelTablePlan, error) {
	return withWorkspaceEngine(ctx, s.handle, func(ctx context.Context, e *workspaceEngine) (analyticsmaterialize.ModelTablePlan, error) {
		return e.sources.PlanModelTable(ctx, model, name, table)
	})
}
func (s lazyWorkspaceSources) SourceRelation(model *semanticmodel.Model, source semanticmodel.Source) (string, error) {
	return SourceRelation(model, source)
}
func (s lazyWorkspaceSources) ResolveSourcePath(model *semanticmodel.Model, source semanticmodel.Source) (string, error) {
	return ResolveSourcePath(model, source)
}

func refreshWorkspaceEngine(ctx context.Context, e *workspaceEngine, model *semanticmodel.Model, tableNames []string, servingStateID string, metadata map[string]string) (timeValue time.Time, snapshotID int64, err error) {
	if err = e.sources.PrepareSourceRuntime(ctx, model); err != nil {
		return time.Time{}, 0, err
	}
	snapshotID, err = e.db.Commit(ctx, servingStateID, metadata, func(tx *sql.Tx) error {
		executor := txExecutor{tx: tx}
		sources := txSourceRuntime{SourceRuntime: e.sources, tx: tx}
		if len(tableNames) > 0 {
			return analyticsmaterialize.ModelTablesNamed(ctx, executor, sources, model, tableNames)
		}
		return analyticsmaterialize.ModelTables(ctx, executor, sources, model)
	})
	if err != nil {
		return time.Time{}, 0, err
	}
	return time.Now(), snapshotID, nil
}
