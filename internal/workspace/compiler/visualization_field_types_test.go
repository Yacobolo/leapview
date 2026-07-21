package compiler

import (
	"strings"
	"testing"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	visualizationir "github.com/Yacobolo/libredash/internal/visualization/ir"
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

func TestCompiledHierarchyRejectsReservedFrameAliases(t *testing.T) {
	t.Parallel()
	authored := reportdef.Visual{Title: "Hierarchy", Type: "tree", Query: reportdef.VisualQuery{
		Dimensions: []reportdef.FieldRef{{Field: "orders.category", Alias: "node"}},
		Measures:   []reportdef.FieldRef{{Field: "order_count", Alias: "value"}},
	}}
	_, err := compileBuiltInVisualizationSpec("hierarchy", authored, nil)
	if err == nil || !strings.Contains(err.Error(), `alias "node" conflicts with a reserved frame field`) {
		t.Fatalf("compileBuiltInVisualizationSpec() error = %v", err)
	}
}

func TestCompiledHierarchyFrameBudgetAccountsForMaterializedAncestors(t *testing.T) {
	t.Parallel()

	authored := reportdef.Visual{Title: "Hierarchy", Type: "treemap", Query: reportdef.VisualQuery{
		Dimensions: []reportdef.FieldRef{{Field: "orders.category", Alias: "category"}, {Field: "orders.status", Alias: "status"}},
		Measures:   []reportdef.FieldRef{{Field: "order_count", Alias: "order_count"}},
		Limit:      80,
	}}
	spec, err := compileBuiltInVisualizationSpec("hierarchy", authored, nil)
	if err != nil {
		t.Fatal(err)
	}
	base, err := visualizationir.SpecificationBase(spec)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := base.DataBudget.MaxRows, int64(160); got != want {
		t.Fatalf("hierarchy frame budget = %d, want %d", got, want)
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
