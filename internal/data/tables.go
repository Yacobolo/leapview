package data

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Yacobolo/libredash/internal/dashboard"
	"github.com/Yacobolo/libredash/internal/semantic"
)

func (m *DuckDBMetrics) tableBlocks(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, table semantic.TableVisual, filters dashboard.Filters, request dashboard.TableRequest, availableRows int) (map[string]dashboard.TableBlock, error) {
	blocks := map[string]dashboard.TableBlock{}
	count := request.Count
	if count <= 0 {
		count = dashboard.TableChunkSize
	}
	if count > dashboard.TableMaxRequestCount {
		count = dashboard.TableMaxRequestCount
	}
	if request.Block == "all" {
		starts := initialBlockStarts(request.Start, count, availableRows)
		for block, start := range starts {
			rows, err := m.tableRows(ctx, runtime, report, table, filters, request, start, count, availableRows)
			if err != nil {
				return nil, err
			}
			blocks[block] = dashboard.TableBlock{
				Start:        start,
				RequestSeq:   request.RequestSeq,
				ResetVersion: request.ResetVersion,
				Sort:         request.Sort,
				Rows:         rows,
			}
		}
		return blocks, nil
	}

	start := clampTableStart(request.Start, availableRows)
	rows, err := m.tableRows(ctx, runtime, report, table, filters, request, start, count, availableRows)
	if err != nil {
		return nil, err
	}
	blocks[request.Block] = dashboard.TableBlock{
		Start:        start,
		RequestSeq:   request.RequestSeq,
		ResetVersion: request.ResetVersion,
		Sort:         request.Sort,
		Rows:         rows,
	}
	return blocks, nil
}

func (m *DuckDBMetrics) queryAggregateTable(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, request dashboard.TableRequest, table semantic.TableVisual, filters dashboard.Filters) (dashboard.Table, error) {
	var (
		columns []dashboard.TableColumn
		rows    []map[string]any
		err     error
	)
	switch table.KindOrDefault() {
	case "matrix_table":
		columns, rows, err = m.matrixTableRows(ctx, runtime, report, table, filters, request)
	case "pivot_table":
		columns, rows, err = m.pivotTableRows(ctx, runtime, report, table, filters, request)
	default:
		err = fmt.Errorf("unsupported aggregate table kind %q", table.KindOrDefault())
	}
	if err != nil {
		return dashboard.EmptyTable(request, err), nil
	}
	totalRows := len(rows)
	isCapped := totalRows > dashboard.TableInteractiveRowCap
	if isCapped {
		rows = rows[:dashboard.TableInteractiveRowCap]
	}
	chunkSize := max(dashboard.TableChunkSize, len(rows))
	return dashboard.Table{
		Version:       2,
		Kind:          table.KindOrDefault(),
		Title:         table.Title,
		Columns:       columns,
		TotalRows:     totalRows,
		AvailableRows: len(rows),
		IsCapped:      isCapped,
		RowCap:        dashboard.TableInteractiveRowCap,
		ChunkSize:     chunkSize,
		RowHeight:     dashboard.TableRowHeight,
		ResetVersion:  request.ResetVersion,
		Sort:          request.Sort,
		Blocks: map[string]dashboard.TableBlock{
			"a": {Start: 0, RequestSeq: request.RequestSeq, ResetVersion: request.ResetVersion, Sort: request.Sort, Rows: rows},
			"b": {Start: chunkSize, RequestSeq: request.RequestSeq, ResetVersion: request.ResetVersion, Sort: request.Sort, Rows: []map[string]any{}},
			"c": {Start: chunkSize * 2, RequestSeq: request.RequestSeq, ResetVersion: request.ResetVersion, Sort: request.Sort, Rows: []map[string]any{}},
		},
		LoadingBlock: "",
		Error:        "",
	}, nil
}

func (m *DuckDBMetrics) matrixTableRows(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, table semantic.TableVisual, filters dashboard.Filters, request dashboard.TableRequest) ([]dashboard.TableColumn, []map[string]any, error) {
	if len(table.ColumnDims) == 1 {
		return m.crossTabTableRows(ctx, runtime, report, table, filters, request, false)
	}
	source, err := m.metricViewSource(table.MetricView)
	if err != nil {
		return nil, nil, err
	}
	metricView := m.workspace.MetricViews[table.MetricView]
	columns := make([]dashboard.TableColumn, 0, len(table.Rows)+len(table.Measures))
	selects := make([]string, 0, len(table.Rows)+len(table.Measures))
	groupBy := make([]string, 0, len(table.Rows))
	for _, dimensionName := range table.Rows {
		dimension := metricView.Dimensions[dimensionName]
		selects = append(selects, fmt.Sprintf("%s AS %s", dimensionExpression(dimension, "e"), dimensionName))
		groupBy = append(groupBy, dimensionName)
		columns = append(columns, dashboard.TableColumn{Key: dimensionName, Label: dimensionLabel(dimensionName, dimension), Role: "row_header"})
	}
	for _, measureName := range table.Measures {
		measure := metricView.Measures[measureName]
		expr, err := measureAggregateExpr(measure)
		if err != nil {
			return nil, nil, err
		}
		selects = append(selects, fmt.Sprintf("%s AS %s", expr, measureName))
		columns = append(columns, dashboard.TableColumn{Key: measureName, Label: measureLabel(measureName, measure), Align: "right", Role: "measure", Measure: measureName})
	}
	where, args := m.filterWhere("e", runtime, report, table.MetricView, filters, "table", request.Table)
	orderBy := strings.Join(groupBy, ", ")
	if request.Sort.Key != "" && tableHasColumn(columns, request.Sort.Key) {
		direction := "DESC"
		if request.Sort.Direction == "asc" {
			direction = "ASC"
		}
		orderBy = request.Sort.Key + " " + direction
	}
	query := fmt.Sprintf(`
SELECT %s
FROM %s e
WHERE %s
GROUP BY %s
ORDER BY %s
LIMIT ?`, strings.Join(selects, ", "), source, where, strings.Join(groupBy, ", "), orderBy)
	args = append(args, dashboard.TableInteractiveRowCap+1)
	rows, err := m.queryTableDatums(ctx, runtime, query, tableColumnKeys(columns), args...)
	return columns, rows, err
}

func (m *DuckDBMetrics) pivotTableRows(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, table semantic.TableVisual, filters dashboard.Filters, request dashboard.TableRequest) ([]dashboard.TableColumn, []map[string]any, error) {
	return m.crossTabTableRows(ctx, runtime, report, table, filters, request, true)
}

func (m *DuckDBMetrics) crossTabTableRows(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, table semantic.TableVisual, filters dashboard.Filters, request dashboard.TableRequest, pivotMode bool) ([]dashboard.TableColumn, []map[string]any, error) {
	source, err := m.metricViewSource(table.MetricView)
	if err != nil {
		return nil, nil, err
	}
	metricView := m.workspace.MetricViews[table.MetricView]
	rowSelects := make([]string, 0, len(table.Rows))
	groupBy := make([]string, 0, len(table.Rows)+1)
	baseColumns := make([]dashboard.TableColumn, 0, len(table.Rows))
	for _, dimensionName := range table.Rows {
		dimension := metricView.Dimensions[dimensionName]
		rowSelects = append(rowSelects, fmt.Sprintf("%s AS %s", dimensionExpression(dimension, "e"), dimensionName))
		groupBy = append(groupBy, dimensionName)
		baseColumns = append(baseColumns, dashboard.TableColumn{Key: dimensionName, Label: dimensionLabel(dimensionName, dimension), Role: "row_header"})
	}
	columnDimensionName := table.ColumnDims[0]
	columnDimension := metricView.Dimensions[columnDimensionName]
	valueSelects := make([]string, 0, len(table.Measures))
	valueColumns := make([]string, 0, len(table.Measures))
	for _, measureName := range table.Measures {
		measureExpr, err := measureAggregateExpr(metricView.Measures[measureName])
		if err != nil {
			return nil, nil, err
		}
		valueSelects = append(valueSelects, fmt.Sprintf("%s AS %s", measureExpr, measureName))
		valueColumns = append(valueColumns, measureName)
	}
	groupBy = append(groupBy, "pivot_label")
	where, args := m.filterWhere("e", runtime, report, table.MetricView, filters, "table", request.Table)
	query := fmt.Sprintf(`
SELECT %s, %s AS pivot_label, %s
FROM %s e
WHERE %s
GROUP BY %s
ORDER BY %s
LIMIT ?`, strings.Join(rowSelects, ", "), dimensionExpression(columnDimension, "e"), strings.Join(valueSelects, ", "), source, where, strings.Join(groupBy, ", "), strings.Join(groupBy, ", "))
	args = append(args, dashboard.TableInteractiveRowCap+1)
	rawRows, err := m.queryTableDatums(ctx, runtime, query, append(append(append([]string{}, table.Rows...), "pivot_label"), valueColumns...), args...)
	if err != nil {
		return nil, nil, err
	}
	columns := append([]dashboard.TableColumn{}, baseColumns...)
	pivotKeys := map[string]string{}
	usedKeys := map[string]string{}
	columnKeys := map[string]string{}
	for _, column := range baseColumns {
		usedKeys[column.Key] = column.Key
	}
	resultByKey := map[string]map[string]any{}
	order := []string{}
	for _, raw := range rawRows {
		rowKeyParts := make([]string, 0, len(table.Rows))
		for _, dimension := range table.Rows {
			rowKeyParts = append(rowKeyParts, fmt.Sprint(raw[dimension]))
		}
		resultKey := strings.Join(rowKeyParts, "\x00")
		row, exists := resultByKey[resultKey]
		if !exists {
			row = map[string]any{}
			for _, dimension := range table.Rows {
				row[dimension] = raw[dimension]
			}
			resultByKey[resultKey] = row
			order = append(order, resultKey)
		}
		label := fmt.Sprint(raw["pivot_label"])
		groupLabel := label
		if pivotMode {
			groupLabel = measureLabel(table.Measures[0], metricView.Measures[table.Measures[0]])
		}
		pivotKey, exists := pivotKeys[label]
		if !exists {
			pivotKey = sanitizeTableKey(label)
			pivotKeys[label] = pivotKey
		}
		for _, measureName := range table.Measures {
			measure := metricView.Measures[measureName]
			columnIdentity := label + "\x00" + measureName
			columnKey, columnExists := columnKeys[columnIdentity]
			candidate := "pivot_" + pivotKey
			columnLabel := label
			if !pivotMode || len(table.Measures) > 1 {
				candidate += "__" + sanitizeTableKey(measureName)
				columnLabel = measureLabel(measureName, measure)
			}
			if !columnExists {
				columnKey = uniqueTableColumnKey(candidate, usedKeys)
				columnKeys[columnIdentity] = columnKey
				usedKeys[columnKey] = columnKey
				columns = append(columns, dashboard.TableColumn{
					Key:         columnKey,
					Label:       columnLabel,
					Align:       "right",
					Role:        "measure",
					Group:       groupLabel,
					Measure:     measureName,
					ColumnValue: label,
				})
			}
			row[columnKey] = raw[measureName]
		}
	}
	result := make([]map[string]any, 0, len(order))
	for _, key := range order {
		result = append(result, resultByKey[key])
	}
	sortAggregateTableRows(result, request.Sort)
	return columns, result, nil
}

func (m *DuckDBMetrics) queryTableDatums(ctx context.Context, runtime *modelRuntime, query string, columns []string, args ...any) ([]map[string]any, error) {
	rows, err := runtime.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]any, len(columns))
	scans := make([]any, len(columns))
	for i := range values {
		scans[i] = &values[i]
	}
	result := []map[string]any{}
	for rows.Next() {
		if err := rows.Scan(scans...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, column := range columns {
			row[column] = normalizeDBValue(values[i])
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func tableColumnKeys(columns []dashboard.TableColumn) []string {
	keys := make([]string, len(columns))
	for i, column := range columns {
		keys[i] = column.Key
	}
	return keys
}

func tableHasColumn(columns []dashboard.TableColumn, key string) bool {
	for _, column := range columns {
		if column.Key == key {
			return true
		}
	}
	return false
}

func sortAggregateTableRows(rows []map[string]any, tableSort dashboard.TableSort) {
	if tableSort.Key == "" {
		return
	}
	direction := tableSort.Direction
	sort.SliceStable(rows, func(i, j int) bool {
		cmp := compareTableValues(rows[i][tableSort.Key], rows[j][tableSort.Key])
		if direction == "desc" {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareTableValues(a, b any) int {
	aFloat, aNumeric := numericTableValue(a)
	bFloat, bNumeric := numericTableValue(b)
	if aNumeric && bNumeric {
		switch {
		case aFloat < bFloat:
			return -1
		case aFloat > bFloat:
			return 1
		default:
			return 0
		}
	}
	aText := strings.ToLower(fmt.Sprint(a))
	bText := strings.ToLower(fmt.Sprint(b))
	switch {
	case aText < bText:
		return -1
	case aText > bText:
		return 1
	default:
		return 0
	}
}

func numericTableValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func dimensionLabel(name string, dimension semantic.MetricDimension) string {
	if strings.TrimSpace(dimension.Label) != "" {
		return dimension.Label
	}
	return name
}

func sanitizeTableKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	key := strings.Trim(builder.String(), "_")
	if key == "" {
		return "value"
	}
	return key
}

func uniqueTableColumnKey(candidate string, existing map[string]string) string {
	used := map[string]struct{}{}
	for _, key := range existing {
		used[key] = struct{}{}
	}
	key := candidate
	for i := 2; ; i++ {
		if _, ok := used[key]; !ok {
			return key
		}
		key = fmt.Sprintf("%s_%d", candidate, i)
	}
}

func initialBlockStarts(start, count, availableRows int) map[string]int {
	start = clampTableStart(start, availableRows)
	if start <= 0 {
		return map[string]int{"a": 0, "b": count, "c": count * 2}
	}
	base := (start / count) * count
	return map[string]int{"a": max(0, base-count), "b": base, "c": base + count}
}

func clampTableStart(start, availableRows int) int {
	if start < 0 {
		return 0
	}
	if availableRows <= 0 {
		return 0
	}
	if start >= availableRows {
		return max(0, availableRows-1)
	}
	return start
}

func (m *DuckDBMetrics) tableRows(ctx context.Context, runtime *modelRuntime, report *semantic.Dashboard, table semantic.TableVisual, filters dashboard.Filters, request dashboard.TableRequest, start, count, availableRows int) ([]map[string]any, error) {
	if count <= 0 || start >= availableRows {
		return []map[string]any{}, nil
	}
	if start+count > availableRows {
		count = availableRows - start
	}
	source, err := m.metricViewSource(table.MetricView)
	if err != nil {
		return nil, err
	}
	where, args := m.filterWhere("e", runtime, report, table.MetricView, filters, "table", request.Table)
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

	args = append(args, count, start)
	rows, err := runtime.db.QueryContext(ctx, query, args...)
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
