package query

import (
	"fmt"
	"strings"

	"github.com/Yacobolo/libredash/internal/semantic"
)

type Planner struct {
	Model *semantic.Model
	Views map[string]*semantic.MetricView
}

type tableAlias struct {
	Table string
	Alias string
	Path  []semantic.Relationship
}

func NewPlanner(model *semantic.Model, views map[string]*semantic.MetricView) *Planner {
	return &Planner{Model: model, Views: views}
}

func (p *Planner) metricView(id string) (*semantic.MetricView, error) {
	view, ok := p.Views[id]
	if !ok {
		return nil, fmt.Errorf("unknown metric view %q", id)
	}
	if view.SemanticModel != p.Model.Name {
		return nil, fmt.Errorf("metric view %q belongs to model %q, want %q", id, view.SemanticModel, p.Model.Name)
	}
	return view, nil
}

func (p *Planner) aliases(view *semantic.MetricView, fields []string) (map[string]tableAlias, error) {
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
		aliases[table] = tableAlias{Table: table, Alias: fmt.Sprintf("t%d", nextAlias), Path: path}
		nextAlias++
	}
	return aliases, nil
}

func (p *Planner) relationshipPath(base, target string) ([]semantic.Relationship, error) {
	if base == target {
		return nil, nil
	}
	var matches [][]semantic.Relationship
	for _, relationship := range p.Model.Relationships {
		if !relationship.Active {
			continue
		}
		fromTable, _, err := splitField(relationship.From)
		if err != nil {
			return nil, err
		}
		toTable, _, err := splitField(relationship.To)
		if err != nil {
			return nil, err
		}
		if fromTable == base && toTable == target && safeCardinality(relationship.Cardinality) {
			matches = append(matches, []semantic.Relationship{relationship})
		}
		if relationship.Cardinality == "one_to_one" && toTable == base && fromTable == target {
			matches = append(matches, []semantic.Relationship{relationship})
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no safe relationship path from %q to %q", base, target)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous relationship path from %q to %q", base, target)
	}
	return matches[0], nil
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
