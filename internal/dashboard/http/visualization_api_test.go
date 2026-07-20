package http

import (
	"encoding/json"
	"testing"

	"github.com/Yacobolo/libredash/internal/dashboard"
	visualizationdefinition "github.com/Yacobolo/libredash/internal/visualization/definition"
	visualizationruntime "github.com/Yacobolo/libredash/internal/visualization/runtime"
)

func TestDashboardVisualizationDescriptionContainsOnlyCompiledContract(t *testing.T) {
	envelope, err := visualizationruntime.VisualEnvelope(dashboard.Visual{
		ID: "revenue", Type: "line", Title: "Revenue",
		Data: []dashboard.Datum{{"label": "July", "value": 42.0}},
	}, 0, 0)
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}
	definition, err := visualizationdefinition.New("revenue", envelope.Spec, visualizationdefinition.QueryBinding{
		Kind: visualizationdefinition.QueryAggregate, ModelID: "sales", DatasetID: "primary",
		Aggregate: &visualizationdefinition.AggregateQueryBinding{
			TableID: "orders", Dimensions: []visualizationdefinition.FieldBinding{{FieldID: "orders.month", Alias: "label"}},
			Measures: []visualizationdefinition.FieldBinding{{FieldID: "orders.revenue", Alias: "value"}}, Limit: 1000,
		},
	})
	if err != nil {
		t.Fatalf("compile definition: %v", err)
	}
	payload, err := json.Marshal(dashboardVisualizationDefinitionDTO(definition, dashboard.PageVisual{ID: "revenue-card"}))
	if err != nil {
		t.Fatalf("marshal description: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode description: %v", err)
	}
	for _, required := range []string{"id", "rendererID", "specRevision", "spec"} {
		if _, ok := decoded[required]; !ok {
			t.Fatalf("description missing %q: %s", required, payload)
		}
	}
	for _, legacy := range []string{"shape", "renderer", "options", "extensions", "query", "columns"} {
		if _, ok := decoded[legacy]; ok {
			t.Fatalf("description retained legacy field %q: %s", legacy, payload)
		}
	}
}

func TestDashboardGridJSONUsesVisualizationEnvelope(t *testing.T) {
	envelope, err := visualizationruntime.TableEnvelope("orders", dashboard.Table{
		Title: "Orders", Columns: []dashboard.TableColumn{{Key: "order_id", Label: "Order ID"}},
		Cardinality: dashboard.ExactCardinality(1), AvailableRows: 1,
		Blocks: map[string]dashboard.TableBlock{"a": {Rows: []map[string]any{{"order_id": "A-1"}}}},
	}, 7, 3)
	if err != nil {
		t.Fatalf("build table envelope: %v", err)
	}
	if envelope.RendererID != visualizationdefinition.RendererTanStack || envelope.DataRevision != 7 {
		t.Fatalf("envelope = %#v", envelope)
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	for _, legacy := range []string{"rows", "columns", "queryId", "servingSnapshot"} {
		if _, ok := decoded[legacy]; ok {
			t.Fatalf("grid response retained legacy top-level field %q: %s", legacy, encoded)
		}
	}
}
