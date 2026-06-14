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
	"github.com/Yacobolo/libredash/internal/semantic"
	_ "github.com/marcboeker/go-duckdb/v2"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type MissingDataError struct {
	DataDir string
	Missing []string
}

func (e *MissingDataError) Error() string {
	return fmt.Sprintf("Olist CSVs are missing in %s: %s. Run scripts/bootstrap_olist.py or set LIBREDASH_DATA_DIR.", e.DataDir, strings.Join(e.Missing, ", "))
}

type DuckDBMetrics struct {
	mu          sync.RWMutex
	db          *sql.DB
	dataDir     string
	dbPath      string
	model       *semantic.Model
	modelPath   string
	ready       bool
	missing     error
	lastRefresh time.Time
}

func NewDuckDBMetrics(dataDir string) (*DuckDBMetrics, error) {
	modelPath := os.Getenv("LIBREDASH_MODEL_PATH")
	if modelPath == "" {
		var err error
		modelPath, err = discoverModelPath()
		if err != nil {
			return nil, err
		}
	}

	model, err := semantic.Load(modelPath)
	if err != nil {
		return nil, fmt.Errorf("loading semantic model: %w", err)
	}

	metrics := &DuckDBMetrics{
		dataDir:   dataDir,
		dbPath:    duckDBPath(dataDir),
		model:     model,
		modelPath: modelPath,
	}
	if err := metrics.validateFiles(); err != nil {
		metrics.missing = err
		return metrics, nil
	}

	if err := os.MkdirAll(filepath.Dir(metrics.dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("duckdb", metrics.dbPath)
	if err != nil {
		return nil, err
	}
	metrics.db = db

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if err := metrics.RefreshCache(context.Background()); err != nil {
		db.Close()
		return nil, err
	}

	metrics.ready = true
	return metrics, nil
}

func (m *DuckDBMetrics) Close() error {
	if m.db == nil {
		return nil
	}
	return m.db.Close()
}

func (m *DuckDBMetrics) DataDir() string {
	return m.dataDir
}

func (m *DuckDBMetrics) Pages() []dashboard.Page {
	pages := make([]dashboard.Page, len(m.model.Pages))
	for i, page := range m.model.Pages {
		pages[i] = page.WithDefaults()
	}
	return pages
}

func (m *DuckDBMetrics) ModelGraph() dashboard.ModelGraph {
	return modelGraph(m.model)
}

func (m *DuckDBMetrics) QueryDashboard(ctx context.Context, filters dashboard.Filters) (dashboard.Patch, error) {
	filters = filters.WithDefaults()
	if !m.ready {
		return dashboard.EmptyPatch(filters, m.dataDir, m.missing), nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	patch := dashboard.Patch{
		Filters: filters,
		Status: dashboard.Status{
			Loading:       false,
			LastUpdated:   m.refreshLabel(),
			DataDirectory: m.dataDir,
		},
		Charts: map[string]dashboard.Chart{},
	}

	kpis, err := m.kpis(ctx, filters)
	if err != nil {
		return dashboard.EmptyPatch(filters, m.dataDir, err), nil
	}
	patch.KPIs = kpis

	charts, err := m.charts(ctx, filters)
	if err != nil {
		return dashboard.EmptyPatch(filters, m.dataDir, err), nil
	}
	patch.Charts = charts

	return patch, nil
}

func (m *DuckDBMetrics) QueryTable(ctx context.Context, filters dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error) {
	filters = filters.WithDefaults()
	request = request.WithDefaults()
	if !m.ready {
		return dashboard.EmptyTable(request, m.missing), nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	tableModel, ok := m.model.Tables[request.Table]
	if !ok {
		return dashboard.EmptyTable(request, fmt.Errorf("unknown table %q", request.Table)), nil
	}

	totalRows, err := m.countRows(ctx, tableModel.Dataset, filters, "table", request.Table)
	if err != nil {
		return dashboard.EmptyTable(request, err), nil
	}
	rows, err := m.tableRows(ctx, tableModel, filters, request)
	if err != nil {
		return dashboard.EmptyTable(request, err), nil
	}

	return dashboard.Table{
		Title:     tableModel.Title,
		Columns:   tableModel.Columns,
		Rows:      rows,
		TotalRows: totalRows,
		Window:    dashboard.TableWindow{Offset: request.Offset, Limit: request.Limit},
		Sort:      request.Sort,
		Loading:   false,
		Error:     "",
	}, nil
}

func (m *DuckDBMetrics) RefreshCache(ctx context.Context) error {
	if m.missing != nil {
		return m.missing
	}
	if m.db == nil {
		return fmt.Errorf("DuckDB is not initialized")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.registerSourceViews(ctx); err != nil {
		return err
	}
	if err := m.materializeCache(ctx); err != nil {
		return err
	}
	m.lastRefresh = time.Now()
	return nil
}

func (m *DuckDBMetrics) validateFiles() error {
	var missing []string
	for _, file := range m.model.SourceFiles() {
		if _, err := os.Stat(filepath.Join(m.dataDir, file)); errors.Is(err, os.ErrNotExist) {
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

func (m *DuckDBMetrics) registerSourceViews(ctx context.Context) error {
	if _, err := m.db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS raw"); err != nil {
		return err
	}
	if _, err := m.db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS cache"); err != nil {
		return err
	}

	for name, source := range m.model.Sources {
		if err := validateIdentifier(name); err != nil {
			return err
		}
		path := filepath.Join(m.dataDir, source.File)
		stmt := fmt.Sprintf("CREATE OR REPLACE VIEW raw.%s AS SELECT * FROM read_csv_auto('%s', header=true)", name, sqlString(path))
		if _, err := m.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("registering source %s: %w", name, err)
		}
	}
	return nil
}

func (m *DuckDBMetrics) materializeCache(ctx context.Context) error {
	for _, name := range m.model.CacheTableNames() {
		if err := validateIdentifier(name); err != nil {
			return err
		}
		table := m.model.Cache.Tables[name]
		stmt := fmt.Sprintf("CREATE OR REPLACE TABLE cache.%s AS %s", name, table.SQL)
		if _, err := m.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("materializing cache.%s: %w", name, err)
		}
	}
	return nil
}

func (m *DuckDBMetrics) kpis(ctx context.Context, filters dashboard.Filters) ([]dashboard.KPI, error) {
	keys := []string{"total_orders", "revenue", "aov", "review"}
	seen := map[string]struct{}{}
	for _, key := range keys {
		seen[key] = struct{}{}
	}
	for _, key := range sortedKeys(m.model.KPIs) {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
		}
	}
	kpis := make([]dashboard.KPI, 0, len(keys))
	for _, key := range keys {
		kpi, ok := m.model.KPIs[key]
		if !ok {
			continue
		}
		value, err := m.kpiValue(ctx, kpi, filters)
		if err != nil {
			return nil, err
		}
		measure := m.model.Datasets[kpi.Dataset].Measures[kpi.Measure]
		kpis = append(kpis, dashboard.KPI{
			Label: kpi.Title,
			Value: formatMetric(value, measure.Format),
			Note:  kpi.Note,
			Tone:  kpi.Tone,
		})
	}
	return kpis, nil
}

func (m *DuckDBMetrics) kpiValue(ctx context.Context, kpi semantic.KPI, filters dashboard.Filters) (float64, error) {
	source, err := m.datasetSource(kpi.Dataset)
	if err != nil {
		return 0, err
	}
	dataset := m.model.Datasets[kpi.Dataset]
	measure := dataset.Measures[kpi.Measure]
	expr, err := measureAggregateExpr(measure)
	if err != nil {
		return 0, err
	}
	where, args := m.filterWhere("e", kpi.Dataset, filters, "kpi", kpi.Measure)
	query := fmt.Sprintf("SELECT COALESCE(%s, 0) FROM %s e WHERE %s", expr, source, where)

	var value float64
	if err := m.db.QueryRowContext(ctx, query, args...).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

func (m *DuckDBMetrics) charts(ctx context.Context, filters dashboard.Filters) (map[string]dashboard.Chart, error) {
	charts := make(map[string]dashboard.Chart, len(m.model.Visuals))
	keys := make([]string, 0, len(m.model.Visuals))
	for key := range m.model.Visuals {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		visual := m.model.Visuals[key]
		points, err := m.visualPoints(ctx, key, visual, filters)
		if err != nil {
			return nil, err
		}
		dataset := m.model.Datasets[visual.Dataset]
		measureName := visual.Query.Measures[0]
		measure := dataset.Measures[measureName]
		series := []string{}
		if visual.Query.Series != "" {
			series = append(series, visual.Query.Series)
		}
		charts[key] = dashboard.Chart{
			Version:    2,
			ID:         key,
			Type:       visual.Type,
			Title:      visual.Title,
			Unit:       measure.Unit,
			Field:      visual.Interaction.Field,
			Dimensions: append([]string{}, visual.Query.Dimensions...),
			Measure:    measureName,
			Series:     series,
			Stacked:    visual.Stacked,
			Selection:  selectedValues(filters, key),
			Data:       points,
		}
	}
	return charts, nil
}

func (m *DuckDBMetrics) visualPoints(ctx context.Context, visualID string, visual semantic.Visual, filters dashboard.Filters) ([]dashboard.Point, error) {
	source, err := m.datasetSource(visual.Dataset)
	if err != nil {
		return nil, err
	}
	dataset := m.model.Datasets[visual.Dataset]
	labelDimension := visual.Query.Dimensions[0]
	labelExpr := dimensionExpression(dataset.Dimensions[labelDimension], "e")
	measureName := visual.Query.Measures[0]
	valueExpr, err := measureAggregateExpr(dataset.Measures[measureName])
	if err != nil {
		return nil, err
	}
	seriesExpr := "''"
	groupBy := []string{"label"}
	if visual.Query.Series != "" {
		seriesExpr = dimensionExpression(dataset.Dimensions[visual.Query.Series], "e")
		groupBy = append(groupBy, "series")
	}

	where, args := m.filterWhere("e", visual.Dataset, filters, "visual", visualID)
	for _, dimensionName := range append(append([]string{}, visual.Query.Dimensions...), visual.Query.Series) {
		if dimensionName == "" {
			continue
		}
		if dimension := dataset.Dimensions[dimensionName]; dimension.Where != "" {
			where = fmt.Sprintf("(%s) AND (%s)", where, dimensionWhere(dimension, "e"))
		}
	}

	orderBy := m.visualOrderBy(visual)
	query := fmt.Sprintf(`
SELECT %s AS label, %s AS series, %s AS value
FROM %s e
WHERE %s
GROUP BY %s
ORDER BY %s`, labelExpr, seriesExpr, valueExpr, source, where, strings.Join(groupBy, ", "), orderBy)
	if visual.Query.Limit > 0 {
		query += fmt.Sprintf("\nLIMIT %d", visual.Query.Limit)
	}

	points, err := m.queryPoints(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	markSelected(points, selectedValues(filters, visualID))
	return points, nil
}

func (m *DuckDBMetrics) queryPoints(ctx context.Context, query string, args ...any) ([]dashboard.Point, error) {
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []dashboard.Point{}
	for rows.Next() {
		var label string
		var series string
		var value float64
		if err := rows.Scan(&label, &series, &value); err != nil {
			return nil, err
		}
		points = append(points, dashboard.Point{Label: label, Series: series, Value: round(value)})
	}
	return points, rows.Err()
}

func (m *DuckDBMetrics) countRows(ctx context.Context, datasetName string, filters dashboard.Filters, targetKind, targetID string) (int, error) {
	source, err := m.datasetSource(datasetName)
	if err != nil {
		return 0, err
	}
	where, args := m.filterWhere("e", datasetName, filters, targetKind, targetID)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s e WHERE %s", source, where)

	var total int
	if err := m.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (m *DuckDBMetrics) tableRows(ctx context.Context, table semantic.TableVisual, filters dashboard.Filters, request dashboard.TableRequest) ([]map[string]any, error) {
	source, err := m.datasetSource(table.Dataset)
	if err != nil {
		return nil, err
	}
	where, args := m.filterWhere("e", table.Dataset, filters, "table", request.Table)
	sortExpr := tableSortExpr(table, request.Sort.Key)
	direction := "DESC"
	if request.Sort.Direction == "asc" {
		direction = "ASC"
	}

	selects := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		if err := validateIdentifier(column.Key); err != nil {
			return nil, err
		}
		selects = append(selects, "e."+column.Key)
	}

	query := fmt.Sprintf(`
SELECT %s
FROM %s e
WHERE %s
ORDER BY %s %s, e.order_id ASC
LIMIT ? OFFSET ?`, strings.Join(selects, ", "), source, where, sortExpr, direction)

	args = append(args, request.Limit, request.Offset)
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]any, len(table.Columns))
	scans := make([]any, len(table.Columns))
	for i := range values {
		scans[i] = &values[i]
	}

	result := []map[string]any{}
	for rows.Next() {
		if err := rows.Scan(scans...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, column := range table.Columns {
			row[column.Key] = normalizeDBValue(values[i])
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func measureAggregateExpr(measure semantic.Measure) (string, error) {
	switch measure.Aggregate {
	case "count":
		return "COUNT(*)", nil
	case "count_distinct":
		if err := validateIdentifier(measure.Column); err != nil {
			return "", err
		}
		return "COUNT(DISTINCT e." + measure.Column + ")", nil
	case "sum":
		if err := validateIdentifier(measure.Column); err != nil {
			return "", err
		}
		return "SUM(e." + measure.Column + ")", nil
	case "avg":
		if err := validateIdentifier(measure.Column); err != nil {
			return "", err
		}
		return "AVG(e." + measure.Column + ")", nil
	case "expression":
		if measure.Expression == "" {
			return "", fmt.Errorf("measure %q is missing expression", measure.Label)
		}
		return measure.Expression, nil
	default:
		return "", fmt.Errorf("unsupported measure aggregate %q", measure.Aggregate)
	}
}

func tableSortExpr(table semantic.TableVisual, key string) string {
	if key == "" {
		key = table.DefaultSort.Key
	}
	for _, column := range table.Columns {
		if column.Key == key {
			return "e." + column.Key
		}
	}
	if table.DefaultSort.Key != "" {
		return "e." + table.DefaultSort.Key
	}
	return "e.order_id"
}

func (m *DuckDBMetrics) visualOrderBy(visual semantic.Visual) string {
	if len(visual.Query.Sort) == 0 {
		return "label ASC"
	}
	dataset := m.model.Datasets[visual.Dataset]
	parts := make([]string, 0, len(visual.Query.Sort))
	for _, sortSpec := range visual.Query.Sort {
		direction := "ASC"
		if strings.EqualFold(sortSpec.Direction, "desc") {
			direction = "DESC"
		}
		expr := sortSpec.Expr
		if expr == "" {
			expr = m.sortExpression(dataset, visual, sortSpec.Field)
		}
		if expr == "" {
			expr = "label"
		}
		parts = append(parts, expr+" "+direction)
	}
	return strings.Join(parts, ", ")
}

func (m *DuckDBMetrics) sortExpression(dataset semantic.Dataset, visual semantic.Visual, field string) string {
	if field == "" {
		return "label"
	}
	if field == "value" || field == visual.Query.Measures[0] {
		return "value"
	}
	if field == visual.Query.Series {
		return "series"
	}
	if dimension, ok := dataset.Dimensions[field]; ok {
		if dimension.OrderExpr != "" {
			return dimension.OrderExpr
		}
		if field == visual.Query.Dimensions[0] {
			return "label"
		}
		return dimensionExpression(dimension, "e")
	}
	return ""
}

func (m *DuckDBMetrics) filterWhere(alias, datasetName string, filters dashboard.Filters, targetKind, targetID string) (string, []any) {
	filters = filters.WithDefaults()
	conditions := []string{"1 = 1"}
	args := []any{}

	if filters.State != "" && filters.State != "all" {
		conditions = append(conditions, alias+".state = ?")
		args = append(args, strings.ToUpper(filters.State))
	}

	switch filters.DateRange {
	case "2017":
		conditions = append(conditions, alias+".purchase_timestamp >= TIMESTAMP '2017-01-01' AND "+alias+".purchase_timestamp < TIMESTAMP '2018-01-01'")
	case "2018":
		conditions = append(conditions, alias+".purchase_timestamp >= TIMESTAMP '2018-01-01' AND "+alias+".purchase_timestamp < TIMESTAMP '2019-01-01'")
	case "recent":
		conditions = append(conditions, alias+".purchase_timestamp >= (SELECT max(purchase_timestamp) - INTERVAL 90 DAY FROM cache.orders_enriched)")
	}

	if filters.Category != "" && filters.Category != "all" {
		conditions = append(conditions, "lower("+alias+".category) LIKE lower(?)")
		args = append(args, "%"+filters.Category+"%")
	}

	for _, selection := range filters.VisualSelections {
		if selection.VisualID == "" || len(selection.Values) == 0 {
			continue
		}
		if targetKind == "visual" && selection.VisualID == targetID {
			continue
		}
		sourceVisual, ok := m.model.Visuals[selection.VisualID]
		if !ok || !targetsSelection(sourceVisual.Interaction.Targets, targetKind, targetID) {
			continue
		}
		if selection.Operator != "" && selection.Operator != "in" {
			continue
		}
		dataset, ok := m.model.Datasets[datasetName]
		if !ok {
			continue
		}
		dimension, ok := dataset.Dimensions[selection.Field]
		if !ok {
			continue
		}
		placeholders := make([]string, 0, len(selection.Values))
		for _, value := range selection.Values {
			placeholders = append(placeholders, "?")
			args = append(args, value)
		}
		conditions = append(conditions, dimensionExpression(dimension, alias)+" IN ("+strings.Join(placeholders, ", ")+")")
	}

	return strings.Join(conditions, " AND "), args
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

func dimensionExpression(dimension semantic.Dimension, alias string) string {
	if identifierPattern.MatchString(dimension.Expr) {
		return alias + "." + dimension.Expr
	}
	return strings.ReplaceAll(dimension.Expr, "{alias}", alias)
}

func dimensionWhere(dimension semantic.Dimension, alias string) string {
	if dimension.Where == "" {
		return ""
	}
	return strings.ReplaceAll(dimension.Where, "{alias}", alias)
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

func markSelected(points []dashboard.Point, values []string) {
	if len(values) == 0 {
		return
	}
	selected := make(map[string]struct{}, len(values))
	for _, value := range values {
		selected[value] = struct{}{}
	}
	for i := range points {
		if _, ok := selected[points[i].Label]; ok {
			points[i].Selected = true
		}
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

func (m *DuckDBMetrics) datasetSource(name string) (string, error) {
	dataset, ok := m.model.Datasets[name]
	if !ok {
		return "", fmt.Errorf("unknown dataset %q", name)
	}
	return cacheSource(dataset.Source)
}

func cacheSource(name string) (string, error) {
	if err := validateIdentifier(name); err != nil {
		return "", err
	}
	return "cache." + name, nil
}

func validateIdentifier(value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("invalid identifier %q", value)
	}
	return nil
}

func discoverModelPath() (string, error) {
	candidates := []string{
		filepath.Join("dashboards", "olist.yaml"),
		filepath.Join("..", "dashboards", "olist.yaml"),
		filepath.Join("..", "..", "dashboards", "olist.yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find dashboards/olist.yaml")
}

func duckDBPath(dataDir string) string {
	if path := os.Getenv("LIBREDASH_DUCKDB_PATH"); path != "" {
		return path
	}
	return filepath.Join(dataDir, "libredash.duckdb")
}

func sqlString(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(path), "'", "''")
}

func modelGraph(model *semantic.Model) dashboard.ModelGraph {
	graph := dashboard.ModelGraph{
		Name:  model.Name,
		Title: model.Title,
		Stats: dashboard.ModelStats{
			Sources:       len(model.Sources),
			CacheTables:   len(model.Cache.Tables),
			Metrics:       len(model.KPIs),
			Visuals:       len(model.Visuals),
			ReportTables:  len(model.Tables),
			Relationships: len(model.Relationships),
		},
	}

	for _, name := range sortedKeys(model.Sources) {
		source := model.Sources[name]
		graph.Nodes = append(graph.Nodes, dashboard.ModelNode{
			ID:     nodeID("source", name),
			Label:  name,
			Kind:   "source",
			Schema: "raw",
			Fields: []dashboard.ModelField{{Name: source.File, Role: "csv"}},
			Meta: []dashboard.ModelMeta{
				{Label: "File", Value: source.File},
				{Label: "Schema", Value: "raw"},
			},
		})
	}

	for _, name := range sortedKeys(model.Cache.Tables) {
		table := model.Cache.Tables[name]
		graph.Nodes = append(graph.Nodes, dashboard.ModelNode{
			ID:          nodeID("cache", name),
			Label:       name,
			Kind:        "cache",
			Schema:      "cache",
			Description: table.Description,
			Fields:      cacheFields(),
			Meta: []dashboard.ModelMeta{
				{Label: "Mode", Value: "DuckDB import"},
				{Label: "Schema", Value: "cache"},
			},
		})
		for _, sourceName := range sortedKeys(model.Sources) {
			graph.Edges = append(graph.Edges, dashboard.ModelEdge{
				ID:     "source_" + sourceName + "_to_cache_" + name,
				Source: nodeID("source", sourceName),
				Target: nodeID("cache", name),
				Label:  "materializes",
				Kind:   "materialization",
			})
		}
	}

	for _, relationship := range model.Relationships {
		fromTable, fromField := modelEndpoint(relationship.From)
		toTable, toField := modelEndpoint(relationship.To)
		graph.Edges = append(graph.Edges, dashboard.ModelEdge{
			ID:          relationship.ID,
			Source:      nodeID("source", fromTable),
			Target:      nodeID("source", toTable),
			Label:       fromField + " -> " + toField,
			Kind:        "relationship",
			SourceField: fromField,
			TargetField: toField,
			Cardinality: relationship.Cardinality,
		})
	}

	for _, name := range sortedKeys(model.Datasets) {
		dataset := model.Datasets[name]
		fields := make([]dashboard.ModelField, 0, len(dataset.Dimensions)+len(dataset.Measures))
		for _, dimension := range sortedKeys(dataset.Dimensions) {
			fields = append(fields, dashboard.ModelField{Name: dimension, Role: "dimension"})
		}
		for _, measure := range sortedKeys(dataset.Measures) {
			fields = append(fields, dashboard.ModelField{Name: measure, Role: "measure"})
		}
		graph.Nodes = append(graph.Nodes, dashboard.ModelNode{
			ID:     nodeID("dataset", name),
			Label:  name,
			Kind:   "dataset",
			Schema: "semantic",
			Fields: fields,
			Meta: []dashboard.ModelMeta{
				{Label: "Source", Value: dataset.Source},
				{Label: "Dimensions", Value: strconv.Itoa(len(dataset.Dimensions))},
				{Label: "Measures", Value: strconv.Itoa(len(dataset.Measures))},
			},
		})
		graph.Edges = append(graph.Edges, dashboard.ModelEdge{
			ID:     "dataset_" + name + "_from_" + dataset.Source,
			Source: nodeID("cache", dataset.Source),
			Target: nodeID("dataset", name),
			Label:  "semantic dataset",
			Kind:   "semantic",
		})
	}

	for _, name := range sortedKeys(model.KPIs) {
		kpi := model.KPIs[name]
		measure := model.Datasets[kpi.Dataset].Measures[kpi.Measure]
		graph.Nodes = append(graph.Nodes, dashboard.ModelNode{
			ID:          nodeID("metric", name),
			Label:       kpi.Title,
			Kind:        "metric",
			Description: kpi.Note,
			Fields: []dashboard.ModelField{
				{Name: kpi.Dataset, Role: "dataset"},
				{Name: measureAggregateLabel(measure.Aggregate, measure.Column), Role: "measure"},
			},
			Meta: []dashboard.ModelMeta{
				{Label: "Format", Value: measure.Format},
				{Label: "Aggregate", Value: measure.Aggregate},
			},
		})
		graph.Edges = append(graph.Edges, semanticEdge("metric", name, kpi.Dataset, "measure"))
	}

	for _, name := range sortedKeys(model.Visuals) {
		visual := model.Visuals[name]
		measureName := visual.Query.Measures[0]
		measure := model.Datasets[visual.Dataset].Measures[measureName]
		graph.Nodes = append(graph.Nodes, dashboard.ModelNode{
			ID:    nodeID("visual", name),
			Label: visual.Title,
			Kind:  "visual",
			Fields: []dashboard.ModelField{
				{Name: visual.Query.Dimensions[0], Role: "axis"},
				{Name: measureAggregateLabel(measure.Aggregate, measure.Column), Role: "value"},
			},
			Meta: []dashboard.ModelMeta{
				{Label: "Type", Value: visual.Type},
				{Label: "Unit", Value: measure.Unit},
				{Label: "Limit", Value: intLabel(visual.Query.Limit)},
			},
		})
		graph.Edges = append(graph.Edges, semanticEdge("visual", name, visual.Dataset, "visual"))
	}

	for _, name := range sortedKeys(model.Tables) {
		table := model.Tables[name]
		fields := make([]dashboard.ModelField, 0, len(table.Columns))
		for _, column := range table.Columns {
			role := "column"
			if column.Align == "right" {
				role = "measure"
			}
			fields = append(fields, dashboard.ModelField{Name: column.Key, Role: role})
		}
		graph.Nodes = append(graph.Nodes, dashboard.ModelNode{
			ID:     nodeID("table", name),
			Label:  table.Title,
			Kind:   "report_table",
			Fields: fields,
			Meta: []dashboard.ModelMeta{
				{Label: "Default sort", Value: table.DefaultSort.Key + " " + table.DefaultSort.Direction},
				{Label: "Columns", Value: strconv.Itoa(len(table.Columns))},
			},
		})
		graph.Edges = append(graph.Edges, semanticEdge("table", name, table.Dataset, "table"))
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

func semanticEdge(kind, name, source, label string) dashboard.ModelEdge {
	return dashboard.ModelEdge{
		ID:     kind + "_" + name + "_from_" + source,
		Source: nodeID("dataset", source),
		Target: nodeID(kind, name),
		Label:  label,
		Kind:   "semantic",
	}
}

func modelEndpoint(path string) (string, string) {
	parts := strings.Split(path, ".")
	if len(parts) < 3 {
		return path, ""
	}
	return parts[len(parts)-2], parts[len(parts)-1]
}

func cacheFields() []dashboard.ModelField {
	return []dashboard.ModelField{
		{Name: "order_id", Role: "key"},
		{Name: "purchase_month", Role: "time"},
		{Name: "status", Role: "dimension"},
		{Name: "state", Role: "dimension"},
		{Name: "category", Role: "dimension"},
		{Name: "revenue", Role: "measure"},
		{Name: "review_score", Role: "measure"},
		{Name: "delivery_days", Role: "measure"},
	}
}

func measureAggregateLabel(aggregate, column string) string {
	if column == "" {
		return aggregate
	}
	return aggregate + "(" + column + ")"
}

func intLabel(value int) string {
	if value == 0 {
		return "all"
	}
	return strconv.Itoa(value)
}

func (m *DuckDBMetrics) refreshLabel() string {
	if m.lastRefresh.IsZero() {
		return time.Now().Format("15:04:05")
	}
	return m.lastRefresh.Format("15:04:05")
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
