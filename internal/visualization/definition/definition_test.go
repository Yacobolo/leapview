package definition

import (
	"testing"

	"github.com/Yacobolo/libredash/internal/visualization/ir"
)

func TestDefinitionValidateRejectsRendererAndQueryMismatches(t *testing.T) {
	t.Parallel()

	tests := map[string]Definition{
		"missing identity": {ID: "orders", RendererID: RendererTanStack, Query: QueryBinding{}},
		"wrong renderer":   {ID: "orders", RendererID: RendererECharts, Query: QueryBinding{Kind: QueryDetail}, Spec: tableSpec()},
		"wrong query":      {ID: "orders", RendererID: RendererTanStack, Query: QueryBinding{Kind: QueryAggregate}, Spec: tableSpec()},
	}
	for name, definition := range tests {
		definition := definition
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := definition.Validate(); err == nil {
				t.Fatal("expected invalid definition to fail")
			}
		})
	}
}

func TestNewComputesRevisionAndSelectsOwnedRenderer(t *testing.T) {
	t.Parallel()

	definition, err := New("orders", tableSpec(), QueryBinding{
		Kind: QueryDetail, ModelID: "sales", DatasetID: "primary",
		Detail: &DetailQueryBinding{TableID: "orders", Fields: []FieldBinding{{FieldID: "orders.order_id", Alias: "order_id"}}, Limit: 1000},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if definition.RendererID != RendererTanStack {
		t.Fatalf("renderer = %q, want %q", definition.RendererID, RendererTanStack)
	}
	computed, err := ir.ComputeSpecRevision(definition.Spec)
	if err != nil {
		t.Fatalf("ComputeSpecRevision: %v", err)
	}
	if definition.SpecRevision != computed.String() {
		t.Fatalf("revision = %q, want %q", definition.SpecRevision, computed)
	}
}

func TestQueryBindingRejectsMissingAndConflictingBranches(t *testing.T) {
	t.Parallel()

	for name, binding := range map[string]QueryBinding{
		"missing branch": {Kind: QueryDetail, ModelID: "sales", DatasetID: "primary"},
		"conflicting branch": {
			Kind: QueryDetail, ModelID: "sales", DatasetID: "primary",
			Detail:    &DetailQueryBinding{TableID: "orders", Fields: []FieldBinding{{FieldID: "orders.id", Alias: "id"}}, Limit: 100},
			Aggregate: &AggregateQueryBinding{TableID: "orders", Measures: []FieldBinding{{FieldID: "orders.count", Alias: "value"}}, Limit: 1},
		},
	} {
		binding := binding
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := binding.Validate(); err == nil {
				t.Fatal("expected invalid query binding to fail")
			}
		})
	}
}

func tableSpec() ir.VisualizationSpec {
	return ir.VisualizationSpec{Value: &ir.TableVisualizationSpec{
		VisualizationSpecBase: ir.VisualizationSpecBase{
			Kind: "table", Title: "Orders",
			Datasets:      []ir.VisualizationDatasetSchema{{ID: "primary", Fields: []ir.VisualizationField{{ID: "order_id", Role: ir.VisualizationFieldRoleIdentity, DataType: ir.VisualizationDataTypeString, Label: "Order ID"}}}},
			DataBudget:    ir.VisualizationDataBudget{MaxRows: 1000, RequiredCompleteness: ir.VisualizationCompletenessPartial},
			Accessibility: ir.VisualizationAccessibility{Title: "Orders", Description: "Order details"}, Interactions: []ir.VisualizationInteraction{},
		},
		Kind: "table", Columns: []ir.TableVisualizationColumn{{Field: ir.VisualizationFieldRef{Dataset: "primary", Field: "order_id"}, Label: "Order ID"}},
		Presentation: ir.GridVisualizationPresentation{RowHeight: 32, ShowHeader: true},
	}}
}
