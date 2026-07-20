package compiler

import (
	"testing"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
)

func TestCompiledDimensionFormatPreservesSemanticScalarTypes(t *testing.T) {
	t.Parallel()
	for semanticType, want := range map[string]string{
		"string": "", "number": "decimal", "boolean": "boolean", "date": "date", "timestamp": "timestamp",
	} {
		if got := compiledDimensionFormat(semanticType); got != want {
			t.Errorf("compiledDimensionFormat(%q) = %q, want %q", semanticType, got, want)
		}
	}
}

func TestCompiledPhysicalFieldFormatUsesMeasureSemanticsWhenModelTypeIsUnknown(t *testing.T) {
	model := &semanticmodel.Model{Measures: map[string]semanticmodel.MetricMeasure{
		"revenue": {Input: semanticmodel.MeasureInput{Field: "orders.revenue"}, Aggregation: "sum", Format: "currency"},
	}}
	if got := compiledPhysicalFieldFormat(model, "orders.revenue", ""); got != "currency" {
		t.Fatalf("compiledPhysicalFieldFormat = %q, want currency", got)
	}
}

func TestCompiledPhysicalFieldFormatDoesNotTreatCountIdentityAsNumeric(t *testing.T) {
	model := &semanticmodel.Model{Measures: map[string]semanticmodel.MetricMeasure{
		"orders": {Input: semanticmodel.MeasureInput{Field: "orders.order_id"}, Aggregation: "count_distinct"},
	}}
	if got := compiledPhysicalFieldFormat(model, "orders.order_id", ""); got != "" {
		t.Fatalf("compiledPhysicalFieldFormat = %q, want string default", got)
	}
}
