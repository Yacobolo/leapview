package compiler

import (
	"testing"

	reportdef "github.com/flidai/leapview/internal/dashboard/report"
	visualizationir "github.com/flidai/leapview/internal/dashboard/visualization/ir"
)

func TestCompileKPIOwnsComparisonGoalAndTrendBindings(t *testing.T) {
	t.Parallel()

	minimum, maximum := 0.0, 100.0
	authored := reportdef.Visual{
		Type:  "kpi",
		Title: "Revenue",
		Query: reportdef.VisualQuery{Measures: []reportdef.FieldRef{{Field: "revenue"}}},
		Datasets: map[string]reportdef.VisualQuery{
			"comparison": {Measures: []reportdef.FieldRef{{Field: "prior_revenue", Alias: "value"}}, Limit: 1},
			"goal":       {Measures: []reportdef.FieldRef{{Field: "target_revenue", Alias: "value"}}, Limit: 1},
			"trend": {
				Time:     reportdef.QueryTime{Field: "orders.created_at", Grain: "month", Alias: "period"},
				Measures: []reportdef.FieldRef{{Field: "revenue", Alias: "value"}},
				Sort:     []reportdef.Sort{{Field: "period", Direction: "asc"}},
				Limit:    12,
			},
		},
		KPI: reportdef.VisualKPI{
			Mode:               "bullet",
			Comparison:         &reportdef.VisualKPIValueBinding{Dataset: "comparison", Field: "value", Reducer: "last", Label: "Previous"},
			Goal:               &reportdef.VisualKPIValueBinding{Dataset: "goal", Field: "value", Label: "Target"},
			Trend:              &reportdef.VisualKPITrendBinding{Dataset: "trend", Category: "period", Value: "value"},
			Delta:              "relative",
			FavorableDirection: "increase",
			MissingComparison:  "hide",
			Ranges: []reportdef.VisualKPIQualitativeRange{{
				Minimum: &minimum, Maximum: &maximum, Label: "On track", Tone: "success",
			}},
		},
	}

	specification, err := compileBuiltInVisualizationSpec("revenue", authored, nil)
	if err != nil {
		t.Fatalf("compileBuiltInVisualizationSpec(): %v", err)
	}
	kpi := specification.Value.(*visualizationir.KPIVisualizationSpec)
	if kpi.Comparison == nil || kpi.Comparison.Field.Dataset != "comparison" || kpi.Comparison.Reducer != visualizationir.VisualizationReferenceReducerLast {
		t.Fatalf("comparison = %#v", kpi.Comparison)
	}
	if kpi.Goal == nil || kpi.Goal.Field.Dataset != "goal" || kpi.Goal.Reducer != visualizationir.VisualizationReferenceReducerFirst {
		t.Fatalf("goal = %#v", kpi.Goal)
	}
	if kpi.Trend == nil || kpi.Trend.Category.Dataset != "trend" || kpi.Trend.Category.Field != "period" || kpi.Trend.Value.Field != "value" {
		t.Fatalf("trend = %#v", kpi.Trend)
	}
	if kpi.Presentation.Mode != visualizationir.VisualizationKPIModeBullet ||
		kpi.Presentation.Delta != visualizationir.VisualizationKPIDeltaModeRelative ||
		kpi.Presentation.FavorableDirection != visualizationir.VisualizationKPIDirectionIncrease ||
		kpi.Presentation.MissingComparison != visualizationir.VisualizationKPIMissingComparisonHide {
		t.Fatalf("presentation = %#v", kpi.Presentation)
	}
	if len(kpi.Presentation.Ranges) != 1 || kpi.Presentation.Ranges[0].Label != "On track" {
		t.Fatalf("ranges = %#v", kpi.Presentation.Ranges)
	}
	if kpi.DataBudget.MaxRows != 12 {
		t.Fatalf("data budget = %d, want 12 to admit the trend dataset", kpi.DataBudget.MaxRows)
	}
}

func TestCompileKPIDefaultsRemainExplicit(t *testing.T) {
	t.Parallel()

	authored := reportdef.Visual{
		Type:  "kpi",
		Query: reportdef.VisualQuery{Measures: []reportdef.FieldRef{{Field: "revenue"}}},
	}
	specification, err := compileBuiltInVisualizationSpec("revenue", authored, nil)
	if err != nil {
		t.Fatal(err)
	}
	presentation := specification.Value.(*visualizationir.KPIVisualizationSpec).Presentation
	if presentation.Mode != visualizationir.VisualizationKPIModeCompact ||
		presentation.Delta != visualizationir.VisualizationKPIDeltaModeAbsolute ||
		presentation.FavorableDirection != visualizationir.VisualizationKPIDirectionNeutral ||
		presentation.MissingComparison != visualizationir.VisualizationKPIMissingComparisonShowUnavailable ||
		presentation.DisplayUnits == nil || *presentation.DisplayUnits != visualizationir.VisualizationDisplayUnitsAuto {
		t.Fatalf("defaults = %#v", presentation)
	}
}

func TestCompileKPIUsesAuthoredDescriptionForAccessibleContext(t *testing.T) {
	t.Parallel()

	authored := reportdef.Visual{
		Type:        "kpi",
		Title:       "Revenue",
		Description: "Revenue against the governed baseline.",
		Query:       reportdef.VisualQuery{Measures: []reportdef.FieldRef{{Field: "revenue"}}},
	}
	specification, err := compileBuiltInVisualizationSpec("revenue", authored, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := specification.Value.(*visualizationir.KPIVisualizationSpec).Accessibility.Description; got != authored.Description {
		t.Fatalf("accessibility description = %q, want %q", got, authored.Description)
	}
}
