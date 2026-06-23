package query

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Yacobolo/libredash/internal/semantic"
)

type Planner struct {
	Model *semantic.Model
}

type tableAlias struct {
	Table string
	Alias string
	Path  []semantic.Relationship
}

func NewPlanner(model *semantic.Model) *Planner {
	return &Planner{Model: model}
}

func (p *Planner) queryView(request Request) (*semantic.QueryScope, error) {
	return p.semanticView(request.Table, request.Dimensions, request.Measures, request.Filters, request.Time.Field)
}

func (p *Planner) rowView(request RowRequest) (*semantic.QueryScope, error) {
	if request.Table == "" && len(request.Measures) == 0 {
		return nil, fmt.Errorf("row query requires table when no measure is selected")
	}
	return p.semanticView(request.Table, request.Dimensions, request.Measures, request.Filters, "")
}

func (p *Planner) rawValueView(request RawValueRequest) (*semantic.QueryScope, error) {
	measures := []Field{}
	if request.Measure.Field != "" {
		measures = append(measures, request.Measure)
	}
	return p.semanticView(request.Table, request.Dimensions, measures, request.Filters, "")
}

func (p *Planner) countView(request CountRequest) (*semantic.QueryScope, error) {
	if request.Table == "" {
		return nil, fmt.Errorf("count query requires table")
	}
	return p.semanticView(request.Table, nil, nil, request.Filters, "")
}

func (p *Planner) semanticView(table string, dimensions []Field, measures []Field, filters []Filter, timeField string) (*semantic.QueryScope, error) {
	if p.Model == nil {
		return nil, fmt.Errorf("semantic model is required")
	}
	baseTable := table
	grain := ""
	resolvedMeasures := map[string]semantic.MetricMeasure{}
	for _, item := range measures {
		measure := item.Measure
		if strings.TrimSpace(measure.SQLExpression()) == "" {
			var err error
			measure, err = p.Model.ResolveMeasure(item.Field)
			if err != nil {
				return nil, err
			}
		} else {
			measure.Field = defaultString(measure.Field, item.Field)
			measure.Name = defaultString(measure.Name, item.Field)
		}
		if measure.Table == "" {
			return nil, fmt.Errorf("measure %q has no base table", item.Field)
		}
		if baseTable == "" {
			baseTable = measure.Table
			grain = measure.Grain
		}
		if measure.Table != baseTable || (grain != "" && measure.Grain != "" && measure.Grain != grain) {
			return nil, fmt.Errorf("cross-fact measures are not supported")
		}
		if grain == "" {
			grain = measure.Grain
		}
		resolvedMeasures[item.Field] = measure
	}
	if baseTable == "" {
		return nil, fmt.Errorf("query requires a base table")
	}
	if _, ok := p.Model.Tables[baseTable]; !ok {
		return nil, fmt.Errorf("unknown table %q", baseTable)
	}
	resolvedDimensions := map[string]semantic.MetricDimension{}
	for _, item := range dimensions {
		dimension, err := p.Model.ResolveDimension(item.Field)
		if err != nil {
			return nil, err
		}
		if _, err := p.relationshipPath(baseTable, dimension.Table); err != nil {
			return nil, err
		}
		resolvedDimensions[item.Field] = dimension
	}
	for _, filter := range filters {
		dimension, err := p.Model.ResolveDimension(filter.Field)
		if err != nil {
			return nil, err
		}
		if _, err := p.relationshipPath(baseTable, dimension.Table); err != nil {
			return nil, err
		}
		resolvedDimensions[filter.Field] = dimension
	}
	if timeField != "" {
		dimension, err := p.Model.ResolveDimension(timeField)
		if err != nil {
			return nil, err
		}
		if _, err := p.relationshipPath(baseTable, dimension.Table); err != nil {
			return nil, err
		}
		resolvedDimensions[timeField] = dimension
	}
	return &semantic.QueryScope{
		BaseTable:  baseTable,
		Grain:      grain,
		Dimensions: resolvedDimensions,
		Measures:   resolvedMeasures,
	}, nil
}

func (p *Planner) aliases(view *semantic.QueryScope, fields []string) (map[string]tableAlias, error) {
	aliases := map[string]tableAlias{
		view.BaseTable: {Table: view.BaseTable, Alias: "t0"},
	}
	nextAlias := 1
	for _, field := range fields {
		table, _, err := splitField(field)
		if err != nil {
			return nil, err
		}
		if _, ok := aliases[table]; ok {
			continue
		}
		path, err := p.relationshipPath(view.BaseTable, table)
		if err != nil {
			return nil, err
		}
		for _, step := range pathTables(view.BaseTable, path) {
			if _, ok := aliases[step.Table]; ok {
				continue
			}
			aliases[step.Table] = tableAlias{Table: step.Table, Alias: fmt.Sprintf("t%d", nextAlias), Path: step.Path}
			nextAlias++
		}
	}
	return aliases, nil
}

func (p *Planner) relationshipPath(base, target string) ([]semantic.Relationship, error) {
	if base == target {
		return nil, nil
	}

	frontier := []pathCandidate{{Table: base, Visited: map[string]bool{base: true}}}
	for len(frontier) > 0 {
		next := []pathCandidate{}
		matches := [][]semantic.Relationship{}
		for _, candidate := range frontier {
			for _, edge := range p.safeEdgesFrom(candidate.Table) {
				if candidate.Visited[edge.Table] {
					continue
				}
				path := append(append([]semantic.Relationship{}, candidate.Path...), edge.Relationship)
				if edge.Table == target {
					matches = append(matches, path)
					continue
				}
				visited := copyVisited(candidate.Visited)
				visited[edge.Table] = true
				next = append(next, pathCandidate{Table: edge.Table, Path: path, Visited: visited})
			}
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("ambiguous relationship path from %q to %q", base, target)
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
		frontier = next
	}
	return nil, fmt.Errorf("no safe relationship path from %q to %q", base, target)
}

func (p *Planner) safeEdgesFrom(table string) []relationshipEdge {
	edges := []relationshipEdge{}
	for _, relationship := range p.Model.Relationships {
		edge, ok := safeEdgeFrom(table, relationship)
		if ok {
			edges = append(edges, edge)
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Table != edges[j].Table {
			return edges[i].Table < edges[j].Table
		}
		if edges[i].Relationship.ID != edges[j].Relationship.ID {
			return edges[i].Relationship.ID < edges[j].Relationship.ID
		}
		if edges[i].Relationship.From != edges[j].Relationship.From {
			return edges[i].Relationship.From < edges[j].Relationship.From
		}
		return edges[i].Relationship.To < edges[j].Relationship.To
	})
	return edges
}

func safeEdgeFrom(table string, relationship semantic.Relationship) (relationshipEdge, bool) {
	if !relationship.Active {
		return relationshipEdge{}, false
	}
	fromTable, _, err := splitField(relationship.From)
	if err != nil {
		return relationshipEdge{}, false
	}
	toTable, _, err := splitField(relationship.To)
	if err != nil {
		return relationshipEdge{}, false
	}
	if fromTable == table && safeCardinality(relationship.Cardinality) {
		return relationshipEdge{Table: toTable, Relationship: relationship}, true
	}
	if relationship.Cardinality == "one_to_one" && toTable == table {
		return relationshipEdge{Table: fromTable, Relationship: relationship}, true
	}
	return relationshipEdge{}, false
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func pathTables(base string, path []semantic.Relationship) []tablePath {
	current := base
	tables := []tablePath{}
	for index, relationship := range path {
		fromTable, _, err := splitField(relationship.From)
		if err != nil {
			return tables
		}
		toTable, _, err := splitField(relationship.To)
		if err != nil {
			return tables
		}
		next := ""
		switch {
		case current == fromTable:
			next = toTable
		case relationship.Cardinality == "one_to_one" && current == toTable:
			next = fromTable
		default:
			return tables
		}
		tables = append(tables, tablePath{Table: next, Path: append([]semantic.Relationship{}, path[:index+1]...)})
		current = next
	}
	return tables
}

func copyVisited(values map[string]bool) map[string]bool {
	next := make(map[string]bool, len(values)+1)
	for key, value := range values {
		next[key] = value
	}
	return next
}

type pathCandidate struct {
	Table   string
	Path    []semantic.Relationship
	Visited map[string]bool
}

type relationshipEdge struct {
	Table        string
	Relationship semantic.Relationship
}

type tablePath struct {
	Table string
	Path  []semantic.Relationship
}

func safeCardinality(cardinality string) bool {
	return cardinality == "many_to_one" || cardinality == "one_to_one"
}

func splitField(field string) (string, string, error) {
	parts := strings.Split(field, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("field %q must be qualified as table.field", field)
	}
	return parts[0], parts[1], nil
}
