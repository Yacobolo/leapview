package command

import (
	"context"
	"encoding/json"
	"testing"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/dashboard"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
)

type fakeMetrics struct {
	canceledTable bool
	queries       []string
}

func (fakeMetrics) DataDir() string { return ".data" }
func (fakeMetrics) RefreshMaterializations(context.Context, string) error {
	return nil
}
func (fakeMetrics) DefaultFilters(string) dashboard.Filters {
	return dashboard.Filters{Controls: map[string]dashboard.FilterControl{"state": {Type: "multi_select", Operator: "in"}}}
}
func (fakeMetrics) NormalizeTableRequest(_ string, request dashboard.TableRequest) dashboard.TableRequest {
	if request.Table == "" {
		request.Table = "orders"
	}
	return request.WithDefaults()
}
func (fakeMetrics) Report(string) (reportdef.Dashboard, *semanticmodel.Model, bool) {
	return reportdef.Dashboard{
			Filters: map[string]reportdef.FilterDefinition{"state": {Type: "multi_select", Label: "State", Operator: "in"}},
			Visuals: map[string]reportdef.Visual{
				"chart": {
					Query: reportdef.VisualQuery{
						Dimensions: []reportdef.FieldRef{{Field: "state", Alias: "label"}},
						Measures:   []reportdef.FieldRef{{Field: "order_count", Alias: "value"}},
					},
					Interaction: reportdef.Interaction{PointSelection: reportdef.SelectionInteraction{
						Toggle:   true,
						Mappings: []reportdef.SelectionMapping{{Field: "state", Value: "label"}},
					}},
				},
				"boolean_chart": {
					Query: reportdef.VisualQuery{
						Dimensions: []reportdef.FieldRef{{Field: "active", Alias: "label"}},
						Measures:   []reportdef.FieldRef{{Field: "order_count", Alias: "value"}},
					},
					Interaction: reportdef.Interaction{PointSelection: reportdef.SelectionInteraction{
						Toggle:   true,
						Mappings: []reportdef.SelectionMapping{{Field: "active", Value: "label"}},
					}},
				},
			},
			Tables: map[string]reportdef.TableVisual{"orders": {Query: reportdef.TableQuery{Table: "orders"}}},
			Pages:  []dashboard.Page{{ID: "overview", Visuals: []dashboard.PageVisual{{Kind: "table", Table: "orders"}}}},
		}, &semanticmodel.Model{
			Name: "model",
			Tables: map[string]semanticmodel.Table{
				"orders": {Dimensions: map[string]semanticmodel.MetricDimension{"state": {Type: "string"}, "active": {Type: "boolean"}}},
			},
			Dimensions: map[string]semanticmodel.SemanticDimension{
				"state":  {Type: "string", Bindings: map[string]semanticmodel.DimensionBinding{"orders": {Field: "orders.state"}}},
				"active": {Type: "boolean", Bindings: map[string]semanticmodel.DimensionBinding{"orders": {Field: "orders.active"}}},
			},
			Measures: map[string]semanticmodel.MetricMeasure{"order_count": {Fact: "orders"}},
		}, true
}
func (m *fakeMetrics) QueryDashboardPage(_ context.Context, _, pageID string, filters dashboard.Filters) (dashboard.Patch, error) {
	m.queries = append(m.queries, pageID)
	return dashboard.Patch{Filters: filters.WithDefaults(), Status: dashboard.Status{DataDirectory: ".data"}}, nil
}
func (m *fakeMetrics) QueryTablePage(_ context.Context, _, _ string, _ dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error) {
	if m.canceledTable {
		return dashboard.EmptyTable(request, context.Canceled), nil
	}
	return dashboard.Table{Title: request.Table, Blocks: map[string]dashboard.TableBlock{"a": {Rows: []map[string]any{{"id": "1"}}}}}, nil
}

func TestTableWindowReturnsTableEvent(t *testing.T) {
	metrics := &fakeMetrics{}

	events := Service{Metrics: metrics}.TableWindow(context.Background(), Request{
		DashboardID:  "dash",
		PageID:       "overview",
		TableCommand: dashboard.TableRequest{Table: "orders", Block: "a", Start: 50, Count: 50},
	})

	if len(events) != 1 || events[0].Type != EventTable || events[0].TableName != "orders" {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Table.Title != "orders" {
		t.Fatalf("table event = %#v", events[0])
	}
}

func TestTableWindowSkipsCanceledTableEvent(t *testing.T) {
	events := Service{Metrics: &fakeMetrics{canceledTable: true}}.TableWindow(context.Background(), Request{
		DashboardID:  "dash",
		PageID:       "overview",
		TableCommand: dashboard.TableRequest{Table: "orders", Block: "a", Start: 50, Count: 50},
	})

	if len(events) != 0 {
		t.Fatalf("unexpected canceled table events: %#v", events)
	}
}

func TestSelectReturnsReloadEventsForActivePage(t *testing.T) {
	metrics := &fakeMetrics{}

	events := Service{Metrics: metrics}.Select(context.Background(), Request{
		DashboardID:  "dash",
		PageID:       "overview",
		Filters:      dashboard.Filters{Selections: []dashboard.InteractionSelection{}},
		TableCommand: dashboard.TableRequest{Table: "orders", Block: "a", Start: 50, Count: 50},
		InteractionCommand: dashboard.InteractionCommand{
			SourceKind:      "visual",
			SourceID:        "chart",
			InteractionKind: "point_selection",
			Action:          "set",
			Toggle:          true,
			Mappings: []dashboard.InteractionCommandMapping{{
				Field: "state",
				Value: "SP",
				Label: "SP",
			}},
		},
	})

	if len(events) != 3 {
		t.Fatalf("events = %#v", events)
	}
	for i, kind := range []EventType{EventLoading, EventDashboard, EventTables} {
		if events[i].Type != kind {
			t.Fatalf("event %d = %#v, want %s", i, events[i], kind)
		}
	}
	if len(metrics.queries) != 1 || metrics.queries[0] != "overview" {
		t.Fatalf("queries = %#v", metrics.queries)
	}
}

func TestSelectCanonicalizesTypedMappingsFromPublishedInteraction(t *testing.T) {
	metrics := &fakeMetrics{}
	events := Service{Metrics: metrics}.Select(context.Background(), Request{
		DashboardID: "dash",
		PageID:      "overview",
		Filters:     dashboard.Filters{}.WithDefaults(),
		InteractionCommand: dashboard.InteractionCommand{
			SourceKind:      "visual",
			SourceID:        "boolean_chart",
			InteractionKind: "point_selection",
			Action:          "set",
			Toggle:          false,
			Mappings: []dashboard.InteractionCommandMapping{{
				Field: "active",
				Value: false,
				Label: "False",
			}},
		},
	})

	if len(events) != 3 || len(events[1].Patch.Filters.Selections) != 1 {
		t.Fatalf("events = %#v", events)
	}
	mapping := events[1].Patch.Filters.Selections[0].Entries[0].Mappings[0]
	if value, ok := mapping.Value.(bool); !ok || value {
		t.Fatalf("typed mapping = %#v, want boolean false", mapping)
	}
	if got := len(events[1].Patch.Filters.Selections[0].Entries); got != 1 {
		t.Fatalf("entry count = %d, want interaction toggle from published config", got)
	}
}

func TestSelectRejectsForgedOrIncompleteMappings(t *testing.T) {
	for _, test := range []struct {
		name     string
		mappings []dashboard.InteractionCommandMapping
	}{
		{name: "forged field", mappings: []dashboard.InteractionCommandMapping{{Field: "orders.secret", Fact: "orders", Value: "x"}}},
		{name: "missing tuple", mappings: nil},
		{name: "forged grain", mappings: []dashboard.InteractionCommandMapping{{Field: "state", Grain: "month", Value: "SP"}}},
		{name: "wrong scalar type", mappings: []dashboard.InteractionCommandMapping{{Field: "state", Value: false}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			metrics := &fakeMetrics{}
			events := Service{Metrics: metrics}.Select(context.Background(), Request{
				DashboardID: "dash",
				PageID:      "overview",
				Filters:     dashboard.Filters{}.WithDefaults(),
				InteractionCommand: dashboard.InteractionCommand{
					SourceKind:      "visual",
					SourceID:        "chart",
					InteractionKind: "point_selection",
					Action:          "set",
					Mappings:        test.mappings,
				},
			})
			if len(events) != 2 || events[1].Patch.Status.Error == "" {
				t.Fatalf("events = %#v, want fail-closed dashboard error", events)
			}
			if len(metrics.queries) != 0 {
				t.Fatalf("queries = %#v, want no query for rejected selection", metrics.queries)
			}
		})
	}
}

func TestSelectRejectsMappingWithOmittedJSONValue(t *testing.T) {
	var command dashboard.InteractionCommand
	if err := json.Unmarshal([]byte(`{
		"sourceKind":"visual",
		"sourceId":"chart",
		"interactionKind":"point_selection",
		"action":"set",
		"mappings":[{"field":"state"}]
	}`), &command); err != nil {
		t.Fatal(err)
	}
	metrics := &fakeMetrics{}
	events := Service{Metrics: metrics}.Select(context.Background(), Request{
		DashboardID:        "dash",
		PageID:             "overview",
		Filters:            dashboard.Filters{}.WithDefaults(),
		InteractionCommand: command,
	})
	if len(events) != 2 || events[1].Patch.Status.Error == "" {
		t.Fatalf("events = %#v, want fail-closed dashboard error", events)
	}
	if len(metrics.queries) != 0 {
		t.Fatalf("queries = %#v, want no query for omitted value", metrics.queries)
	}
}

func TestSelectPreservesUIOnlyRowKeyForTableWithoutSemanticMappings(t *testing.T) {
	metrics := &fakeMetrics{}
	events := Service{Metrics: metrics}.Select(context.Background(), Request{
		DashboardID: "dash",
		PageID:      "overview",
		Filters:     dashboard.Filters{}.WithDefaults(),
		InteractionCommand: dashboard.InteractionCommand{
			SourceKind:      "table",
			SourceID:        "orders",
			InteractionKind: "row_selection",
			Action:          "set",
			Toggle:          true,
			Mappings: []dashboard.InteractionCommandMapping{{
				Field: dashboard.UIRowSelectionField,
				Value: "row-1",
			}},
		},
	})

	if len(events) != 3 || events[1].Patch.Status.Error != "" {
		t.Fatalf("events = %#v", events)
	}
	mapping := events[1].Patch.Filters.Selections[0].Entries[0].Mappings[0]
	if mapping.Field != dashboard.UIRowSelectionField || mapping.Value != "row-1" {
		t.Fatalf("stored UI-only mapping = %#v", mapping)
	}
}

func TestReloadReturnsReloadEventsForCurrentFilters(t *testing.T) {
	metrics := &fakeMetrics{}

	events := Service{Metrics: metrics}.Reload(context.Background(), Request{
		DashboardID:  "dash",
		PageID:       "overview",
		Filters:      dashboard.Filters{Controls: map[string]dashboard.FilterControl{"state": {Type: "multi_select", Operator: "in", Values: []string{"SP"}}}},
		TableCommand: dashboard.TableRequest{Table: "orders", Block: "a", Start: 50, Count: 50},
	})

	if len(events) != 3 {
		t.Fatalf("events = %#v", events)
	}
	for i, kind := range []EventType{EventLoading, EventDashboard, EventTables} {
		if events[i].Type != kind {
			t.Fatalf("event %d = %#v, want %s", i, events[i], kind)
		}
	}
	if events[1].Patch.Status.DataDirectory != ".data" {
		t.Fatalf("dashboard patch = %#v", events[1].Patch)
	}
	if len(metrics.queries) != 1 || metrics.queries[0] != "overview" {
		t.Fatalf("queries = %#v", metrics.queries)
	}
}
