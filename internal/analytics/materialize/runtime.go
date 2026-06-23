package materialize

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	analyticsduckdb "github.com/Yacobolo/libredash/internal/analytics/duckdb"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	semanticquery "github.com/Yacobolo/libredash/internal/analytics/query"
)

type RuntimeConfig struct {
	ModelID string
	Model   *semanticmodel.Model
	DataDir string
	DBDir   string
}

type Runtime struct {
	model               *semanticmodel.Model
	dataDir             string
	db                  *analyticsduckdb.Database
	queries             *semanticquery.Service
	attachedConnections map[string]struct{}
	lastRefresh         time.Time
}

func OpenRuntime(ctx context.Context, config RuntimeConfig) (*Runtime, error) {
	if config.Model == nil {
		return nil, fmt.Errorf("semantic model is required")
	}
	if err := ValidateFiles(config.Model, config.DataDir); err != nil {
		return nil, err
	}
	dbPath := DatabasePath(config.DBDir, config.ModelID)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := analyticsduckdb.Open(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	runtime := &Runtime{
		model:               config.Model,
		dataDir:             config.DataDir,
		db:                  db,
		queries:             semanticquery.NewService(semanticquery.NewPlanner(config.Model), db),
		attachedConnections: map[string]struct{}{},
	}
	if err := runtime.Refresh(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return runtime, nil
}

func DatabasePath(dbDir, modelID string) string {
	if path := os.Getenv("LIBREDASH_DUCKDB_PATH"); path != "" {
		return path
	}
	return filepath.Join(dbDir, "libredash-"+modelID+".duckdb")
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Runtime) Refresh(ctx context.Context) error {
	lastRefresh, err := Refresh(ctx, r.db.SQLDB(), r.model, r.dataDir, r.attachedConnections)
	if err != nil {
		return err
	}
	r.lastRefresh = lastRefresh
	return nil
}

func (r *Runtime) Queries() *semanticquery.Service {
	if r == nil {
		return nil
	}
	return r.queries
}

func (r *Runtime) LastRefresh() time.Time {
	if r == nil {
		return time.Time{}
	}
	return r.lastRefresh
}

func (r *Runtime) DBPath() string {
	if r == nil {
		return ""
	}
	return r.db.Path()
}
