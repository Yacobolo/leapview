package runtime

import (
	"context"
	"fmt"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"strings"

	"github.com/Yacobolo/libredash/internal/dashboard"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
)

func (m *Service) visuals(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, filters dashboard.Filters, keys []string) (map[string]dashboard.Visual, error) {
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
		measureName := visual.Query.Measures[0].Field
		measure := semanticmodel.MetricMeasure{}
		if resolved, err := runtime.model.ResolveMeasure(measureName); err == nil {
			measure = resolved
		}
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

func (m *Service) visualData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
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

func (m *Service) categoryData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	dimensionAlias := "label"
	measureAlias := "value"
	dimensions := []reportdef.QueryField{fieldRef(visual.Query.Dimensions[0].Field, dimensionAlias)}
	columns := []string{dimensionAlias, measureAlias}
	if !visual.Query.Series.IsZero() {
		dimensions = append(dimensions, fieldRef(visual.Query.Series.Field, "series"))
		columns = []string{dimensionAlias, "series", measureAlias}
	}
	sorts := visualSorts(visual)
	if len(visual.Query.Sort) == 0 {
		sorts = []reportdef.QuerySort{{Field: dimensionAlias, Direction: "asc"}}
	}
	data, err := m.querySemanticDatums(ctx, runtime, reportdef.AggregateQuery{
		Dimensions: dimensions,
		Measures:   []reportdef.QueryField{queryFieldRef(visual.Query.Measures[0], measureAlias)},
		Filters:    queryFilters,
		Sort:       sorts,
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

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func fieldAlias(field string) string {
	parts := strings.Split(field, ".")
	return parts[len(parts)-1]
}

func (m *Service) categoryMultiMeasureData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	data := []dashboard.Datum{}

	for _, measureName := range visual.Query.Measures {
		rows, err := m.querySemanticDatums(ctx, runtime, reportdef.AggregateQuery{
			Dimensions: []reportdef.QueryField{fieldRef(visual.Query.Dimensions[0].Field, "label")},
			Measures:   []reportdef.QueryField{queryFieldRef(measureName, "value")},
			Filters:    queryFilters,
			Sort:       visualSorts(visual),
			Limit:      visual.Query.Limit,
		})
		if err != nil {
			return nil, err
		}
		measure, _ := runtime.model.ResolveMeasure(measureName.Field)
		for _, row := range rows {
			row["series"] = measureLabel(measureName.Field, measure)
		}
		data = append(data, rows...)
	}
	markSelected(data, "label", selectedValues(filters, visualID))
	return data, nil
}

func (m *Service) categoryDeltaData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
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

func (m *Service) binnedMeasureData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	bins, err := runtime.data.Histogram(ctx, reportdef.RawValueQuery{
		Measure: queryFieldRef(visual.Query.Measures[0], "value"),
		Filters: queryFilters,
	}, optionInt(visual.Options, "bin_count", 20, 5, 60))
	if err != nil {
		return nil, err
	}
	data := make([]dashboard.Datum, 0, len(bins))
	for _, bin := range bins {
		data = append(data, dashboard.Datum{
			"label":    formatBinLabel(bin.Start, bin.End),
			"binStart": round(bin.Start),
			"binEnd":   round(bin.End),
			"value":    bin.Count,
		})
	}
	return data, nil
}

func (m *Service) hierarchyData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	dimensions := make([]reportdef.QueryField, 0, len(visual.Query.Dimensions))
	levelAliases := make([]string, 0, len(visual.Query.Dimensions))
	for index, dimensionName := range visual.Query.Dimensions {
		alias := fmt.Sprintf("level_%d", index)
		dimensions = append(dimensions, fieldRef(dimensionName.Field, alias))
		levelAliases = append(levelAliases, alias)
	}
	rows, err := runtime.data.Query(ctx, reportdef.AggregateQuery{
		Dimensions: dimensions,
		Measures:   []reportdef.QueryField{queryFieldRef(visual.Query.Measures[0], "value")},
		Filters:    queryFilters,
		Sort:       visualSorts(visual),
		Limit:      visual.Query.Limit,
	})
	if err != nil {
		return nil, err
	}
	data := make([]dashboard.Datum, 0, len(rows))
	for _, row := range rows {
		path := make([]string, 0, len(levelAliases))
		for _, alias := range levelAliases {
			item := normalizeDatumValue(row[alias])
			if item == nil || fmt.Sprint(item) == "" {
				continue
			}
			path = append(path, fmt.Sprint(item))
		}
		data = append(data, dashboard.Datum{
			"path":  path,
			"value": normalizeDatumValue(row["value"]),
		})
	}
	return data, nil
}

func (m *Service) singleValueData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	measureRef := visual.Query.Measures[0]
	measureName := measureRef.Field
	title := visual.Title
	if title == "" {
		if measure, err := runtime.model.ResolveMeasure(measureName); err == nil {
			title = measure.Label
		} else if measureRef.Measure.Label != "" {
			title = measureRef.Measure.Label
		}
	}
	if title == "" {
		title = defaultString(measureName, measureRef.Alias)
	}
	queryFilters, err := m.semanticFilters(ctx, runtime, report, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	dimensions := []reportdef.QueryField{}
	if len(visual.Query.Dimensions) == 1 {
		dimensions = append(dimensions, fieldRef(visual.Query.Dimensions[0].Field, "label"))
	}
	sorts := visualSorts(visual)
	if len(dimensions) == 0 {
		sorts = nil
	}
	data, err := m.querySemanticDatums(ctx, runtime, reportdef.AggregateQuery{
		Dimensions: dimensions,
		Measures:   []reportdef.QueryField{queryFieldRef(measureRef, "value")},
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

func (m *Service) matrixData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	return m.dimensionPairData(ctx, runtime, report, visualID, visual, filters, "row", "column")
}

func (m *Service) graphData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	return m.dimensionPairData(ctx, runtime, report, visualID, visual, filters, "source", "target")
}

func (m *Service) dimensionPairData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters, leftAlias, rightAlias string) ([]dashboard.Datum, error) {
	rightSQLAlias := rightAlias
	if rightAlias == "column" {
		rightSQLAlias = "chart_column"
	}
	queryFilters, err := m.semanticFilters(ctx, runtime, report, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	data, err := m.querySemanticDatums(ctx, runtime, reportdef.AggregateQuery{
		Dimensions: []reportdef.QueryField{
			fieldRef(visual.Query.Dimensions[0].Field, leftAlias),
			fieldRef(visual.Query.Dimensions[1].Field, rightSQLAlias),
		},
		Measures: []reportdef.QueryField{queryFieldRef(visual.Query.Measures[0], "value")},
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

func (m *Service) geoData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	data, err := m.querySemanticDatums(ctx, runtime, reportdef.AggregateQuery{
		Dimensions: []reportdef.QueryField{fieldRef(visual.Query.Dimensions[0].Field, "name")},
		Measures:   []reportdef.QueryField{queryFieldRef(visual.Query.Measures[0], "value")},
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

func (m *Service) ohlcData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	return m.querySemanticDatums(ctx, runtime, reportdef.AggregateQuery{
		Dimensions: []reportdef.QueryField{fieldRef(visual.Query.Dimensions[0].Field, "label")},
		Measures: []reportdef.QueryField{
			queryFieldRef(visual.Query.Measures[0], "open"),
			queryFieldRef(visual.Query.Measures[1], "close"),
			queryFieldRef(visual.Query.Measures[2], "low"),
			queryFieldRef(visual.Query.Measures[3], "high"),
		},
		Filters: queryFilters,
		Sort:    visualSorts(visual),
		Limit:   visual.Query.Limit,
	})
}

func (m *Service) distributionData(ctx context.Context, runtime *modelRuntime, report *reportdef.Dashboard, visualID string, visual reportdef.Visual, filters dashboard.Filters) ([]dashboard.Datum, error) {
	queryFilters, err := m.semanticFilters(ctx, runtime, report, filters, "visual", visualID)
	if err != nil {
		return nil, err
	}
	return m.queryDistributionDatums(ctx, runtime, reportdef.RawValueQuery{
		Dimensions: []reportdef.QueryField{fieldRef(visual.Query.Dimensions[0].Field, "label")},
		Measure:    queryFieldRef(visual.Query.Measures[0], "value"),
		Filters:    queryFilters,
	}, distributionSorts(visual), visual.Query.Limit)
}

func visualQueryDimensions(visual reportdef.Visual) []string {
	dimensions := queryDimensionFields(visual.Query.Dimensions)
	if !visual.Query.Series.IsZero() {
		dimensions = append(dimensions, visual.Query.Series.Field)
	}
	return dimensions
}

func (m *Service) querySemanticDatums(ctx context.Context, runtime *modelRuntime, request reportdef.AggregateQuery) ([]dashboard.Datum, error) {
	rows, err := runtime.data.Query(ctx, request)
	if err != nil {
		return nil, err
	}
	return datumsFromAnalytics(rows), nil
}

func (m *Service) queryDistributionDatums(ctx context.Context, runtime *modelRuntime, request reportdef.RawValueQuery, sort []reportdef.QuerySort, limit int) ([]dashboard.Datum, error) {
	rows, err := runtime.data.Distribution(ctx, request, sort, limit)
	if err != nil {
		return nil, err
	}
	return datumsFromAnalytics(rows), nil
}

func datumsFromAnalytics(rows reportdef.QueryRows) []dashboard.Datum {
	data := make([]dashboard.Datum, 0, len(rows))
	for _, row := range rows {
		datum := dashboard.Datum{}
		for column, value := range row {
			datum[column] = normalizeDatumValue(value)
		}
		data = append(data, datum)
	}
	return data
}
