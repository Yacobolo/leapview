package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/dashboard"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/semantic"
	"github.com/Yacobolo/libredash/internal/workspace"
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
	mu         sync.RWMutex
	dataDir    string
	catalog    dashboard.Catalog
	workspace  *workspace.Definition
	runtimes   map[string]*modelRuntime
	defaultID  string
	defaultMID string
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
	workspace, err := semantic.LoadWorkspace(catalogPath)
	if err != nil {
		return nil, fmt.Errorf("loading workspace: %w", err)
	}

	metrics := &Service{
		dataDir:    dataDir,
		workspace:  workspace,
		runtimes:   map[string]*modelRuntime{},
		defaultID:  workspace.Catalog.Dashboards[0].ID,
		defaultMID: workspace.Catalog.SemanticModels[0].ID,
	}
	metrics.catalog = metrics.catalogView()

	for modelID, model := range workspace.Models {
		runtime := &modelRuntime{
			model: model,
		}
		metrics.runtimes[modelID] = runtime
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

	return metrics, nil
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

func (m *Service) Catalog() dashboard.Catalog {
	return m.catalog
}

func (m *Service) WorkspaceAssets(workspaceID, deploymentID string) ([]workspace.Asset, []workspace.AssetEdge, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.workspace == nil {
		return nil, nil, false
	}
	graph, err := workspacecompiler.ExtractLineage(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), m.workspace)
	if err != nil {
		return nil, nil, false
	}
	return graph.Assets, graph.Edges, true
}

func (m *Service) DefaultDashboardID() string {
	return m.defaultID
}

func (m *Service) ModelIDForDashboard(dashboardID string) string {
	report, ok := m.workspace.Dashboards[dashboardID]
	if !ok {
		return ""
	}
	if report.SemanticModel != "" {
		return report.SemanticModel
	}
	return ""
}

func (m *Service) Report(dashboardID string) (reportdef.Dashboard, *semanticmodel.Model, bool) {
	report, ok := m.workspace.Dashboards[dashboardID]
	if !ok {
		return reportdef.Dashboard{}, nil, false
	}
	if report.SemanticModel != "" {
		model, ok := m.workspace.Models[report.SemanticModel]
		if !ok {
			return reportdef.Dashboard{}, nil, false
		}
		return *report, model, true
	}
	return reportdef.Dashboard{}, nil, false
}

func (m *Service) NormalizeTableRequest(dashboardID string, request dashboard.TableRequest) dashboard.TableRequest {
	report, ok := m.workspace.Dashboards[dashboardID]
	if !ok {
		return request.WithDefaults()
	}
	defaults := dashboard.TableRequest{Block: "all", Start: 0, Count: dashboard.TableChunkSize}
	if table, ok := report.Tables["orders"]; ok && table.KindOrDefault() == "data_table" {
		defaults.Table = "orders"
		defaults.Sort = table.DefaultSort
	} else {
		for _, name := range sortedKeys(report.Tables) {
			table := report.Tables[name]
			if table.KindOrDefault() != "data_table" {
				continue
			}
			defaults.Table = name
			defaults.Sort = table.DefaultSort
			break
		}
	}
	if defaults.Table == "" {
		defaults = dashboard.DefaultTableRequest()
	}
	if request.Table == "" {
		request.Table = defaults.Table
	}
	if request.Block == "" {
		request.Block = defaults.Block
	}
	if request.Block != "all" && request.Block != "a" && request.Block != "b" && request.Block != "c" {
		request.Block = defaults.Block
	}
	if request.Count <= 0 {
		request.Count = defaults.Count
	}
	if request.Count > dashboard.TableMaxRequestCount {
		request.Count = dashboard.TableMaxRequestCount
	}
	if request.Start < 0 {
		request.Start = 0
	}
	if request.Sort.Key == "" {
		request.Sort = defaults.Sort
	}
	if request.Sort.Direction != "asc" && request.Sort.Direction != "desc" {
		if defaults.Sort.Direction != "" {
			request.Sort.Direction = defaults.Sort.Direction
		} else {
			request.Sort.Direction = "desc"
		}
	}
	return request
}

func (m *Service) DefaultFilters(dashboardID string) dashboard.Filters {
	report, ok := m.workspace.Dashboards[dashboardID]
	if !ok {
		return dashboard.Filters{}.WithDefaults()
	}
	return report.DefaultFilters()
}

func (m *Service) Pages(dashboardID string) []dashboard.Page {
	report, ok := m.workspace.Dashboards[dashboardID]
	if !ok {
		return nil
	}
	pages := make([]dashboard.Page, len(report.Pages))
	for i, page := range report.Pages {
		pages[i] = page.WithDefaults()
	}
	return pages
}

func (m *Service) catalogView() dashboard.Catalog {
	catalog := dashboard.Catalog{
		Workspace: dashboard.CatalogWorkspace{
			ID:          workspaceID(m.workspace.Catalog.Workspace),
			Title:       workspaceTitle(m.workspace.Catalog.Workspace),
			Description: m.workspace.Catalog.Workspace.Description,
		},
		Models:     make([]dashboard.CatalogModel, 0, len(m.workspace.Catalog.SemanticModels)),
		Dashboards: make([]dashboard.CatalogDashboard, 0, len(m.workspace.Catalog.Dashboards)),
	}
	for _, model := range m.workspace.Catalog.SemanticModels {
		catalog.Models = append(catalog.Models, dashboard.CatalogModel{
			ID:          model.ID,
			Title:       model.Title,
			Description: model.Description,
		})
	}
	for _, report := range m.workspace.Catalog.Dashboards {
		pageCount := 0
		semanticModel := ""
		if loaded, ok := m.workspace.Dashboards[report.ID]; ok {
			pageCount = len(loaded.Pages)
			semanticModel = loaded.SemanticModel
		}
		catalog.Dashboards = append(catalog.Dashboards, dashboard.CatalogDashboard{
			ID:            report.ID,
			Title:         report.Title,
			Description:   report.Description,
			SemanticModel: semanticModel,
			Tags:          append([]string{}, report.Tags...),
			PageCount:     pageCount,
		})
	}
	return catalog
}

func workspaceID(workspace workspace.CatalogWorkspace) string {
	if strings.TrimSpace(workspace.ID) != "" {
		return workspace.ID
	}
	return "libredash"
}

func workspaceTitle(workspace workspace.CatalogWorkspace) string {
	if strings.TrimSpace(workspace.Title) != "" {
		return workspace.Title
	}
	return "LibreDash Workspace"
}

func (m *Service) reportRuntime(dashboardID string) (*reportdef.Dashboard, *modelRuntime, error) {
	report, ok := m.workspace.Dashboards[dashboardID]
	if !ok {
		return nil, nil, fmt.Errorf("unknown dashboard %q", dashboardID)
	}
	if report.SemanticModel == "" {
		return nil, nil, fmt.Errorf("dashboard %q has no semantic model", dashboardID)
	}
	runtime, ok := m.runtimes[report.SemanticModel]
	if !ok {
		return nil, nil, fmt.Errorf("unknown semantic model %q", report.SemanticModel)
	}
	return report, runtime, nil
}

func (m *Service) QueryDashboard(ctx context.Context, dashboardID string, filters dashboard.Filters) (dashboard.Patch, error) {
	return m.QueryDashboardPage(ctx, dashboardID, "", filters)
}

func (m *Service) QueryDashboardPage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters) (dashboard.Patch, error) {
	report, runtime, err := m.reportRuntime(dashboardID)
	if report != nil {
		page := dashboardPage(report, pageID)
		filters = report.NormalizeFiltersForPage(page.ID, filters)
	} else {
		filters = filters.WithDefaults()
	}
	if err != nil {
		return dashboard.EmptyPatch(filters, m.dataDir, err), nil
	}
	if !runtime.ready {
		return dashboard.EmptyPatch(filters, m.dataDir, runtime.missing), nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	patch := dashboard.Patch{
		Filters: filters,
		Status: dashboard.Status{
			Loading:       false,
			LastUpdated:   refreshLabel(runtime),
			DataDirectory: m.dataDir,
		},
		Visuals: map[string]dashboard.Visual{},
	}

	page := dashboardPage(report, pageID)
	options, err := m.filterOptions(ctx, runtime, report, report.PageFilterIDs(page.ID))
	if err != nil {
		return dashboard.EmptyPatch(filters, m.dataDir, err), nil
	}
	patch.FilterOptions = options

	visuals, err := m.visuals(ctx, runtime, report, filters, pageVisualIDs(page))
	if err != nil {
		return dashboard.EmptyPatch(filters, m.dataDir, err), nil
	}
	patch.Visuals = visuals

	return patch, nil
}

func dashboardPage(report *reportdef.Dashboard, pageID string) dashboard.Page {
	if report == nil || len(report.Pages) == 0 {
		return dashboard.Page{}
	}
	if pageID != "" {
		for _, page := range report.Pages {
			if page.ID == pageID {
				return page.WithDefaults()
			}
		}
	}
	return report.Pages[0].WithDefaults()
}

func pageVisualIDs(page dashboard.Page) []string {
	seen := map[string]struct{}{}
	ids := []string{}
	for _, item := range page.Visuals {
		if item.Visual == "" {
			continue
		}
		if _, ok := seen[item.Visual]; ok {
			continue
		}
		seen[item.Visual] = struct{}{}
		ids = append(ids, item.Visual)
	}
	sort.Strings(ids)
	return ids
}

func (m *Service) QueryTable(ctx context.Context, dashboardID string, filters dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error) {
	return m.QueryTablePage(ctx, dashboardID, "", filters, request)
}

func (m *Service) QueryTablePage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error) {
	report, runtime, err := m.reportRuntime(dashboardID)
	if report != nil {
		page := dashboardPage(report, pageID)
		filters = report.NormalizeFiltersForPage(page.ID, filters)
	} else {
		filters = filters.WithDefaults()
	}
	request = m.NormalizeTableRequest(dashboardID, request)
	if err != nil {
		return dashboard.EmptyTable(request, err), nil
	}
	if !runtime.ready {
		return dashboard.EmptyTable(request, runtime.missing), nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	tableModel, ok := report.Tables[request.Table]
	if !ok {
		return dashboard.EmptyTable(request, fmt.Errorf("unknown table %q", request.Table)), nil
	}
	if tableModel.KindOrDefault() == "matrix_table" || tableModel.KindOrDefault() == "pivot_table" {
		return m.queryAggregateTable(ctx, runtime, report, request, tableModel, filters)
	}

	totalRows, err := m.countRows(ctx, runtime, report, tableModel.Query.Table, filters, "table", request.Table)
	if err != nil {
		return dashboard.EmptyTable(request, err), nil
	}
	availableRows := min(totalRows, dashboard.TableInteractiveRowCap)
	blocks, err := m.tableBlocks(ctx, runtime, report, tableModel, filters, request, availableRows)
	if err != nil {
		return dashboard.EmptyTable(request, err), nil
	}

	style := tableModel.Style.WithDefaults()
	return dashboard.Table{
		Version:       2,
		Kind:          tableModel.KindOrDefault(),
		Title:         tableModel.Title,
		Style:         style,
		Columns:       tableModel.Columns,
		TotalRows:     totalRows,
		AvailableRows: availableRows,
		IsCapped:      totalRows > availableRows,
		RowCap:        dashboard.TableInteractiveRowCap,
		ChunkSize:     dashboard.TableChunkSize,
		RowHeight:     style.RowHeight(),
		ResetVersion:  request.ResetVersion,
		Sort:          request.Sort,
		Blocks:        blocks,
		LoadingBlock:  "",
		Error:         "",
	}, nil
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

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func refreshLabel(runtime *modelRuntime) string {
	if runtime.data == nil || runtime.data.LastRefresh().IsZero() {
		return time.Now().Format("15:04:05")
	}
	return runtime.data.LastRefresh().Format("15:04:05")
}

func setupRequired(err error) bool {
	var setup setupRequiredError
	return errors.As(err, &setup) && setup.SetupRequired()
}
