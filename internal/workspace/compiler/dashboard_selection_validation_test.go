package compiler

import (
	"fmt"
	"strings"
	"testing"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/dashboard"
	"github.com/Yacobolo/libredash/internal/dashboard/report"
)

func TestValidateDashboardUsesSemanticSelectionResolver(t *testing.T) {
	tests := []struct {
		name    string
		mapping report.SelectionMapping
		wantErr string
	}{
		{name: "conformed", mapping: report.SelectionMapping{Field: "release_decade", Value: "label"}},
		{name: "physical requires fact", mapping: report.SelectionMapping{Field: "ratings.release_decade", Value: "label"}, wantErr: `physical field "ratings.release_decade" requires fact`},
		{name: "semantic forbids fact", mapping: report.SelectionMapping{Field: "release_decade", Fact: "ratings", Value: "label"}, wantErr: `semantic dimension "release_decade" must not specify fact`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dashboardDefinition, model := compilerSelectionFixture(test.mapping)
			err := ValidateDashboard(dashboardDefinition, map[string]*semanticmodel.Model{"model": model})
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateDashboard() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateDashboard() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateDashboardRejectsPointSelectionForGraphRenderers(t *testing.T) {
	for _, visualType := range []string{"graph", "sankey"} {
		t.Run(visualType, func(t *testing.T) {
			dashboardDefinition, model := compilerSelectionFixture(report.SelectionMapping{Field: "release_decade", Value: "source"})
			source := dashboardDefinition.Visuals["source"]
			source.Type = visualType
			source.Shape = "graph"
			source.Query.Dimensions = []report.FieldRef{
				{Field: "release_decade", Alias: "source"},
				{Field: "release_decade", Alias: "target"},
			}
			dashboardDefinition.Visuals["source"] = source

			err := ValidateDashboard(dashboardDefinition, map[string]*semanticmodel.Model{"model": model})
			want := fmt.Sprintf(`visual "source" type %q shape "graph" does not support point_selection`, visualType)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("ValidateDashboard() error = %v, want containing %q", err, want)
			}
		})
	}
}

func TestValidateDashboardResolvesNumericSpatialSelectionCoordinates(t *testing.T) {
	dashboardDefinition, model := compilerSpatialSelectionFixture()
	if err := ValidateDashboard(dashboardDefinition, map[string]*semanticmodel.Model{"model": model}); err != nil {
		t.Fatalf("ValidateDashboard() error = %v", err)
	}

	source := dashboardDefinition.Visuals["source"]
	source.Interaction.SpatialSelection.Latitude.Field = "ratings.release_decade"
	dashboardDefinition.Visuals["source"] = source
	if err := ValidateDashboard(dashboardDefinition, map[string]*semanticmodel.Model{"model": model}); err == nil || !strings.Contains(err.Error(), `field "ratings.release_decade" must be numeric`) {
		t.Fatalf("nonnumeric spatial coordinate error = %v", err)
	}
}

func compilerSpatialSelectionFixture() (*report.Dashboard, *semanticmodel.Model) {
	model := &semanticmodel.Model{
		Name: "model",
		Tables: map[string]semanticmodel.Table{"ratings": {Dimensions: map[string]semanticmodel.MetricDimension{
			"latitude": {Type: "number"}, "longitude": {Type: "number"}, "release_decade": {Type: "string"},
		}}},
		Measures: map[string]semanticmodel.MetricMeasure{"rating_count": {Fact: "ratings", Aggregation: "count", Empty: "zero"}},
	}
	source := report.Visual{
		Title: "Source", Type: "map",
		Query: report.VisualQuery{Table: "ratings", Dimensions: []report.FieldRef{
			{Field: "ratings.latitude", Alias: "latitude"}, {Field: "ratings.longitude", Alias: "longitude"},
		}, Measures: []report.FieldRef{{Field: "rating_count", Alias: "value"}}, Limit: 100},
		Geo: report.VisualGeo{Layers: []report.VisualGeoLayer{{ID: "density", Kind: "density", Latitude: "latitude", Longitude: "longitude", Value: "value"}}},
		Interaction: report.Interaction{SpatialSelection: report.SpatialSelectionInteraction{
			Gestures:  []string{"box", "lasso", "radius"},
			Latitude:  report.SpatialSelectionMapping{Source: "latitude", Field: "ratings.latitude", Fact: "ratings"},
			Longitude: report.SpatialSelectionMapping{Source: "longitude", Field: "ratings.longitude", Fact: "ratings"},
			Targets:   []string{"target"},
		}},
	}
	target := report.Visual{Title: "Target", Type: "kpi", Query: report.VisualQuery{Table: "ratings", Measures: []report.FieldRef{{Field: "rating_count", Alias: "value"}}, Limit: 1}}
	return &report.Dashboard{
		ID: "dashboard", Title: "Dashboard", SemanticModel: "model",
		Visuals: map[string]report.Visual{"source": source, "target": target},
		Pages:   []dashboard.Page{{ID: "overview", Title: "Overview"}},
	}, model
}

func compilerSelectionFixture(mapping report.SelectionMapping) (*report.Dashboard, *semanticmodel.Model) {
	model := &semanticmodel.Model{
		Name: "model",
		Tables: map[string]semanticmodel.Table{
			"ratings": {Dimensions: map[string]semanticmodel.MetricDimension{"release_decade": {Type: "string"}}},
			"tags":    {Dimensions: map[string]semanticmodel.MetricDimension{"release_decade": {Type: "string"}}},
		},
		Dimensions: map[string]semanticmodel.SemanticDimension{
			"release_decade": {Type: "string", Bindings: map[string]semanticmodel.DimensionBinding{
				"ratings": {Field: "ratings.release_decade"},
				"tags":    {Field: "tags.release_decade"},
			}},
		},
		Measures: map[string]semanticmodel.MetricMeasure{
			"rating_count": {Fact: "ratings", Aggregation: "count", Empty: "zero"},
			"tag_count":    {Fact: "tags", Aggregation: "count", Empty: "zero"},
		},
	}
	source := report.Visual{
		Title: "Source", Type: "bar",
		Query: report.VisualQuery{
			Dimensions: []report.FieldRef{{Field: mapping.Field, Alias: "label"}},
			Measures:   []report.FieldRef{{Field: "rating_count", Alias: "value"}},
		},
		Interaction: report.Interaction{PointSelection: report.SelectionInteraction{
			Mappings: []report.SelectionMapping{mapping}, Targets: []string{"target"},
		}},
	}
	target := report.Visual{
		Title: "Target", Type: "combo",
		Query: report.VisualQuery{
			Dimensions: []report.FieldRef{{Field: "release_decade", Alias: "label"}},
			Measures: []report.FieldRef{
				{Field: "rating_count", Alias: "rating_count"},
				{Field: "tag_count", Alias: "tag_count"},
			},
		},
	}
	return &report.Dashboard{
		ID: "dashboard", Title: "Dashboard", SemanticModel: "model",
		Visuals: map[string]report.Visual{"source": source, "target": target},
		Pages:   []dashboard.Page{{ID: "overview", Title: "Overview"}},
	}, model
}
