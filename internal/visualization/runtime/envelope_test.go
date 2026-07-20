package runtime

import (
	"testing"

	"github.com/Yacobolo/libredash/internal/dashboard"
	visualizationdefinition "github.com/Yacobolo/libredash/internal/visualization/definition"
	"github.com/Yacobolo/libredash/internal/visualization/ir"
)

func TestVisualEnvelopeFromDefinitionKeepsCompiledSpecAndStreamRevision(t *testing.T) {
	visual := dashboard.Visual{ID: "revenue", Type: "line", Title: "Runtime title", Shape: "category_value", Data: []dashboard.Datum{{"label": "Jan", "value": 10.5}}}
	draft, err := VisualEnvelope(visual, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	compiledSpec := draft.Spec
	compiledSpec.Value.(*ir.CartesianVisualizationSpec).Title = "Compiled title"
	definition, err := visualizationdefinition.New("revenue", compiledSpec, visualizationdefinition.QueryBinding{
		Kind: visualizationdefinition.QueryAggregate, ModelID: "sales", DatasetID: "primary",
		Aggregate: &visualizationdefinition.AggregateQueryBinding{Measures: []visualizationdefinition.FieldBinding{{FieldID: "revenue", Alias: "value"}}, Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := VisualEnvelopeFromDefinition(definition, visual, 9, 4)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.SpecRevision != definition.SpecRevision || envelope.Spec.Value.(*ir.CartesianVisualizationSpec).Title != "Compiled title" {
		t.Fatalf("envelope did not retain compiled specification: %#v", envelope)
	}
	state := envelope.DataState.Value.(*ir.InlineVisualizationDataState)
	if envelope.DataRevision != 9 || state.DataRevision != 9 || state.Datasets[0].SpecRevision != definition.SpecRevision {
		t.Fatalf("stream revision was not applied: %#v", state)
	}
}

func TestVisualEnvelopeFromDefinitionUsesCompiledDatasetOrdering(t *testing.T) {
	draft, err := VisualEnvelope(dashboard.Visual{
		ID: "revenue", Type: "line", Title: "Revenue", Shape: "category_value",
		Data: []dashboard.Datum{{"label": "Jan", "value": 10.5}},
	}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	compiled := draft.Spec.Value.(*ir.CartesianVisualizationSpec)
	compiled.Datasets[0].Fields[0], compiled.Datasets[0].Fields[1] = compiled.Datasets[0].Fields[1], compiled.Datasets[0].Fields[0]
	definition, err := visualizationdefinition.New("revenue", draft.Spec, visualizationdefinition.QueryBinding{
		Kind: visualizationdefinition.QueryAggregate, ModelID: "sales", DatasetID: "primary",
		Aggregate: &visualizationdefinition.AggregateQueryBinding{Measures: []visualizationdefinition.FieldBinding{{FieldID: "revenue", Alias: "value"}}, Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Runtime presentation metadata is deliberately contradictory. The
	// immutable compiled schema is the only source of frame ordering.
	envelope, err := VisualEnvelopeFromDefinition(definition, dashboard.Visual{
		ID: "revenue", Type: "kpi", Shape: "single_value",
		Data: []dashboard.Datum{{"value": 10.5, "label": "Jan"}},
	}, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	state := envelope.DataState.Value.(*ir.InlineVisualizationDataState)
	if got, want := state.Datasets[0].Columns, []string{"value", "label"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("columns = %#v, want compiled order %#v", got, want)
	}
}

func TestVisualEnvelopeFromDefinitionProjectsSelectionAsDatumRef(t *testing.T) {
	visual := dashboard.Visual{
		ID: "orders", Type: "bar", Title: "Orders", Shape: "category_value",
		Interaction: dashboard.InteractionConfig{Kind: "point_selection", Mappings: []dashboard.InteractionConfigMapping{{Field: "orders.status", Fact: "orders", Value: "label"}}},
		Selection:   []dashboard.InteractionSelectionEntry{{Mappings: []dashboard.InteractionSelectionMapping{{Field: "orders.status", Fact: "orders", Value: "delivered"}}, Label: "Delivered"}},
		Data:        []dashboard.Datum{{"label": "delivered", "value": 42}},
	}
	draft, err := VisualEnvelope(visual, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	definition, err := visualizationdefinition.New("orders", draft.Spec, visualizationdefinition.QueryBinding{
		Kind: visualizationdefinition.QueryAggregate, ModelID: "sales", DatasetID: "primary", Identity: []string{"label"},
		Aggregate: &visualizationdefinition.AggregateQueryBinding{Dimensions: []visualizationdefinition.FieldBinding{{FieldID: "orders.status", Alias: "label"}}, Measures: []visualizationdefinition.FieldBinding{{FieldID: "order_count", Alias: "value"}}, Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := VisualEnvelopeFromDefinition(definition, visual, 8, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope.Selection) != 1 || envelope.Selection[0].Datum.DataRevision != 8 || envelope.Selection[0].Datum.Identity["label"] != "delivered" {
		t.Fatalf("selection = %#v", envelope.Selection)
	}
}

func TestChartEnvelopeUsesColumnarTypedIR(t *testing.T) {
	t.Parallel()
	visual := dashboard.Visual{ID: "revenue", Type: "line", Title: "Revenue", Shape: "category_value", Renderer: "echarts", Dimensions: []string{"month"}, Measures: []string{"revenue"}, Data: []dashboard.Datum{{"label": "Jan", "value": 10.5}}}
	envelope, err := VisualEnvelope(visual, 4, 2)
	if err != nil {
		t.Fatalf("VisualEnvelope: %v", err)
	}
	if envelope.RendererID != "echarts" {
		t.Fatalf("renderer = %q", envelope.RendererID)
	}
	state := envelope.DataState.Value.(*ir.InlineVisualizationDataState)
	if len(state.Datasets) != 1 || len(state.Datasets[0].Rows) != 1 || len(state.Datasets[0].Rows[0]) != 2 {
		t.Fatalf("unexpected columnar state: %#v", state)
	}
	if err := ir.ValidateEnvelope(envelope); err != nil {
		t.Fatalf("ValidateEnvelope: %v", err)
	}
}

func TestTableEnvelopePreservesWindowIdentity(t *testing.T) {
	t.Parallel()
	count := 1
	table := dashboard.Table{Kind: "data_table", Title: "Orders", Columns: []dashboard.TableColumn{{Key: "order_id", Label: "Order", Role: "row_header"}}, Cardinality: dashboard.ExactCardinality(count), AvailableRows: count, RowCap: 100, ChunkSize: 50, RowHeight: 34, ResetVersion: 3, Sort: dashboard.TableSort{Key: "order_id", Direction: "asc"}, Blocks: map[string]dashboard.TableBlock{"a": {Start: 0, RequestSeq: 7, ResetVersion: 3, Sort: dashboard.TableSort{Key: "order_id", Direction: "asc"}, Rows: []map[string]any{{"order_id": "one"}}}}}
	envelope, err := TableEnvelope("orders", table, 8, 5)
	if err != nil {
		t.Fatalf("TableEnvelope: %v", err)
	}
	state := envelope.DataState.Value.(*ir.WindowedVisualizationDataState)
	if state.Blocks["a"].RequestSeq != 7 || state.ResetVersion != 3 {
		t.Fatalf("window identity lost: %#v", state)
	}
	if err := ir.ValidateEnvelope(envelope); err != nil {
		t.Fatalf("ValidateEnvelope: %v", err)
	}
}

func TestTableEnvelopeOmitsUnknownCardinalityCount(t *testing.T) {
	t.Parallel()

	table := dashboard.Table{
		Kind: "data_table", Title: "Orders", Columns: []dashboard.TableColumn{{Key: "order_id", Label: "Order", Role: "row_header"}},
		Cardinality: dashboard.TableCardinality{Kind: dashboard.CardinalityUnknown}, AvailableRows: 10000,
		RowCap: 10000, ChunkSize: 50, RowHeight: 34, Sort: dashboard.TableSort{Key: "order_id", Direction: "asc"}, Blocks: map[string]dashboard.TableBlock{},
	}
	envelope, err := TableEnvelope("orders", table, 1, 1)
	if err != nil {
		t.Fatalf("TableEnvelope: %v", err)
	}
	state, ok := envelope.DataState.Value.(*ir.WindowedVisualizationDataState)
	if !ok {
		t.Fatalf("data state = %T", envelope.DataState.Value)
	}
	if state.Cardinality.Count != nil {
		t.Fatalf("unknown cardinality count = %v, want nil", *state.Cardinality.Count)
	}
}

func TestEnvelopePreservesServerInteractionIdentity(t *testing.T) {
	t.Parallel()
	visual := dashboard.Visual{
		ID: "orders", Type: "bar", Title: "Orders",
		Interaction: dashboard.InteractionConfig{Kind: "point_selection", Toggle: true, Mappings: []dashboard.InteractionConfigMapping{{Field: "orders.status", Fact: "orders", Value: "label", Label: "label"}}},
		Data:        []dashboard.Datum{{"label": "delivered", "value": 1}},
	}
	envelope, err := VisualEnvelope(visual, 1, 1)
	if err != nil {
		t.Fatalf("VisualEnvelope: %v", err)
	}
	interactions := envelope.Spec.Value.(*ir.CartesianVisualizationSpec).Interactions
	if len(interactions) != 1 || interactions[0].ID != "point_selection" || interactions[0].Mode != ir.VisualizationSelectionModeMultiple {
		t.Fatalf("interaction identity was not preserved: %#v", interactions)
	}
}
