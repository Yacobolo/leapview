package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	workspacecompiler "github.com/Yacobolo/libredash/internal/workspace/compiler"
)

type DataRuntimeConfig struct {
	ModelID string
	Model   *semanticmodel.Model
	DataDir string
	DBDir   string
}

type DataRuntimeFactory interface {
	OpenDashboardDataRuntime(ctx context.Context, config DataRuntimeConfig) (DataRuntime, error)
}

type DataRuntime interface {
	reportdef.DataService
	Refresh(ctx context.Context) error
	Close() error
	LastRefresh() time.Time
}

type setupRequiredError interface {
	SetupRequired() bool
}

type Service struct {
	mu               sync.RWMutex
	dataDir          string
	runtimes         map[string]*modelRuntime
	catalog          *CatalogService
	reports          *ReportService
	queries          *QueryService
	filters          *FilterService
	visuals          *VisualQueryService
	tables           *TableQueryService
	snapshots        *SnapshotService
	materializations *MaterializationService
}

type modelRuntime struct {
	model   *semanticmodel.Model
	data    DataRuntime
	ready   bool
	missing error
}

func New(dataDir string, factory DataRuntimeFactory) (*Service, error) {
	catalogPath := os.Getenv("LIBREDASH_CATALOG_PATH")
	if catalogPath == "" {
		var err error
		catalogPath, err = discoverCatalogPath()
		if err != nil {
			return nil, err
		}
	}
	duckDBDir := dataDir
	if path := os.Getenv("LIBREDASH_DUCKDB_DIR"); path != "" {
		duckDBDir = path
	}
	return NewFromCatalog(dataDir, catalogPath, duckDBDir, factory)
}

func NewFromCatalog(dataDir, catalogPath, duckDBDir string, factory DataRuntimeFactory) (*Service, error) {
	if factory == nil {
		return nil, fmt.Errorf("dashboard data runtime factory is required")
	}
	workspace, err := workspacecompiler.CompileDefinition(catalogPath)
	if err != nil {
		return nil, fmt.Errorf("loading workspace: %w", err)
	}

	service := &Service{
		dataDir:  dataDir,
		runtimes: map[string]*modelRuntime{},
	}
	service.catalog = NewCatalogService(&service.mu, workspace)
	service.reports = &ReportService{
		workspace: workspace,
		defaultID: workspace.Catalog.Dashboards[0].ID,
	}
	service.filters = &FilterService{}
	service.visuals = &VisualQueryService{filters: service.filters}
	service.tables = &TableQueryService{
		mu:       &service.mu,
		reports:  service.reports,
		runtimes: service.runtimes,
		filters:  service.filters,
	}
	service.snapshots = &SnapshotService{
		mu:       &service.mu,
		dataDir:  dataDir,
		reports:  service.reports,
		runtimes: service.runtimes,
		filters:  service.filters,
		visuals:  service.visuals,
	}
	service.queries = &QueryService{
		snapshots: service.snapshots,
		tables:    service.tables,
	}
	service.materializations = &MaterializationService{
		mu:       &service.mu,
		runtimes: service.runtimes,
	}

	for modelID, model := range workspace.Models {
		runtime := &modelRuntime{
			model: model,
		}
		service.runtimes[modelID] = runtime
		dataRuntime, err := factory.OpenDashboardDataRuntime(context.Background(), DataRuntimeConfig{
			ModelID: modelID,
			Model:   model,
			DataDir: dataDir,
			DBDir:   duckDBDir,
		})
		if err != nil {
			if setupRequired(err) {
				runtime.missing = err
				continue
			}
			return nil, err
		}
		runtime.data = dataRuntime
		runtime.ready = true
	}

	return service, nil
}

func (m *Service) Close() error {
	var closeErr error
	for _, runtime := range m.runtimes {
		if runtime.data == nil {
			continue
		}
		if err := runtime.data.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func (m *Service) DataDir() string {
	return m.dataDir
}

func discoverCatalogPath() (string, error) {
	candidates := []string{
		filepath.Join("dashboards", "catalog.yaml"),
		filepath.Join("..", "dashboards", "catalog.yaml"),
		filepath.Join("..", "..", "dashboards", "catalog.yaml"),
		filepath.Join("..", "..", "..", "dashboards", "catalog.yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find dashboards/catalog.yaml")
}

func setupRequired(err error) bool {
	var setup setupRequiredError
	return errors.As(err, &setup) && setup.SetupRequired()
}
