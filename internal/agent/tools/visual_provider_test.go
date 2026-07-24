package tools

import (
	"context"
	"reflect"
	"strings"
	"testing"

	semanticmodel "github.com/Yacobolo/leapview/internal/analytics/model"
	reportdef "github.com/Yacobolo/leapview/internal/dashboard/report"
)

func TestAgentVisualShapeUsesVisualTypeDefaults(t *testing.T) {
	tests := map[string]string{
		"histogram":   "binned_measure",
		"candlestick": "ohlc",
		"boxplot":     "distribution",
		"heatmap":     "matrix",
		"sankey":      "graph",
		"map":         "geo",
		"sunburst":    "hierarchy",
		"kpi":         "single_value",
	}
	for visualType, want := range tests {
		t.Run(visualType, func(t *testing.T) {
			if got := agentVisualShape(agentVisualInput{Type: visualType}); got != want {
				t.Fatalf("shape = %q, want %q", got, want)
			}
		})
	}
}

func TestAgentVisualInputRejectsLegacyAndUnknownProperties(t *testing.T) {
	for _, property := range []string{"shape", "options", "rendererOptions", "unexpected"} {
		t.Run(property, func(t *testing.T) {
			_, err := decodeAgentVisualInput([]byte(`{"type":"histogram","model":"sales","dataset":"orders","` + property + `":{}}`))
			if err == nil || !strings.Contains(err.Error(), property) {
				t.Fatalf("decode error = %v, want closed-contract rejection for %q", err, property)
			}
		})
	}
	for _, property := range []string{`"shape"`, `"options"`, `"rendererOptions"`} {
		if strings.Contains(agentVisualToolSchema, property) {
			t.Fatalf("agent schema still exposes legacy property %s", property)
		}
	}
}

func TestAgentVisualInputAcceptsAndNormalizesGovernedFilters(t *testing.T) {
	input, err := decodeAgentVisualInput([]byte(`{
		"workspace":"sales",
		"type":"bar",
		"model":"commerce",
		"dataset":"commerce.orders",
		"dimensions":[{"field":"commerce.orders.country"}],
		"measures":[{"field":"commerce.revenue"}],
		"filters":[{
			"field":"commerce.orders.country",
			"fact":"commerce.orders",
			"operator":"in",
			"values":["DK","SE"],
			"groups":[{"filters":[{"field":"commerce.orders.status","operator":"not_contains","values":["cancelled"]}]}]
		}]
	}`))
	if err != nil {
		t.Fatalf("decodeAgentVisualInput(): %v", err)
	}
	if input.Dataset != "orders" || len(input.Filters) != 1 {
		t.Fatalf("normalized input = %#v", input)
	}
	want := agentVisualFilter{
		Field: "orders.country", Fact: "orders", Operator: "in", Values: []string{"DK", "SE"},
		Groups: []agentVisualFilterGroup{{Filters: []agentVisualFilter{{
			Field: "orders.status", Operator: "not_contains", Values: []string{"cancelled"},
		}}}},
	}
	if !reflect.DeepEqual(input.Filters[0], want) {
		t.Fatalf("normalized filter = %#v, want %#v", input.Filters[0], want)
	}
}

func TestAgentVisualInputAcceptsGroupOnlyFilters(t *testing.T) {
	input, err := decodeAgentVisualInput([]byte(`{
		"workspace":"sales",
		"type":"bar",
		"model":"commerce",
		"dataset":"orders",
		"dimensions":[{"field":"orders.country"}],
		"measures":[{"field":"revenue"}],
		"filters":[{"groups":[{"filters":[{"field":"orders.country","operator":"equals","values":["DK"]}]}]}]
	}`))
	if err != nil {
		t.Fatalf("decodeAgentVisualInput(): %v", err)
	}
	if len(input.Filters) != 1 || len(input.Filters[0].Groups) != 1 {
		t.Fatalf("group-only filters = %#v", input.Filters)
	}
}

func TestAgentVisualQueriesApplyGovernedFilters(t *testing.T) {
	var captured reportdef.AggregateQuery
	provider := VisualProvider{
		AggregateRows: func(_ context.Context, _, _ string, request reportdef.AggregateQuery) (reportdef.QueryRows, error) {
			captured = request
			return reportdef.QueryRows{{"label": "DK", "value": 3}}, nil
		},
	}
	input := agentVisualInput{
		Type: "bar", Dataset: "orders", Model: "commerce",
		Dimensions: []agentVisualFieldRef{{Field: "orders.country"}},
		Measures:   []agentVisualFieldRef{{Field: "revenue"}},
		Filters: []agentVisualFilter{{
			Field: "orders.country", Operator: "equals", Values: []string{"DK"},
		}},
		Limit: 10,
	}
	_, err := provider.agentChartData(context.Background(), "sales", input, "category_value", &semanticmodel.Model{})
	if err != nil {
		t.Fatalf("agentChartData(): %v", err)
	}
	want := []reportdef.QueryFilter{{Field: "orders.country", Operator: "equals", Values: []any{"DK"}}}
	if !reflect.DeepEqual(captured.Filters, want) {
		t.Fatalf("query filters = %#v, want %#v", captured.Filters, want)
	}
}

func TestAgentVisualFieldUsagePreservesSemanticUnitsAndFormats(t *testing.T) {
	model := &semanticmodel.Model{
		Measures: map[string]semanticmodel.MetricMeasure{
			"return_rate": {Label: "Return rate", Unit: "percent", Format: "percent_1"},
		},
	}
	got := agentVisualFieldUsage("sales", "commerce", model, agentVisualFieldRef{Field: "return_rate", Alias: "rate"}, "measure")
	if got.Ref.Type != "measure" || got.Ref.ID != "commerce.return_rate" || got.Label != "Return rate" ||
		got.Alias == nil || *got.Alias != "rate" || got.Unit == nil || *got.Unit != "percent" ||
		got.Format == nil || *got.Format != "percent_1" {
		t.Fatalf("field usage = %#v", got)
	}
}

func TestAgentHistogramProducesBinnedPayload(t *testing.T) {
	provider := VisualProvider{
		Histogram: func(context.Context, string, string, reportdef.RawValueQuery, int) ([]reportdef.HistogramBin, error) {
			return []reportdef.HistogramBin{{Bucket: 0, Count: 4, Start: 10, End: 20}}, nil
		},
	}
	input := agentVisualInput{
		Type: "histogram", Dataset: "orders", Model: "sales",
		Measures: []agentVisualFieldRef{{Field: "revenue"}}, Presentation: agentVisualPresentation{HistogramBins: 12},
	}
	data, err := provider.agentChartData(context.Background(), "sales", input, agentVisualShape(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 || data[0]["binStart"] != float64(10) || data[0]["binEnd"] != float64(20) || data[0]["value"] != 4 {
		t.Fatalf("histogram data = %#v", data)
	}
}
