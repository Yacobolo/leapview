package query

import (
	"fmt"
	"strings"
)

func (p *Planner) Plan(request Request) (Plan, error) {
	view, err := p.metricView(request.MetricView)
	if err != nil {
		return Plan{}, err
	}

	fieldSet := []string{}
	for _, dimension := range request.Dimensions {
		field, _, err := view.ResolveDimensionRef(dimension.Field)
		if err != nil {
			return Plan{}, err
		}
		fieldSet = append(fieldSet, field)
	}
	if request.Time.Field != "" {
		field, _, err := view.ResolveDimensionRef(request.Time.Field)
		if err != nil {
			return Plan{}, err
		}
		request.Time.Field = field
		fieldSet = append(fieldSet, field)
	}
	for _, measure := range request.Measures {
		field, resolved, err := view.ResolveMeasureRef(measure.Field)
		if err != nil {
			return Plan{}, err
		}
		if resolved.Table != view.BaseTable {
			return Plan{}, fmt.Errorf("measure %q is not owned by base table %q", field, view.BaseTable)
		}
		fieldSet = append(fieldSet, field)
	}
	for _, filter := range request.Filters {
		field, _, err := view.ResolveDimensionRef(filter.Field)
		if err != nil {
			return Plan{}, err
		}
		fieldSet = append(fieldSet, field)
	}

	aliases, err := p.aliases(view, fieldSet)
	if err != nil {
		return Plan{}, err
	}
	from, err := joinSQL(view.BaseTable, aliases)
	if err != nil {
		return Plan{}, err
	}

	selects := []string{}
	groupBy := []string{}
	columns := []string{}
	if request.Time.Field != "" {
		dimension := view.Dimensions[request.Time.Field]
		alias := request.Time.Alias
		if alias == "" {
			alias = fieldAlias(request.Time.Field)
		}
		expr := dimensionExpr(dimension, aliases)
		if request.Time.Grain != "" {
			if !allowedTimeGrain(request.Time.Grain) {
				return Plan{}, fmt.Errorf("unsupported time grain %q", request.Time.Grain)
			}
			expr = fmt.Sprintf("date_trunc('%s', %s)", request.Time.Grain, expr)
		}
		selects = append(selects, expr+" AS "+alias)
		groupBy = append(groupBy, alias)
		columns = append(columns, alias)
	}
	for _, item := range request.Dimensions {
		field, _, _ := view.ResolveDimensionRef(item.Field)
		dimension := view.Dimensions[field]
		alias := item.Alias
		if alias == "" {
			alias = fieldAlias(field)
		}
		selects = append(selects, dimensionExpr(dimension, aliases)+" AS "+alias)
		groupBy = append(groupBy, alias)
		columns = append(columns, alias)
	}
	for _, item := range request.Measures {
		field, _, _ := view.ResolveMeasureRef(item.Field)
		measure := view.Measures[field]
		alias := item.Alias
		if alias == "" {
			alias = fieldAlias(field)
		}
		selects = append(selects, measureExpr(measure, aliases)+" AS "+alias)
		columns = append(columns, alias)
	}
	if len(selects) == 0 {
		return Plan{}, fmt.Errorf("query requires at least one selected field")
	}

	whereParts := []string{"1 = 1"}
	args := []any{}
	for _, filter := range request.Filters {
		field, _, _ := view.ResolveDimensionRef(filter.Field)
		expr := dimensionExpr(view.Dimensions[field], aliases)
		part, partArgs, err := filterSQL(expr, filter)
		if err != nil {
			return Plan{}, err
		}
		if part != "" {
			whereParts = append(whereParts, part)
			args = append(args, partArgs...)
		}
	}

	var sql strings.Builder
	sql.WriteString("SELECT ")
	sql.WriteString(strings.Join(selects, ", "))
	sql.WriteString("\nFROM ")
	sql.WriteString(from)
	sql.WriteString("\nWHERE ")
	sql.WriteString(strings.Join(whereParts, " AND "))
	if len(groupBy) > 0 {
		sql.WriteString("\nGROUP BY ")
		sql.WriteString(strings.Join(groupBy, ", "))
	}
	if len(request.Sort) > 0 {
		parts := make([]string, 0, len(request.Sort))
		for _, sort := range request.Sort {
			direction := "ASC"
			if strings.EqualFold(sort.Direction, "desc") {
				direction = "DESC"
			}
			parts = append(parts, fieldAlias(sort.Field)+" "+direction)
		}
		sql.WriteString("\nORDER BY ")
		sql.WriteString(strings.Join(parts, ", "))
	}
	if request.Limit > 0 {
		sql.WriteString(fmt.Sprintf("\nLIMIT %d", request.Limit))
	}
	return Plan{SQL: sql.String(), Args: args, Columns: columns}, nil
}

func allowedTimeGrain(grain string) bool {
	switch grain {
	case "day", "week", "month", "quarter", "year":
		return true
	default:
		return false
	}
}

func fieldAlias(field string) string {
	if field == "value" || field == "" {
		return field
	}
	parts := strings.Split(field, ".")
	return parts[len(parts)-1]
}
