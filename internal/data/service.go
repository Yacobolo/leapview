package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Yacobolo/libredash/internal/dashboard"
	semanticquery "github.com/Yacobolo/libredash/internal/query"
	"github.com/Yacobolo/libredash/internal/semantic"
	_ "github.com/marcboeker/go-duckdb/v2"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var aggregateWrapperPattern = regexp.MustCompile(`(?is)^\s*(?:AVG|SUM|MIN|MAX|MEDIAN)\s*\((.+)\)\s*$`)

type MissingDataError struct {
	DataDir string
	Missing []string
}

func (e *MissingDataError) Error() string {
	return fmt.Sprintf("local source files are missing in %s: %s. Run the workspace bootstrap script or set LIBREDASH_DATA_DIR.", e.DataDir, strings.Join(e.Missing, ", "))
}

type DuckDBMetrics struct {
	mu         sync.RWMutex
	dataDir    string
	catalog    dashboard.Catalog
	workspace  *semantic.Workspace
	runtimes   map[string]*modelRuntime
	defaultID  string
	defaultMID string
}

type modelRuntime struct {
	db                  *sql.DB
	dbPath              string
	model               *semantic.Model
	ready               bool
	missing             error
	lastRefresh         time.Time
	attachedConnections map[string]struct{}
}

func NewDuckDBMetrics(dataDir string) (*DuckDBMetrics, error) {
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
	return NewDuckDBMetricsFromCatalog(dataDir, catalogPath, duckDBDir)
}

func NewDuckDBMetricsFromCatalog(dataDir, catalogPath, duckDBDir string) (*DuckDBMetrics, error) {
	workspace, err := semantic.LoadWorkspace(catalogPath)
	if err != nil {
		return nil, fmt.Errorf("loading workspace: %w", err)
	}

	metrics := &DuckDBMetrics{
		dataDir:    dataDir,
		workspace:  workspace,
		runtimes:   map[string]*modelRuntime{},
		defaultID:  workspace.Catalog.Dashboards[0].ID,
		defaultMID: workspace.Catalog.SemanticModels[0].ID,
	}
	metrics.catalog = metrics.catalogView()

	for modelID, model := range workspace.Models {
		runtime := &modelRuntime{
			model:  model,
			dbPath: duckDBPath(duckDBDir, modelID),
		}
		metrics.runtimes[modelID] = runtime
		if err := metrics.validateFiles(runtime); err != nil {
			runtime.missing = err
			continue
		}
		if err := os.MkdirAll(filepath.Dir(runtime.dbPath), 0o755); err != nil {
			return nil, err
		}
		db, err := sql.Open("duckdb", runtime.dbPath)
		if err != nil {
			return nil, err
		}
		runtime.db = db
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, err
		}
		if err := metrics.RefreshCache(context.Background(), modelID); err != nil {
			db.Close()
			return nil, err
		}
		runtime.ready = true
	}

	return metrics, nil
}

func (m *DuckDBMetrics) Close() error {
	var closeErr error
	for _, runtime := range m.runtimes {
		if runtime.db == nil {
			continue
		}
		if err := runtime.db.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func (m *DuckDBMetrics) DataDir() string {
	return m.dataDir
}

func (m *DuckDBMetrics) Catalog() dashboard.Catalog {
	return m.catalog
}

func (m *DuckDBMetrics) MetricViews() []dashboard.MetricViewSummary {
	summaries := make([]dashboard.MetricViewSummary, 0, len(m.workspace.MetricViews))
	for _, id := range sortedKeys(m.workspace.MetricViews) {
		summary, ok := m.metricViewSummary(id)
		if ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries
}

func (m *DuckDBMetrics) MetricView(id string) (dashboard.MetricViewDetail, bool) {
	summary, ok := m.metricViewSummary(id)
	if !ok {
		return dashboard.MetricViewDetail{}, false
	}
	view := m.workspace.MetricViews[id]
	detail := dashboard.MetricViewDetail{MetricViewSummary: summary}

	for _, name := range sortedKeys(view.Dimensions) {
		dimension := view.Dimensions[name]
		detail.Dimensions = append(detail.Dimensions, dashboard.MetricViewDimension{
			Name:      name,
			Label:     dimension.Label,
			Expr:      dimension.Expr,
			Where:     dimension.Where,
			OrderExpr: dimension.OrderExpr,
		})
	}
	for _, name := range sortedKeys(view.Measures) {
		measure := view.Measures[name]
		detail.Measures = append(detail.Measures, dashboard.MetricViewMeasure{
			Name:        name,
			Label:       measure.Label,
			Description: measure.Description,
			Expression:  measure.Expression,
			Unit:        measure.Unit,
			Format:      measure.Format,
		})
	}
	for _, report := range m.dashboardsForMetricView(id) {
		detail.Dashboards = append(detail.Dashboards, dashboard.MetricViewDashboard{
			ID:          report.ID,
			Title:       report.Title,
			Description: report.Description,
			Tags:        append([]string{}, report.Tags...),
			PageCount:   dashboardPageCount(m.workspace.Dashboards[report.ID]),
		})
	}
	return detail, true
}

func (m *DuckDBMetrics) DefaultDashboardID() string {
	return m.defaultID
}

func (m *DuckDBMetrics) ModelIDForDashboard(dashboardID string) string {
	report, ok := m.workspace.Dashboards[dashboardID]
	if !ok {
		return ""
	}
	view, ok := m.firstMetricView(report)
	if !ok {
		return ""
	}
	return view.SemanticModel
}

func (m *DuckDBMetrics) Report(dashboardID string) (semantic.Dashboard, *semantic.Model, bool) {
	report, ok := m.workspace.Dashboards[dashboardID]
	if !ok {
		return semantic.Dashboard{}, nil, false
	}
	view, ok := m.firstMetricView(report)
	if !ok {
		return semantic.Dashboard{}, nil, false
	}
	model, ok := m.workspace.Models[view.SemanticModel]
	if !ok {
		return semantic.Dashboard{}, nil, false
	}
	return *report, model, true
}

func (m *DuckDBMetrics) NormalizeTableRequest(dashboardID string, request dashboard.TableRequest) dashboard.TableRequest {
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

func (m *DuckDBMetrics) DefaultFilters(dashboardID string) dashboard.Filters {
	report, ok := m.workspace.Dashboards[dashboardID]
	if !ok {
		return dashboard.Filters{}.WithDefaults()
	}
	return report.DefaultFilters()
}

func (m *DuckDBMetrics) Pages(dashboardID string) []dashboard.Page {
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

func (m *DuckDBMetrics) ModelGraph(modelID string) (dashboard.ModelGraph, bool) {
	model, ok := m.workspace.Models[modelID]
	if !ok {
		return dashboard.ModelGraph{}, false
	}
	return modelGraph(model, m.workspace.MetricViews), true
}

func (m *DuckDBMetrics) catalogView() dashboard.Catalog {
	catalog := dashboard.Catalog{
		Workspace: dashboard.CatalogWorkspace{
			ID:          workspaceID(m.workspace.Catalog.Workspace),
			Title:       workspaceTitle(m.workspace.Catalog.Workspace),
			Description: m.workspace.Catalog.Workspace.Description,
		},
		Models:      make([]dashboard.CatalogModel, 0, len(m.workspace.Catalog.SemanticModels)),
		MetricViews: make([]dashboard.CatalogMetricView, 0, len(m.workspace.Catalog.MetricViews)),
		Dashboards:  make([]dashboard.CatalogDashboard, 0, len(m.workspace.Catalog.Dashboards)),
	}
	modelTitles := map[string]string{}
	for _, model := range m.workspace.Catalog.SemanticModels {
		modelTitles[model.ID] = model.Title
		catalog.Models = append(catalog.Models, dashboard.CatalogModel{
			ID:          model.ID,
			Title:       model.Title,
			Description: model.Description,
		})
	}
	metricViewTitles := map[string]string{}
	for _, view := range m.workspace.Catalog.MetricViews {
		metricViewTitles[view.ID] = view.Title
		catalog.MetricViews = append(catalog.MetricViews, dashboard.CatalogMetricView{
			ID:            view.ID,
			Title:         view.Title,
			Description:   view.Description,
			SemanticModel: view.SemanticModel,
			ModelTitle:    modelTitles[view.SemanticModel],
		})
	}
	for _, report := range m.workspace.Catalog.Dashboards {
		pageCount := 0
		metricViews := []string{}
		metricViewNames := []string{}
		if loaded, ok := m.workspace.Dashboards[report.ID]; ok {
			pageCount = len(loaded.Pages)
			metricViews = append(metricViews, loaded.MetricViews...)
			for _, viewID := range loaded.MetricViews {
				if title := metricViewTitles[viewID]; title != "" {
					metricViewNames = append(metricViewNames, title)
				}
			}
		}
		catalog.Dashboards = append(catalog.Dashboards, dashboard.CatalogDashboard{
			ID:               report.ID,
			Title:            report.Title,
			Description:      report.Description,
			MetricViews:      metricViews,
			MetricViewTitles: metricViewNames,
			Tags:             append([]string{}, report.Tags...),
			PageCount:        pageCount,
		})
	}
	return catalog
}

func (m *DuckDBMetrics) metricViewSummary(id string) (dashboard.MetricViewSummary, bool) {
	view, ok := m.workspace.MetricViews[id]
	if !ok {
		return dashboard.MetricViewSummary{}, false
	}
	modelTitle := ""
	for _, model := range m.workspace.Catalog.SemanticModels {
		if model.ID == view.SemanticModel {
			modelTitle = model.Title
			break
		}
	}
	return dashboard.MetricViewSummary{
		ID:             view.ID,
		Title:          view.Title,
		Description:    view.Description,
		SemanticModel:  view.SemanticModel,
		ModelTitle:     modelTitle,
		BaseTable:      view.BaseTable,
		Timeseries:     view.Time.DefaultField,
		DimensionCount: len(view.Dimensions),
		MeasureCount:   len(view.Measures),
		DashboardCount: len(m.dashboardsForMetricView(id)),
	}, true
}

func (m *DuckDBMetrics) dashboardsForMetricView(id string) []semantic.CatalogDashboard {
	reports := []semantic.CatalogDashboard{}
	for _, report := range m.workspace.Catalog.Dashboards {
		loaded, ok := m.workspace.Dashboards[report.ID]
		if !ok || !contains(loaded.MetricViews, id) {
			continue
		}
		reports = append(reports, report)
	}
	return reports
}

func dashboardPageCount(report *semantic.Dashboard) int {
	if report == nil {
		return 0
	}
	return len(report.Pages)
}

func workspaceID(workspace semantic.CatalogWorkspace) string {
	if strings.TrimSpace(workspace.ID) != "" {
		return workspace.ID
	}
	return "libredash"
}

func workspaceTitle(workspace semantic.CatalogWorkspace) string {
	if strings.TrimSpace(workspace.Title) != "" {
		return workspace.Title
	}
	return "LibreDash Workspace"
}

func (m *DuckDBMetrics) reportRuntime(dashboardID string) (*semantic.Dashboard, *modelRuntime, error) {
	report, ok := m.workspace.Dashboards[dashboardID]
	if !ok {
		return nil, nil, fmt.Errorf("unknown dashboard %q", dashboardID)
	}
	view, ok := m.firstMetricView(report)
	if !ok {
		return nil, nil, fmt.Errorf("dashboard %q has no metrics views", dashboardID)
	}
	runtime, ok := m.runtimes[view.SemanticModel]
	if !ok {
		return nil, nil, fmt.Errorf("unknown semantic model %q", view.SemanticModel)
	}
	return report, runtime, nil
}

func (m *DuckDBMetrics) firstMetricView(report *semantic.Dashboard) (*semantic.MetricView, bool) {
	if report == nil || len(report.MetricViews) == 0 {
		return nil, false
	}
	view, ok := m.workspace.MetricViews[report.MetricViews[0]]
	return view, ok
}

func (m *DuckDBMetrics) QueryDashboard(ctx context.Context, dashboardID string, filters dashboard.Filters) (dashboard.Patch, error) {
	return m.QueryDashboardPage(ctx, dashboardID, "", filters)
}

func (m *DuckDBMetrics) QueryDashboardPage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters) (dashboard.Patch, error) {
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

func dashboardPage(report *semantic.Dashboard, pageID string) dashboard.Page {
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

func (m *DuckDBMetrics) QueryTable(ctx context.Context, dashboardID string, filters dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error) {
	return m.QueryTablePage(ctx, dashboardID, "", filters, request)
}

func (m *DuckDBMetrics) QueryTablePage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error) {
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

	totalRows, err := m.countRows(ctx, runtime, report, tableModel.MetricView, filters, "table", request.Table)
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

func (m *DuckDBMetrics) filterOptions(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, names []string) (map[string][]dashboard.FilterOption, error) {
	options := map[string][]dashboard.FilterOption{}
	names = append([]string{}, names...)
	sort.Strings(names)
	for _, name := range names {
		filter := report.Filters[name]
		if filter.Values.Source != "distinct" {
			continue
		}
		limit := filter.Values.Limit
		if limit <= 0 {
			limit = 200
		}
		if limit > 500 {
			limit = 500
		}
		plan, err := semanticquery.NewPlanner(runtime.model, m.workspace.MetricViews).Plan(semanticquery.Request{
			MetricView: filter.MetricView,
			Dimensions: []semanticquery.Field{{Field: filter.Dimension, Alias: "value"}},
			Sort:       []semanticquery.Sort{{Field: "value", Direction: "asc"}},
			Limit:      limit,
		})
		if err != nil {
			return nil, err
		}
		rows, err := runtime.db.QueryContext(ctx, plan.SQL, plan.Args...)
		if err != nil {
			return nil, err
		}
		values := []dashboard.FilterOption{}
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				rows.Close()
				return nil, err
			}
			values = append(values, dashboard.FilterOption{Value: value, Label: value})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		options[name] = values
	}
	return options, nil
}

func (m *DuckDBMetrics) semanticFilters(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, metricViewID string, filters dashboard.Filters, targetKind, targetID string) ([]semanticquery.Filter, error) {
	filters = filters.WithDefaults()
	result := []semanticquery.Filter{}
	for _, name := range sortedKeys(report.Filters) {
		filter := report.Filters[name]
		if filter.MetricView != metricViewID {
			continue
		}
		control, ok := filters.Controls[name]
		if !ok {
			continue
		}
		switch filter.Type {
		case "date_range":
			dateFilters := m.dateSemanticFilters(runtime, filter, control)
			result = append(result, dateFilters...)
		case "multi_select":
			if control.Operator != "in" || len(control.Values) == 0 {
				continue
			}
			values := make([]any, len(control.Values))
			for i, value := range control.Values {
				values[i] = value
			}
			result = append(result, semanticquery.Filter{Field: filter.Dimension, Operator: "in", Values: values})
		case "text":
			value := strings.TrimSpace(control.Value)
			if value == "" {
				continue
			}
			operator := control.Operator
			if operator == "" {
				operator = filter.DefaultOperator
			}
			result = append(result, semanticquery.Filter{Field: filter.Dimension, Operator: operator, Values: []any{value}})
		}
	}
	for _, selection := range filters.VisualSelections {
		if selection.VisualID == "" || len(selection.Values) == 0 {
			continue
		}
		if targetKind == "visual" && selection.VisualID == targetID {
			continue
		}
		sourceVisual, ok := report.Visuals[selection.VisualID]
		if !ok || !targetsSelection(sourceVisual.Interaction.Targets, targetKind, targetID) {
			continue
		}
		if selection.Operator != "" && selection.Operator != "in" {
			continue
		}
		metricView, ok := m.workspace.MetricViews[metricViewID]
		if !ok {
			continue
		}
		field, _, err := metricView.ResolveDimensionRef(selection.Field)
		if err != nil {
			continue
		}
		values := make([]any, len(selection.Values))
		for i, value := range selection.Values {
			values[i] = value
		}
		result = append(result, semanticquery.Filter{Field: field, Operator: "in", Values: values})
	}
	return result, nil
}

func (m *DuckDBMetrics) dateSemanticFilters(runtime *modelRuntime, filter semantic.FilterDefinition, control dashboard.FilterControl) []semanticquery.Filter {
	if control.From != "" || control.To != "" {
		result := []semanticquery.Filter{}
		if control.From != "" {
			result = append(result, semanticquery.Filter{Field: filter.Dimension, Operator: "greater_than_or_equal", Values: []any{control.From}})
		}
		if control.To != "" {
			result = append(result, semanticquery.Filter{Field: filter.Dimension, Operator: "less_than", Values: []any{control.To}})
		}
		return result
	}
	if control.Preset == "" || control.Preset == "all" {
		return nil
	}
	for _, preset := range filter.Presets {
		if preset.Value != control.Preset {
			continue
		}
		if preset.From != "" && preset.To != "" {
			return []semanticquery.Filter{
				{Field: filter.Dimension, Operator: "greater_than_or_equal", Values: []any{preset.From}},
				{Field: filter.Dimension, Operator: "less_than", Values: []any{preset.To}},
			}
		}
		if preset.RelativeDays > 0 {
			// The demo relative preset is anchored to the imported order timeline. Leave
			// it unbounded here rather than injecting physical SQL into semantic filters.
			return nil
		}
	}
	return nil
}

func (m *DuckDBMetrics) RefreshCache(ctx context.Context, modelID string) error {
	runtime, ok := m.runtimes[modelID]
	if !ok {
		return fmt.Errorf("unknown semantic model %q", modelID)
	}
	if runtime.missing != nil {
		return runtime.missing
	}
	if runtime.db == nil {
		return fmt.Errorf("DuckDB is not initialized")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.registerSourceViews(ctx, runtime); err != nil {
		return err
	}
	if err := m.materializeModelTables(ctx, runtime); err != nil {
		return err
	}
	runtime.lastRefresh = time.Now()
	return nil
}

func (m *DuckDBMetrics) validateFiles(runtime *modelRuntime) error {
	var missing []string
	for name, source := range runtime.model.Sources {
		if source.Path == "" {
			continue
		}
		connection := runtime.model.Connections[source.Connection]
		if connection.Kind != "local" {
			continue
		}
		file, err := m.resolveSourcePath(runtime.model, source)
		if err != nil {
			return fmt.Errorf("resolving local source %s: %w", name, err)
		}
		if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
			missing = append(missing, file)
		} else if err != nil {
			return err
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return &MissingDataError{DataDir: m.dataDir, Missing: missing}
	}
	return nil
}

func (m *DuckDBMetrics) registerSourceViews(ctx context.Context, runtime *modelRuntime) error {
	if _, err := runtime.db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS raw"); err != nil {
		return err
	}
	if _, err := runtime.db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS model"); err != nil {
		return err
	}

	if err := m.prepareSourceRuntime(ctx, runtime); err != nil {
		return err
	}

	for _, name := range sortedKeys(runtime.model.Sources) {
		source := runtime.model.Sources[name]
		if err := validateIdentifier(name); err != nil {
			return err
		}
		relation, err := m.sourceRelation(runtime.model, source)
		if err != nil {
			return fmt.Errorf("compiling source %s: %w", name, err)
		}
		stmt := fmt.Sprintf("CREATE OR REPLACE VIEW raw.%s AS %s", name, relation)
		if _, err := runtime.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("registering source %s: %w", name, err)
		}
	}
	return nil
}

func (m *DuckDBMetrics) materializeModelTables(ctx context.Context, runtime *modelRuntime) error {
	for _, name := range runtime.model.TableNames() {
		if err := validateIdentifier(name); err != nil {
			return err
		}
		table := runtime.model.Tables[name]
		sourceSQL := table.Transform.SQL
		if table.Source != "" {
			if err := validateIdentifier(table.Source); err != nil {
				return err
			}
			sourceSQL = "SELECT * FROM raw." + table.Source
		}
		stmt := fmt.Sprintf("CREATE OR REPLACE TABLE model.%s AS %s", name, sourceSQL)
		if _, err := runtime.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("materializing model.%s: %w", name, err)
		}
	}
	return nil
}

func (m *DuckDBMetrics) visuals(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, filters dashboard.Filters, keys []string) (map[string]dashboard.Visual, error) {
	visuals := make(map[string]dashboard.Visual, len(keys))
	for _, key := range keys {
		visual, ok := report.Visuals[key]
		if !ok {
			return nil, fmt.Errorf("page references unknown visual %q", key)
		}
		data, err := m.visualData(ctx, runtime, report, key, visual, filters)
		if err != nil {
			return nil, err
		}
		metricView := m.workspace.MetricViews[visual.MetricView]
		measureName := visual.Query.Measures[0].Field
		measure := metricView.Measures[measureName]
		title := visual.Title
		if title == "" {
			title = measure.Label
		}
		if title == "" {
			title = measureName
		}
		unit := measure.Unit
		if len(visual.Query.Measures) > 1 {
			unit = ""
		}
		series := []string{}
		if !visual.Query.Series.IsZero() {
			series = append(series, visual.Query.Series.Field)
		}
		rendererOptions := map[string]map[string]any{}
		for renderer, options := range visual.RendererOptions {
			if typed, ok := options.(map[string]any); ok {
				rendererOptions[renderer] = typed
			}
		}
		visualType := visual.Type
		if visualType == "" && visual.KindOrDefault() == "kpi" {
			visualType = "kpi"
		}
		visuals[key] = dashboard.Visual{
			Version:         3,
			ID:              key,
			Kind:            visual.KindOrDefault(),
			Shape:           visual.ShapeOrDefault(),
			Renderer:        visual.RendererOrDefault(),
			Type:            visualType,
			Title:           title,
			Unit:            unit,
			Format:          measure.Format,
			Field:           visual.Interaction.Field,
			Dimensions:      displayFields(queryDimensionFields(visual.Query.Dimensions)),
			Measure:         displayField(measureName),
			Measures:        displayFields(queryMeasureFields(visual.Query.Measures)),
			Series:          series,
			Options:         visual.CoreOptions(),
			RendererOptions: rendererOptions,
			Selection:       selectedValues(filters, key),
			Data:            data,
		}
	}
	return visuals, nil
}

func (m *DuckDBMetrics) visualData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	switch visual.ShapeOrDefault() {
	case "single_value":
		return m.singleValueData(ctx, runtime, report, visualID, visual, filters)
	case "category_multi_measure":
		return m.categoryMultiMeasureData(ctx, runtime, report, visualID, visual, filters)
	case "category_delta":
		return m.categoryDeltaData(ctx, runtime, report, visualID, visual, filters)
	case "binned_measure":
		return m.binnedMeasureData(ctx, runtime, report, visualID, visual, filters)
	case "hierarchy":
		return m.hierarchyData(ctx, runtime, report, visualID, visual, filters)
	case "matrix":
		return m.matrixData(ctx, runtime, report, visualID, visual, filters)
	case "graph":
		return m.graphData(ctx, runtime, report, visualID, visual, filters)
	case "geo":
		return m.geoData(ctx, runtime, report, visualID, visual, filters)
	case "ohlc":
		return m.ohlcData(ctx, runtime, report, visualID, visual, filters)
	case "distribution":
		return m.distributionData(ctx, runtime, report, visualID, visual, filters)
	default:
		return m.categoryData(ctx, runtime, report, visualID, visual, filters)
	}
}

func (m *DuckDBMetrics) categoryData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, visual.MetricView, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	dimensions := []semanticquery.Field{fieldRef(visual.Query.Dimensions[0].Field, "label")}
	columns := []string{"label", "value"}
	if !visual.Query.Series.IsZero() {
		dimensions = append(dimensions, fieldRef(visual.Query.Series.Field, "series"))
		columns = []string{"label", "series", "value"}
	}
	data, err := m.querySemanticDatums(ctx, runtime, semanticquery.Request{
		MetricView: visual.MetricView,
		Dimensions: dimensions,
		Measures:   []semanticquery.Field{fieldRef(visual.Query.Measures[0].Field, "value")},
		Filters:    queryFilters,
		Sort:       visualSorts(visual),
		Limit:      visual.Query.Limit,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range data {
		for _, column := range columns {
			if _, ok := row[column]; !ok && column == "series" {
				row[column] = ""
			}
		}
	}
	markSelected(data, "label", selectedValues(filters, visualID))
	return data, nil
}

func (m *DuckDBMetrics) categoryMultiMeasureData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, visual.MetricView, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	metricView := m.workspace.MetricViews[visual.MetricView]
	data := []dashboard.Datum{}

	for _, measureName := range visual.Query.Measures {
		rows, err := m.querySemanticDatums(ctx, runtime, semanticquery.Request{
			MetricView: visual.MetricView,
			Dimensions: []semanticquery.Field{fieldRef(visual.Query.Dimensions[0].Field, "label")},
			Measures:   []semanticquery.Field{fieldRef(measureName.Field, "value")},
			Filters:    queryFilters,
			Sort:       visualSorts(visual),
			Limit:      visual.Query.Limit,
		})
		if err != nil {
			return nil, err
		}
		measure := metricView.Measures[measureName.Field]
		for _, row := range rows {
			row["series"] = measureLabel(measureName.Field, measure)
		}
		data = append(data, rows...)
	}
	markSelected(data, "label", selectedValues(filters, visualID))
	return data, nil
}

func (m *DuckDBMetrics) categoryDeltaData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	rows, err := m.categoryData(ctx, runtime, report, visualID, visual, filters)
	if err != nil {
		return nil, err
	}
	cumulative := 0.0
	for _, row := range rows {
		value := datumFloat(row["value"])
		start := cumulative
		cumulative += value
		row["start"] = round(start)
		row["end"] = round(cumulative)
		row["positive"] = value >= 0
	}
	return rows, nil
}

func (m *DuckDBMetrics) binnedMeasureData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	metricView := m.workspace.MetricViews[visual.MetricView]
	measure := metricView.Measures[visual.Query.Measures[0].Field]
	columnExpr, err := rawValueExpression(measure)
	if err != nil {
		return nil, err
	}
	columnExpr = modelTableExpr(columnExpr, metricView.BaseTable, "e")
	columnExpr = "CAST(" + columnExpr + " AS DOUBLE)"
	where, args := m.visualWhere(runtime, report, visual, filters, visualID)
	binCount := optionInt(visual.Options, "bin_count", 20, 5, 60)

	var minValue, maxValue sql.NullFloat64
	boundsQuery := fmt.Sprintf("SELECT MIN(%s), MAX(%s) FROM model.%s e WHERE %s AND %s IS NOT NULL", columnExpr, columnExpr, metricView.BaseTable, where, columnExpr)
	if err := runtime.db.QueryRowContext(ctx, boundsQuery, args...).Scan(&minValue, &maxValue); err != nil {
		return nil, err
	}
	if !minValue.Valid || !maxValue.Valid {
		return []dashboard.Datum{}, nil
	}
	if minValue.Float64 == maxValue.Float64 {
		var count int
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM model.%s e WHERE %s AND %s IS NOT NULL", metricView.BaseTable, where, columnExpr)
		if err := runtime.db.QueryRowContext(ctx, countQuery, args...).Scan(&count); err != nil {
			return nil, err
		}
		return []dashboard.Datum{{
			"label":    formatBinLabel(minValue.Float64, maxValue.Float64),
			"binStart": round(minValue.Float64),
			"binEnd":   round(maxValue.Float64),
			"value":    count,
		}}, nil
	}

	bucketExpr := fmt.Sprintf("LEAST(%d, CAST(FLOOR(((%s - ?) / NULLIF(? - ?, 0)) * ?) AS INTEGER))", binCount-1, columnExpr)
	query := fmt.Sprintf(`
SELECT %s AS bucket, COUNT(*) AS value
FROM model.%s e
WHERE %s AND %s IS NOT NULL
GROUP BY bucket
ORDER BY bucket ASC`, bucketExpr, metricView.BaseTable, where, columnExpr)
	queryArgs := append([]any{minValue.Float64, maxValue.Float64, minValue.Float64, binCount}, args...)
	rows, err := runtime.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	width := (maxValue.Float64 - minValue.Float64) / float64(binCount)
	data := []dashboard.Datum{}
	for rows.Next() {
		var bucket int
		var count int
		if err := rows.Scan(&bucket, &count); err != nil {
			return nil, err
		}
		start := minValue.Float64 + float64(bucket)*width
		end := start + width
		data = append(data, dashboard.Datum{
			"label":    formatBinLabel(start, end),
			"binStart": round(start),
			"binEnd":   round(end),
			"value":    count,
		})
	}
	return data, rows.Err()
}

func (m *DuckDBMetrics) hierarchyData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, visual.MetricView, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	dimensions := make([]semanticquery.Field, 0, len(visual.Query.Dimensions))
	levelAliases := make([]string, 0, len(visual.Query.Dimensions))
	for index, dimensionName := range visual.Query.Dimensions {
		alias := fmt.Sprintf("level_%d", index)
		dimensions = append(dimensions, fieldRef(dimensionName.Field, alias))
		levelAliases = append(levelAliases, alias)
	}
	plan, err := semanticquery.NewPlanner(runtime.model, m.workspace.MetricViews).Plan(semanticquery.Request{
		MetricView: visual.MetricView,
		Dimensions: dimensions,
		Measures:   []semanticquery.Field{fieldRef(visual.Query.Measures[0].Field, "value")},
		Filters:    queryFilters,
		Sort:       visualSorts(visual),
		Limit:      visual.Query.Limit,
	})
	if err != nil {
		return nil, err
	}
	rows, err := runtime.db.QueryContext(ctx, plan.SQL, plan.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]any, len(levelAliases)+1)
	scans := make([]any, len(values))
	for index := range values {
		scans[index] = &values[index]
	}
	data := []dashboard.Datum{}
	for rows.Next() {
		if err := rows.Scan(scans...); err != nil {
			return nil, err
		}
		path := make([]string, 0, len(levelAliases))
		for index := range levelAliases {
			item := normalizeDatumValue(values[index])
			if item == nil || fmt.Sprint(item) == "" {
				continue
			}
			path = append(path, fmt.Sprint(item))
		}
		data = append(data, dashboard.Datum{
			"path":  path,
			"value": normalizeDatumValue(values[len(values)-1]),
		})
	}
	return data, rows.Err()
}

func (m *DuckDBMetrics) singleValueData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	metricView := m.workspace.MetricViews[visual.MetricView]
	measureName := visual.Query.Measures[0].Field
	title := visual.Title
	if title == "" {
		title = metricView.Measures[measureName].Label
	}
	if title == "" {
		title = measureName
	}
	queryFilters, err := m.semanticFilters(ctx, runtime, report, visual.MetricView, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	dimensions := []semanticquery.Field{}
	if len(visual.Query.Dimensions) == 1 {
		dimensions = append(dimensions, fieldRef(visual.Query.Dimensions[0].Field, "label"))
	}
	sorts := visualSorts(visual)
	if len(dimensions) == 0 {
		sorts = nil
	}
	data, err := m.querySemanticDatums(ctx, runtime, semanticquery.Request{
		MetricView: visual.MetricView,
		Dimensions: dimensions,
		Measures:   []semanticquery.Field{fieldRef(measureName, "value")},
		Filters:    queryFilters,
		Sort:       sorts,
		Limit:      visual.Query.Limit,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range data {
		if _, ok := row["label"]; !ok {
			row["label"] = title
		}
		row["series"] = ""
	}
	markSelected(data, "label", selectedValues(filters, visualID))
	return data, nil
}

func (m *DuckDBMetrics) matrixData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	return m.dimensionPairData(ctx, runtime, report, visualID, visual, filters, "row", "column")
}

func (m *DuckDBMetrics) graphData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	return m.dimensionPairData(ctx, runtime, report, visualID, visual, filters, "source", "target")
}

func (m *DuckDBMetrics) dimensionPairData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters, leftAlias, rightAlias string) ([]dashboard.Datum, error) {
	rightSQLAlias := rightAlias
	if rightAlias == "column" {
		rightSQLAlias = "chart_column"
	}
	queryFilters, err := m.semanticFilters(ctx, runtime, report, visual.MetricView, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	data, err := m.querySemanticDatums(ctx, runtime, semanticquery.Request{
		MetricView: visual.MetricView,
		Dimensions: []semanticquery.Field{
			fieldRef(visual.Query.Dimensions[0].Field, leftAlias),
			fieldRef(visual.Query.Dimensions[1].Field, rightSQLAlias),
		},
		Measures: []semanticquery.Field{fieldRef(visual.Query.Measures[0].Field, "value")},
		Filters:  queryFilters,
		Sort:     visualSorts(visual),
		Limit:    visual.Query.Limit,
	})
	if err != nil {
		return nil, err
	}
	if rightAlias == "column" {
		for _, row := range data {
			row["column"] = row[rightSQLAlias]
			delete(row, rightSQLAlias)
		}
	}
	if leftAlias == "row" {
		markSelected(data, "row", selectedValues(filters, visualID))
	}
	return data, nil
}

func (m *DuckDBMetrics) geoData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, visual.MetricView, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	data, err := m.querySemanticDatums(ctx, runtime, semanticquery.Request{
		MetricView: visual.MetricView,
		Dimensions: []semanticquery.Field{fieldRef(visual.Query.Dimensions[0].Field, "name")},
		Measures:   []semanticquery.Field{fieldRef(visual.Query.Measures[0].Field, "value")},
		Filters:    queryFilters,
		Sort:       visualSorts(visual),
		Limit:      visual.Query.Limit,
	})
	if err != nil {
		return nil, err
	}
	markSelected(data, "name", selectedValues(filters, visualID))
	return data, nil
}

func (m *DuckDBMetrics) ohlcData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, visual.MetricView, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	return m.querySemanticDatums(ctx, runtime, semanticquery.Request{
		MetricView: visual.MetricView,
		Dimensions: []semanticquery.Field{fieldRef(visual.Query.Dimensions[0].Field, "label")},
		Measures: []semanticquery.Field{
			fieldRef(visual.Query.Measures[0].Field, "open"),
			fieldRef(visual.Query.Measures[1].Field, "close"),
			fieldRef(visual.Query.Measures[2].Field, "low"),
			fieldRef(visual.Query.Measures[3].Field, "high"),
		},
		Filters: queryFilters,
		Sort:    visualSorts(visual),
		Limit:   visual.Query.Limit,
	})
}

func (m *DuckDBMetrics) distributionData(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	measureField := visual.Query.Measures[0].Field
	metricView := m.workspace.MetricViews[visual.MetricView]
	measure := metricView.Measures[measureField]
	rawExpr, err := rawValueExpression(measure)
	if err != nil {
		return nil, err
	}
	rawExpr = modelTableExpr(rawExpr, metricView.BaseTable, "e")
	dimension := metricView.Dimensions[visual.Query.Dimensions[0].Field]
	labelExpr := dimensionExpression(dimension, "e")
	where, args := m.filterWhere("e", runtime, report, visual.MetricView, filters, "visual", visualID)
	query := fmt.Sprintf(`
SELECT %s AS label,
       MIN(%s) AS min,
       quantile_cont(%s, 0.25) AS q1,
       median(%s) AS median,
       quantile_cont(%s, 0.75) AS q3,
       MAX(%s) AS max
FROM model.%s e
WHERE %s AND %s IS NOT NULL
GROUP BY label
ORDER BY %s`, labelExpr, rawExpr, rawExpr, rawExpr, rawExpr, rawExpr, metricView.BaseTable, where, rawExpr, m.visualOrderBy(runtime.model, visual))
	if visual.Query.Limit > 0 {
		query += fmt.Sprintf("\nLIMIT %d", visual.Query.Limit)
	}
	return m.queryDatums(ctx, runtime, query, []string{"label", "min", "q1", "median", "q3", "max"}, args...)
}

func (m *DuckDBMetrics) visualWhere(runtime *modelRuntime, report *semantic.Dashboard, visual semantic.Visual, filters dashboard.Filters, visualID string) (string, []any) {
	dataset := m.workspace.MetricViews[visual.MetricView]
	where, args := m.filterWhere("e", runtime, report, visual.MetricView, filters, "visual", visualID)
	for _, dimensionName := range visualQueryDimensions(visual) {
		if dimensionName == "" {
			continue
		}
		if dimension := dataset.Dimensions[dimensionName]; dimension.Where != "" {
			where = fmt.Sprintf("(%s) AND (%s)", where, dimensionWhere(dimension, "e"))
		}
	}
	return where, args
}

func visualQueryDimensions(visual semantic.Visual) []string {
	dimensions := queryDimensionFields(visual.Query.Dimensions)
	if !visual.Query.Series.IsZero() {
		dimensions = append(dimensions, visual.Query.Series.Field)
	}
	return dimensions
}

func (m *DuckDBMetrics) queryDatums(ctx context.Context, runtime *modelRuntime, query string, columns []string, args ...any) ([]dashboard.Datum, error) {
	rows, err := runtime.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]any, len(columns))
	scans := make([]any, len(columns))
	for index := range values {
		scans[index] = &values[index]
	}
	data := []dashboard.Datum{}
	for rows.Next() {
		if err := rows.Scan(scans...); err != nil {
			return nil, err
		}
		row := dashboard.Datum{}
		for index, column := range columns {
			row[column] = normalizeDatumValue(values[index])
		}
		data = append(data, row)
	}
	return data, rows.Err()
}

func (m *DuckDBMetrics) querySemanticDatums(ctx context.Context, runtime *modelRuntime, request semanticquery.Request) ([]dashboard.Datum, error) {
	plan, err := semanticquery.NewPlanner(runtime.model, m.workspace.MetricViews).Plan(request)
	if err != nil {
		return nil, err
	}
	return m.queryDatums(ctx, runtime, plan.SQL, plan.Columns, plan.Args...)
}

func visualSemanticFilters(ctx context.Context, m *DuckDBMetrics, runtime *modelRuntime, report *semantic.Dashboard, visual semantic.Visual, filters dashboard.Filters, visualID string) ([]semanticquery.Filter, error) {
	return m.semanticFilters(ctx, runtime, report, visual.MetricView, filters, "visual", visualID)
}

func fieldRef(field string, alias string) semanticquery.Field {
	return semanticquery.Field{Field: field, Alias: alias}
}

func queryDimensionFields(dimensions []semantic.FieldRef) []string {
	fields := make([]string, len(dimensions))
	for i, dimension := range dimensions {
		fields[i] = dimension.Field
	}
	return fields
}

func queryMeasureFields(measures []semantic.FieldRef) []string {
	fields := make([]string, len(measures))
	for i, measure := range measures {
		fields[i] = measure.Field
	}
	return fields
}

func displayFields(fields []string) []string {
	values := make([]string, len(fields))
	for i, field := range fields {
		values[i] = displayField(field)
	}
	return values
}

func displayField(field string) string {
	parts := strings.Split(field, ".")
	return parts[len(parts)-1]
}

func visualSorts(visual semantic.Visual) []semanticquery.Sort {
	if len(visual.Query.Sort) == 0 {
		return []semanticquery.Sort{{Field: defaultSortColumn(visual), Direction: "asc"}}
	}
	sorts := make([]semanticquery.Sort, 0, len(visual.Query.Sort))
	for _, sort := range visual.Query.Sort {
		field := sort.Field
		if field == "" {
			field = defaultSortColumn(visual)
		}
		if field != "value" && field != "series" {
			if len(visual.Query.Dimensions) > 0 && field == visual.Query.Dimensions[0].Field {
				field = defaultSortColumn(visual)
			}
			if !visual.Query.Series.IsZero() && field == visual.Query.Series.Field {
				field = "series"
			}
		}
		sorts = append(sorts, semanticquery.Sort{Field: field, Direction: sort.Direction})
	}
	return sorts
}

func (m *DuckDBMetrics) countRows(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, metricViewID string, filters dashboard.Filters, targetKind, targetID string) (int, error) {
	source, err := m.metricViewSource(metricViewID)
	if err != nil {
		return 0, err
	}
	where, args := m.filterWhere("e", runtime, report, metricViewID, filters, targetKind, targetID)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s e WHERE %s", source, where)

	var total int
	if err := runtime.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func measureAggregateExpr(measure semantic.MetricMeasure) (string, error) {
	if strings.TrimSpace(measure.Expression) == "" {
		return "", fmt.Errorf("measure %q is missing expression", measure.Label)
	}
	return measure.Expression, nil
}

func rawValueExpression(measure semantic.MetricMeasure) (string, error) {
	expr := strings.TrimSpace(measure.Expression)
	if expr == "" {
		return "", fmt.Errorf("measure %q is missing expression", measure.Label)
	}
	if matches := aggregateWrapperPattern.FindStringSubmatch(expr); len(matches) == 2 {
		return strings.TrimSpace(matches[1]), nil
	}
	if strings.Contains(expr, "(") {
		return "", fmt.Errorf("measure %q cannot be used as a raw value expression", measure.Label)
	}
	return expr, nil
}

func measureLabel(name string, measure semantic.MetricMeasure) string {
	if strings.TrimSpace(measure.Label) != "" {
		return measure.Label
	}
	return name
}

func optionInt(options map[string]any, key string, fallback, minValue, maxValue int) int {
	if options == nil {
		return fallback
	}
	var value int
	switch typed := options[key].(type) {
	case int:
		value = typed
	case int64:
		value = int(typed)
	case float64:
		value = int(typed)
	case string:
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return fallback
		}
		value = parsed
	default:
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func datumFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		parsed, _ := strconv.ParseFloat(typed, 64)
		return parsed
	default:
		return 0
	}
}

func formatBinLabel(start, end float64) string {
	if math.Abs(start-end) < 0.000001 {
		return strconv.FormatFloat(round(start), 'f', -1, 64)
	}
	return fmt.Sprintf("%s-%s", strconv.FormatFloat(round(start), 'f', -1, 64), strconv.FormatFloat(round(end), 'f', -1, 64))
}

func (m *DuckDBMetrics) visualOrderBy(model *semantic.Model, visual semantic.Visual) string {
	if len(visual.Query.Sort) == 0 {
		return "label ASC"
	}
	metricView := m.workspace.MetricViews[visual.MetricView]
	parts := make([]string, 0, len(visual.Query.Sort))
	for _, sortSpec := range visual.Query.Sort {
		direction := "ASC"
		if strings.EqualFold(sortSpec.Direction, "desc") {
			direction = "DESC"
		}
		expr := sortSpec.Expr
		if expr == "" {
			expr = m.sortExpression(metricView, visual, sortSpec.Field)
		}
		if expr == "" {
			expr = "label"
		}
		parts = append(parts, expr+" "+direction)
	}
	return strings.Join(parts, ", ")
}

func (m *DuckDBMetrics) sortExpression(metricView *semantic.MetricView, visual semantic.Visual, field string) string {
	if field == "" {
		return defaultSortColumn(visual)
	}
	if field == "value" || field == visual.Query.Measures[0].Field {
		return "value"
	}
	if field == visual.Query.Series.Field {
		return "series"
	}
	if metricView == nil {
		return ""
	}
	if dimension, ok := metricView.Dimensions[field]; ok {
		if dimension.OrderExpr != "" {
			return dimension.OrderExpr
		}
		for index, dimensionName := range visual.Query.Dimensions {
			if field == dimensionName.Field {
				return dimensionSortColumn(visual.ShapeOrDefault(), index)
			}
		}
		return dimensionExpression(dimension, "e")
	}
	return ""
}

func defaultSortColumn(visual semantic.Visual) string {
	switch visual.ShapeOrDefault() {
	case "matrix":
		return "row"
	case "graph":
		return "source"
	case "geo":
		return "name"
	case "hierarchy":
		return "value"
	default:
		return "label"
	}
}

func dimensionSortColumn(shape string, index int) string {
	switch shape {
	case "matrix":
		if index == 1 {
			return "chart_column"
		}
		return "row"
	case "graph":
		if index == 1 {
			return "target"
		}
		return "source"
	case "geo":
		return "name"
	case "hierarchy":
		return fmt.Sprintf("level_%d", index)
	default:
		return "label"
	}
}

func (m *DuckDBMetrics) filterWhere(alias string, runtime *modelRuntime, report *semantic.Dashboard, metricViewID string, filters dashboard.Filters, targetKind, targetID string) (string, []any) {
	filters = filters.WithDefaults()
	conditions := []string{"1 = 1"}
	args := []any{}

	for _, name := range sortedKeys(report.Filters) {
		filter := report.Filters[name]
		if filter.MetricView != metricViewID {
			continue
		}
		control, ok := filters.Controls[name]
		if !ok {
			continue
		}
		metricView, ok := m.workspace.MetricViews[filter.MetricView]
		if !ok {
			continue
		}
		dimension, ok := metricView.Dimensions[filter.Dimension]
		if !ok {
			continue
		}
		expr := dimensionExpression(dimension, alias)
		switch filter.Type {
		case "date_range":
			condition, conditionArgs := m.dateFilterCondition(runtime, filter, control, expr)
			if condition != "" {
				conditions = append(conditions, condition)
				args = append(args, conditionArgs...)
			}
		case "multi_select":
			if control.Operator != "in" || len(control.Values) == 0 {
				continue
			}
			placeholders := make([]string, 0, len(control.Values))
			for _, value := range control.Values {
				placeholders = append(placeholders, "?")
				args = append(args, value)
			}
			conditions = append(conditions, expr+" IN ("+strings.Join(placeholders, ", ")+")")
		case "text":
			value := strings.TrimSpace(control.Value)
			if value == "" {
				continue
			}
			switch control.Operator {
			case "equals":
				conditions = append(conditions, "lower("+expr+") = lower(?)")
				args = append(args, value)
			case "starts_with":
				conditions = append(conditions, "lower("+expr+") LIKE lower(?)")
				args = append(args, value+"%")
			case "not_contains":
				conditions = append(conditions, "lower("+expr+") NOT LIKE lower(?)")
				args = append(args, "%"+value+"%")
			default:
				conditions = append(conditions, "lower("+expr+") LIKE lower(?)")
				args = append(args, "%"+value+"%")
			}
		}
	}

	for _, selection := range filters.VisualSelections {
		if selection.VisualID == "" || len(selection.Values) == 0 {
			continue
		}
		if targetKind == "visual" && selection.VisualID == targetID {
			continue
		}
		sourceVisual, ok := report.Visuals[selection.VisualID]
		if !ok || !targetsSelection(sourceVisual.Interaction.Targets, targetKind, targetID) {
			continue
		}
		if selection.Operator != "" && selection.Operator != "in" {
			continue
		}
		metricView, ok := m.workspace.MetricViews[metricViewID]
		if !ok {
			continue
		}
		field, dimension, err := metricView.ResolveDimensionRef(selection.Field)
		if err != nil {
			continue
		}
		_ = field
		placeholders := make([]string, 0, len(selection.Values))
		for _, value := range selection.Values {
			placeholders = append(placeholders, "?")
			args = append(args, value)
		}
		conditions = append(conditions, dimensionExpression(dimension, alias)+" IN ("+strings.Join(placeholders, ", ")+")")
	}

	return strings.Join(conditions, " AND "), args
}

func (m *DuckDBMetrics) dateFilterCondition(runtime *modelRuntime, filter semantic.FilterDefinition, control dashboard.FilterControl, expr string) (string, []any) {
	if control.From != "" || control.To != "" {
		conditions := []string{}
		args := []any{}
		if control.From != "" {
			conditions = append(conditions, expr+" >= CAST(? AS TIMESTAMP)")
			args = append(args, control.From)
		}
		if control.To != "" {
			conditions = append(conditions, expr+" < CAST(? AS TIMESTAMP) + INTERVAL 1 DAY")
			args = append(args, control.To)
		}
		return strings.Join(conditions, " AND "), args
	}
	if control.Preset == "" || control.Preset == "all" {
		return "", nil
	}
	for _, preset := range filter.Presets {
		if preset.Value != control.Preset {
			continue
		}
		if preset.RelativeDays > 0 {
			source, err := m.metricViewSource(filter.MetricView)
			if err != nil {
				return "", nil
			}
			metricView := m.workspace.MetricViews[filter.MetricView]
			dimension := metricView.Dimensions[filter.Dimension]
			sourceExpr := dimensionExpression(dimension, "recent")
			return fmt.Sprintf("%s >= (SELECT max(%s) - INTERVAL %d DAY FROM %s recent)", expr, sourceExpr, preset.RelativeDays, source), nil
		}
		if preset.From != "" && preset.To != "" {
			return expr + " >= CAST(? AS TIMESTAMP) AND " + expr + " < CAST(? AS TIMESTAMP)", []any{preset.From, preset.To}
		}
	}
	return "", nil
}

func targetsSelection(targets semantic.InteractionTargets, targetKind, targetID string) bool {
	switch targetKind {
	case "visual":
		return contains(targets.Visuals, targetID)
	case "table":
		return contains(targets.Tables, targetID)
	default:
		return false
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func dimensionExpression(dimension semantic.MetricDimension, alias string) string {
	expr := dimension.SQLExpression()
	if identifierPattern.MatchString(expr) {
		return alias + "." + expr
	}
	return modelTableExpr(expr, dimension.Table, alias)
}

func dimensionWhere(dimension semantic.MetricDimension, alias string) string {
	if dimension.Where == "" {
		return ""
	}
	return strings.ReplaceAll(dimension.Where, "{alias}", alias)
}

func modelTableExpr(expr, table, alias string) string {
	expr = strings.ReplaceAll(expr, table+".", alias+".")
	return strings.ReplaceAll(expr, "{alias}", alias)
}

func selectedValues(filters dashboard.Filters, visualID string) []string {
	for _, selection := range filters.VisualSelections {
		if selection.VisualID == visualID {
			values := make([]string, len(selection.Values))
			copy(values, selection.Values)
			return values
		}
	}
	return []string{}
}

func markSelected(data []dashboard.Datum, key string, values []string) {
	if len(values) == 0 {
		return
	}
	selected := make(map[string]struct{}, len(values))
	for _, value := range values {
		selected[value] = struct{}{}
	}
	for _, row := range data {
		value, ok := row[key]
		if !ok {
			continue
		}
		if _, ok := selected[fmt.Sprint(value)]; ok {
			row["selected"] = true
		}
	}
}

func normalizeDatumValue(value any) any {
	switch typed := normalizeDBValue(value).(type) {
	case float64:
		return round(typed)
	case float32:
		return round(float64(typed))
	default:
		return typed
	}
}

func normalizeDBValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format("2006-01-02")
	case float32:
		return round(float64(typed))
	case float64:
		return round(typed)
	default:
		return typed
	}
}

func (m *DuckDBMetrics) metricViewSource(name string) (string, error) {
	view, ok := m.workspace.MetricViews[name]
	if !ok {
		return "", fmt.Errorf("unknown metrics view %q", name)
	}
	return modelSource(view.BaseTable)
}

func modelSource(name string) (string, error) {
	if err := validateIdentifier(name); err != nil {
		return "", err
	}
	return "model." + name, nil
}

func validateIdentifier(value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("invalid identifier %q", value)
	}
	return nil
}

func discoverCatalogPath() (string, error) {
	candidates := []string{
		filepath.Join("dashboards", "catalog.yaml"),
		filepath.Join("..", "dashboards", "catalog.yaml"),
		filepath.Join("..", "..", "dashboards", "catalog.yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find dashboards/catalog.yaml")
}

func duckDBPath(dataDir, modelID string) string {
	if path := os.Getenv("LIBREDASH_DUCKDB_PATH"); path != "" {
		return path
	}
	return filepath.Join(dataDir, "libredash-"+modelID+".duckdb")
}

func sqlString(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(path), "'", "''")
}

func modelGraph(model *semantic.Model, metricViews map[string]*semantic.MetricView) dashboard.ModelGraph {
	graph := dashboard.ModelGraph{
		Name:  model.Name,
		Title: model.Title,
		Stats: dashboard.ModelStats{
			Sources:       len(model.Sources),
			ModelTables:   len(model.Tables),
			Metrics:       measureCount(model.Name, metricViews),
			Visuals:       0,
			ReportTables:  0,
			Relationships: len(model.Relationships),
		},
	}

	for _, name := range sortedKeys(model.Sources) {
		source := model.Sources[name]
		sourceKind := source.Kind()
		meta := []dashboard.ModelMeta{
			{Label: "Kind", Value: sourceKind},
			{Label: "Schema", Value: "raw"},
		}
		if source.Format != "" {
			meta = append(meta, dashboard.ModelMeta{Label: "Format", Value: source.Format})
		}
		if source.Path != "" {
			meta = append(meta, dashboard.ModelMeta{Label: "Path", Value: source.Path})
		}
		if source.Object != "" {
			meta = append(meta, dashboard.ModelMeta{Label: "Object", Value: source.Object})
		}
		if source.Connection != "" {
			meta = append(meta, dashboard.ModelMeta{Label: "Connection", Value: source.Connection})
			if connection, ok := model.Connections[source.Connection]; ok {
				meta = append(meta, dashboard.ModelMeta{Label: "Connection Kind", Value: connection.Kind})
			}
		}
		graph.Nodes = append(graph.Nodes, dashboard.ModelNode{
			ID:     nodeID("source", name),
			Label:  name,
			Kind:   "source",
			Schema: "raw",
			Fields: []dashboard.ModelField{{Name: source.Description(), Role: source.Role()}},
			Meta:   meta,
		})
	}

	for _, name := range sortedKeys(model.Tables) {
		table := model.Tables[name]
		graph.Nodes = append(graph.Nodes, dashboard.ModelNode{
			ID:          nodeID("model_table", name),
			Label:       name,
			Kind:        "model_table",
			Schema:      "model",
			Description: table.Description,
			Fields:      modelTableFields(table),
			Meta: []dashboard.ModelMeta{
				{Label: "Mode", Value: "DuckDB import"},
				{Label: "Kind", Value: table.Kind},
				{Label: "Grain", Value: table.Grain},
				{Label: "Schema", Value: "model"},
			},
		})
		if table.Source != "" {
			graph.Edges = append(graph.Edges, dashboard.ModelEdge{
				ID:     "source_" + table.Source + "_to_model_table_" + name,
				Source: nodeID("source", table.Source),
				Target: nodeID("model_table", name),
				Label:  "backs",
				Kind:   "materialization",
			})
		}
	}

	for _, relationship := range model.Relationships {
		fromTable, fromField := modelEndpoint(relationship.From)
		toTable, toField := modelEndpoint(relationship.To)
		graph.Edges = append(graph.Edges, dashboard.ModelEdge{
			ID:          relationship.ID,
			Source:      nodeID("model_table", fromTable),
			Target:      nodeID("model_table", toTable),
			Label:       fromField + " -> " + toField,
			Kind:        "relationship",
			SourceField: fromField,
			TargetField: toField,
			Cardinality: relationship.Cardinality,
		})
	}

	for _, name := range sortedKeys(metricViews) {
		view := metricViews[name]
		if view.SemanticModel != model.Name {
			continue
		}
		fields := make([]dashboard.ModelField, 0, len(view.Dimensions)+len(view.Measures))
		for _, dimension := range sortedKeys(view.Dimensions) {
			fields = append(fields, dashboard.ModelField{Name: dimension, Role: "dimension"})
		}
		for _, measure := range sortedKeys(view.Measures) {
			fields = append(fields, dashboard.ModelField{Name: measure, Role: "measure"})
		}
		graph.Nodes = append(graph.Nodes, dashboard.ModelNode{
			ID:          nodeID("metrics_view", name),
			Label:       view.Title,
			Kind:        "metrics_view",
			Schema:      "metrics",
			Description: view.Description,
			Fields:      fields,
			Meta: []dashboard.ModelMeta{
				{Label: "Base table", Value: view.BaseTable},
				{Label: "Time", Value: view.Time.DefaultField},
				{Label: "Dimensions", Value: strconv.Itoa(len(view.Dimensions))},
				{Label: "Measures", Value: strconv.Itoa(len(view.Measures))},
			},
		})
		graph.Edges = append(graph.Edges, dashboard.ModelEdge{
			ID:     "metrics_view_" + name + "_from_" + view.BaseTable,
			Source: nodeID("model_table", view.BaseTable),
			Target: nodeID("metrics_view", name),
			Label:  "metrics view",
			Kind:   "metrics",
		})
	}

	return graph
}

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func nodeID(kind, name string) string {
	return kind + ":" + name
}

func modelEndpoint(path string) (string, string) {
	parts := strings.Split(path, ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	if len(parts) < 2 {
		return path, ""
	}
	return parts[len(parts)-2], parts[len(parts)-1]
}

func modelTableFields(table semantic.ModelTable) []dashboard.ModelField {
	fields := []dashboard.ModelField{}
	for _, name := range sortedKeys(table.Dimensions) {
		role := "dimension"
		if name == table.PrimaryKey {
			role = "key"
		}
		fields = append(fields, dashboard.ModelField{Name: name, Role: role})
	}
	for _, name := range sortedKeys(table.Measures) {
		fields = append(fields, dashboard.ModelField{Name: name, Role: "measure"})
	}
	return fields
}

func refreshLabel(runtime *modelRuntime) string {
	if runtime.lastRefresh.IsZero() {
		return time.Now().Format("15:04:05")
	}
	return runtime.lastRefresh.Format("15:04:05")
}

func measureCount(modelID string, metricViews map[string]*semantic.MetricView) int {
	count := 0
	for _, view := range metricViews {
		if view.SemanticModel == modelID {
			count += len(view.Measures)
		}
	}
	return count
}

func formatMetric(value float64, format string) string {
	switch format {
	case "currency":
		return formatCurrency(value)
	case "integer":
		return formatInt(int64(math.Round(value)))
	case "decimal":
		return fmt.Sprintf("%.2f", value)
	default:
		return fmt.Sprintf("%.2f", value)
	}
}

func formatCurrency(value float64) string {
	if value >= 1000000 {
		return fmt.Sprintf("R$ %.1fm", value/1000000)
	}
	if value >= 1000 {
		return fmt.Sprintf("R$ %.1fk", value/1000)
	}
	return fmt.Sprintf("R$ %.0f", value)
}

func formatInt(value int64) string {
	if value >= 1000000 {
		return fmt.Sprintf("%.1fm", float64(value)/1000000)
	}
	if value >= 1000 {
		return fmt.Sprintf("%.1fk", float64(value)/1000)
	}
	return fmt.Sprintf("%d", value)
}

func round(value float64) float64 {
	return math.Round(value*100) / 100
}
