package compiler

import (
	"strings"
	"testing"

	reportdef "github.com/flidai/leapview/internal/dashboard/report"
	visualizationir "github.com/flidai/leapview/internal/dashboard/visualization/ir"
)

func TestCompiledCartesianDecisionContextUsesClosedIR(t *testing.T) {
	t.Parallel()

	minimum, maximum, target := 0.0, 100.0, 80.0
	authored := reportdef.Visual{
		Type: "line",
		Query: reportdef.VisualQuery{
			Dimensions: []reportdef.FieldRef{{Field: "orders.month"}},
			Measures:   []reportdef.FieldRef{{Field: "revenue"}},
		},
		Presentation: reportdef.VisualPresentation{
			Axes: []reportdef.VisualAxis{
				{ID: "x", Title: "Month", TickDensity: "sparse"},
				{ID: "primary_y", Title: "Revenue", Scale: "linear", Zero: "include", Minimum: &minimum, Maximum: &maximum, Unit: "USD"},
			},
			ReferenceLines: []reportdef.VisualReferenceLine{{
				ID: "target", Axis: "primary_y", Value: reportdef.VisualReferenceValue{Number: &target}, Label: "Target", Tone: "success",
			}},
			ReferenceBands: []reportdef.VisualReferenceBand{{
				ID: "healthy", Axis: "primary_y",
				From:  reportdef.VisualReferenceValue{Field: "value", Reducer: "minimum"},
				To:    reportdef.VisualReferenceValue{Field: "value", Reducer: "maximum"},
				Label: "Observed range",
			}},
			EventAnnotations: []reportdef.VisualEventAnnotation{{
				ID: "launch", Axis: "x", Value: reportdef.VisualReferenceValue{Text: "2026-03-01"}, Label: "Launch",
			}},
			Tooltip: []string{"label", "value"},
		},
	}

	spec, err := compileBuiltInVisualizationSpec("revenue", authored, nil)
	if err != nil {
		t.Fatalf("compileBuiltInVisualizationSpec() error = %v", err)
	}
	chart := spec.Value.(*visualizationir.CartesianVisualizationSpec)
	if chart.Axes == nil || len(*chart.Axes) != 2 {
		t.Fatalf("axes = %#v, want two axes", chart.Axes)
	}
	if got := (*chart.Axes)[1]; got.ID != visualizationir.VisualizationCartesianAxisPrimaryY || got.Scale != visualizationir.VisualizationAxisScaleLinear || got.Zero != visualizationir.VisualizationAxisZeroPolicyInclude || got.Minimum == nil || *got.Minimum != 0 || got.Maximum == nil || *got.Maximum != 100 {
		t.Fatalf("primary axis = %#v", got)
	}
	if chart.ReferenceLines == nil || len(*chart.ReferenceLines) != 1 {
		t.Fatalf("reference lines = %#v", chart.ReferenceLines)
	}
	lineValue, ok := (*chart.ReferenceLines)[0].Value.Value.(*visualizationir.NumberVisualizationReferenceValue)
	if !ok || lineValue.Value != 80 {
		t.Fatalf("reference line value = %#v, want number 80", (*chart.ReferenceLines)[0].Value)
	}
	bandFrom, ok := (*chart.ReferenceBands)[0].From.Value.(*visualizationir.FieldVisualizationReferenceValue)
	if !ok || bandFrom.Field != (visualizationir.VisualizationFieldRef{Dataset: "primary", Field: "value"}) || bandFrom.Reducer != visualizationir.VisualizationReferenceReducerMinimum {
		t.Fatalf("reference band from = %#v", (*chart.ReferenceBands)[0].From)
	}
	if chart.Tooltip == nil || len(*chart.Tooltip) != 2 || (*chart.Tooltip)[1].Field != "value" {
		t.Fatalf("tooltip = %#v", chart.Tooltip)
	}
}

func TestCompiledCartesianDecisionContextRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	authored := reportdef.Visual{
		Type:  "line",
		Query: reportdef.VisualQuery{Measures: []reportdef.FieldRef{{Field: "revenue"}}},
		Presentation: reportdef.VisualPresentation{ReferenceLines: []reportdef.VisualReferenceLine{{
			ID: "target", Axis: "primary_y", Value: reportdef.VisualReferenceValue{Field: "deleted_target"},
		}}},
	}

	_, err := compileBuiltInVisualizationSpec("revenue", authored, nil)
	if err == nil || !strings.Contains(err.Error(), `reference field "deleted_target" is not in the compiled result`) {
		t.Fatalf("compileBuiltInVisualizationSpec() error = %v", err)
	}
}

func TestCompiledCartesianSeriesIntentIsDeterministic(t *testing.T) {
	t.Parallel()

	authored := reportdef.Visual{
		Type: "area",
		Query: reportdef.VisualQuery{
			Dimensions: []reportdef.FieldRef{{Field: "orders.month"}},
			Series:     reportdef.FieldRef{Field: "orders.status", Alias: "status"},
			Measures:   []reportdef.FieldRef{{Field: "revenue"}},
		},
		Presentation: reportdef.VisualPresentation{
			Stacking:     "percent",
			SeriesOrder:  []string{"delivered", "processing"},
			SeriesColors: map[string]string{"processing": "data_3", "canceled": "danger"},
		},
	}

	spec, err := compileBuiltInVisualizationSpec("revenue", authored, nil)
	if err != nil {
		t.Fatalf("compileBuiltInVisualizationSpec() error = %v", err)
	}
	presentation := spec.Value.(*visualizationir.CartesianVisualizationSpec).Presentation
	if presentation.Stacking == nil || *presentation.Stacking != visualizationir.VisualizationStackingModePercent {
		t.Fatalf("stacking = %v, want percent", presentation.Stacking)
	}
	if presentation.SeriesIntent == nil || len(*presentation.SeriesIntent) != 3 {
		t.Fatalf("series intent = %#v, want three entries", presentation.SeriesIntent)
	}
	first, second, third := (*presentation.SeriesIntent)[0], (*presentation.SeriesIntent)[1], (*presentation.SeriesIntent)[2]
	if first.Value != "delivered" || first.Order == nil || *first.Order != 0 || first.Color != nil {
		t.Fatalf("first series intent = %#v", first)
	}
	if second.Value != "processing" || second.Order == nil || *second.Order != 1 || second.Color == nil || *second.Color != visualizationir.VisualizationColorIntentData3 {
		t.Fatalf("second series intent = %#v", second)
	}
	if third.Value != "canceled" || third.Order != nil || third.Color == nil || *third.Color != visualizationir.VisualizationColorIntentDanger {
		t.Fatalf("third series intent = %#v", third)
	}
}
