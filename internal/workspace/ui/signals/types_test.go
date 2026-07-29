package signals

import (
	"encoding/json"
	"strings"
	"testing"

	semanticmodel "github.com/flidai/leapview/internal/analytics/model"
	"github.com/flidai/leapview/internal/dashboard"
	dashboarddefinition "github.com/flidai/leapview/internal/dashboard/definition"
	dashboardfilter "github.com/flidai/leapview/internal/dashboard/filter"
	reportdef "github.com/flidai/leapview/internal/dashboard/report"
	dashboardsignals "github.com/flidai/leapview/internal/dashboard/ui/signals"
	visualizationdefinition "github.com/flidai/leapview/internal/dashboard/visualization/definition"
	workspacecompiler "github.com/flidai/leapview/internal/project/compiler"
)

func TestVisualizationSignalKeepsDataStateOpaque(t *testing.T) {
	report := testDashboardReport()
	model := testSemanticModel()
	compiled, definitions := compiledTestDashboard(t, &report, model)
	envelope := dashboardsignals.DashboardInitialEnvelope("client", "stream-instance", dashboard.Catalog{}, compiled, model, definitions, report.Pages, report.Pages[0], dashboard.Filters{})

	encoded, err := json.Marshal(envelope.Visuals["active_chart"])
	if err != nil {
		t.Fatal(err)
	}
	var signal map[string]any
	if err := json.Unmarshal(encoded, &signal); err != nil {
		t.Fatal(err)
	}
	transport, ok := signal["dataState"].(map[string]any)
	if !ok {
		t.Fatalf("visualization signal must encode data state through one typed transport: %s", encoded)
	}
	if transport["schemaVersion"] != float64(1) || transport["encoding"] != "json" || transport["kind"] != "inline" {
		t.Fatalf("visualization data-state transport header = %#v", transport)
	}
	if _, ok := transport["payload"].(string); !ok {
		t.Fatalf("visualization data-state transport payload must stay opaque: %#v", transport)
	}
	if _, ok := signal["dataStateJson"]; ok {
		t.Fatalf("legacy unversioned dataStateJson must not be emitted: %s", encoded)
	}
}

func compiledTestVisualizations(t *testing.T, report *reportdef.Dashboard, model *semanticmodel.Model) map[string]visualizationdefinition.Definition {
	t.Helper()
	definitions, err := workspacecompiler.CompileVisualizationDefinitions(report, model)
	if err != nil {
		t.Fatal(err)
	}
	return definitions
}

func compiledTestDashboard(t *testing.T, report *reportdef.Dashboard, model *semanticmodel.Model) (dashboarddefinition.Definition, map[string]visualizationdefinition.Definition) {
	t.Helper()
	if err := workspacecompiler.ValidateDashboard(report, map[string]*semanticmodel.Model{model.Name: model}); err != nil {
		t.Fatal(err)
	}
	definitions := compiledTestVisualizations(t, report, model)
	compiled, err := workspacecompiler.CompileDashboardDefinition(report, definitions)
	if err != nil {
		t.Fatal(err)
	}
	return compiled, definitions
}

func TestDashboardInitialEnvelopeValidatesPageScopedPayloads(t *testing.T) {
	report := testDashboardReport()
	model := testSemanticModel()
	compiled, definitions := compiledTestDashboard(t, &report, model)
	envelope := dashboardsignals.DashboardInitialEnvelope("client", "stream-instance", dashboard.Catalog{}, compiled, model, definitions, report.Pages, report.Pages[0], dashboard.Filters{})

	if err := dashboardsignals.ValidateDashboardEnvelope(envelope); err != nil {
		t.Fatalf("validate dashboard envelope: %v", err)
	}
	if _, ok := envelope.Visuals["active_chart"]; !ok {
		t.Fatalf("active visual missing: %#v", envelope.Visuals)
	}
	if _, ok := envelope.Visuals["off_page_chart"]; ok {
		t.Fatalf("off-page visual was emitted: %#v", envelope.Visuals)
	}
	if len(envelope.FilterContract.Bindings) != 2 {
		t.Fatalf("dashboard filter bindings = %#v, want both page bindings", envelope.FilterContract.Bindings)
	}
	if len(envelope.FilterState.AppliedControls) != 2 {
		t.Fatalf("applied filter state = %#v, want both page bindings", envelope.FilterState)
	}
	if envelope.Runtime.StreamInstanceID == nil || *envelope.Runtime.StreamInstanceID != "stream-instance" {
		t.Fatalf("stream instance id = %#v", envelope.Runtime.StreamInstanceID)
	}
	if envelope.Status.RefreshID != "" || envelope.Status.Generation != 0 {
		t.Fatalf("initial refresh status = %#v", envelope.Status)
	}
	if envelope.AgentContext.Surface != "dashboard" || envelope.AgentContext.PageID != report.Pages[0].ID || envelope.AgentContext.ModelID != model.Name {
		t.Fatalf("agent context = %#v", envelope.AgentContext)
	}
	if envelope.AgentContext.References == nil || envelope.AgentVisuals == nil {
		t.Fatalf("dashboard agent collections must be non-nil: context=%#v visuals=%#v", envelope.AgentContext, envelope.AgentVisuals)
	}
}

func TestDashboardEnvelopeRejectsMissingReferencedPayload(t *testing.T) {
	report := testDashboardReport()
	model := testSemanticModel()
	compiled, definitions := compiledTestDashboard(t, &report, model)
	envelope := dashboardsignals.DashboardInitialEnvelope("client", "stream-instance", dashboard.Catalog{}, compiled, model, definitions, report.Pages, report.Pages[0], dashboard.Filters{})
	delete(envelope.Visuals, "active_chart")

	err := dashboardsignals.ValidateDashboardEnvelope(envelope)
	if err == nil || !strings.Contains(err.Error(), `missing visual "active_chart"`) {
		t.Fatalf("validate error = %v", err)
	}
}

func TestDashboardEnvelopeRejectsUnusedPayload(t *testing.T) {
	report := testDashboardReport()
	model := testSemanticModel()
	compiled, definitions := compiledTestDashboard(t, &report, model)
	envelope := dashboardsignals.DashboardInitialEnvelope("client", "stream-instance", dashboard.Catalog{}, compiled, model, definitions, report.Pages, report.Pages[0], dashboard.Filters{})
	envelope.Visuals["off_page_chart"] = envelope.Visuals["active_chart"]

	err := dashboardsignals.ValidateDashboardEnvelope(envelope)
	if err == nil || !strings.Contains(err.Error(), `unused visual payload "off_page_chart"`) {
		t.Fatalf("validate error = %v", err)
	}
}

func testDashboardReport() reportdef.Dashboard {
	return reportdef.Dashboard{
		ID:            "report",
		Title:         "Report",
		SemanticModel: "test",
		FilterDefinitions: map[string]dashboardfilter.Definition{
			"state": {
				Label: "State", Field: "orders.state",
				Predicates: []dashboardfilter.PredicatePolicy{{Kind: dashboardfilter.ExpressionSet, Operators: []dashboardfilter.Operator{dashboardfilter.OperatorIn}}},
				Options:    dashboardfilter.OptionSource{Kind: dashboardfilter.OptionSourceDistinct, Limit: 50},
			},
			"category": {
				Label: "Category", Field: "orders.category",
				Predicates: []dashboardfilter.PredicatePolicy{{Kind: dashboardfilter.ExpressionComparison, Operators: []dashboardfilter.Operator{dashboardfilter.OperatorContains}}},
			},
		},
		Visuals: reportdef.MergeVisualizations(reportdef.ChartVisualizations(map[string]reportdef.Visual{
			"active_chart":   {Title: "Active", Type: "bar", Query: reportdef.VisualQuery{Dimensions: testFieldRefs("orders.status"), Measures: testFieldRefs("order_count")}},
			"off_page_chart": {Title: "Off Page", Type: "bar", Query: reportdef.VisualQuery{Dimensions: testFieldRefs("orders.status"), Measures: testFieldRefs("order_count")}},
		}), reportdef.TabularVisualizations("table", map[string]reportdef.TableVisual{
			"orders": {Title: "Orders", Query: reportdef.TableQuery{Table: "orders", Fields: []string{"orders.order_id"}}, Columns: []dashboard.TableColumn{{Key: "order_id", Label: "Order"}}},
		})),
		Pages: []dashboard.Page{
			{
				ID:     "overview",
				Title:  "Overview",
				Canvas: dashboard.PageCanvas{Width: 1200, Height: 800},
				FilterBindings: map[string]dashboardfilter.Binding{
					"state": {
						Filter:  "state",
						Default: dashboardfilter.Expression{Kind: dashboardfilter.ExpressionUnfiltered},
						URL:     dashboardfilter.URLPolicy{Param: "state", Encoding: dashboardfilter.URLEncodingTypedV1},
					},
				},
				Visuals: []dashboard.PageVisual{
					{ID: "state-slicer", Kind: "slicer", Binding: dashboardfilter.BindingRef{Scope: dashboardfilter.ScopePage, ID: "state"}, Placement: dashboard.PagePlacement{Col: 1, Row: 1, ColSpan: 3, RowSpan: 1}},
					{ID: "chart", Kind: "visual", Visual: "active_chart", Placement: dashboard.PagePlacement{Col: 1, Row: 2, ColSpan: 6, RowSpan: 4}},
				},
			},
			{
				ID:     "detail",
				Title:  "Detail",
				Canvas: dashboard.PageCanvas{Width: 1200, Height: 800},
				FilterBindings: map[string]dashboardfilter.Binding{
					"category": {
						Filter:  "category",
						Default: dashboardfilter.Expression{Kind: dashboardfilter.ExpressionUnfiltered},
						URL:     dashboardfilter.URLPolicy{Param: "category", Encoding: dashboardfilter.URLEncodingTypedV1},
					},
				},
				Visuals: []dashboard.PageVisual{
					{ID: "orders", Kind: "visual", Visual: "orders", Placement: dashboard.PagePlacement{Col: 1, Row: 1, ColSpan: 6, RowSpan: 4}},
				},
			},
		},
	}
}

func testSemanticModel() *semanticmodel.Model {
	return &semanticmodel.Model{
		Name:  "test",
		Title: "Test",
		Tables: map[string]semanticmodel.Table{
			"orders": {Source: "orders", PrimaryKey: "order_id", Grain: "order_id", Dimensions: map[string]semanticmodel.MetricDimension{
				"order_id": {Expr: "order_id", Type: "string"},
				"status":   {Expr: "status", Type: "string"},
				"state":    {Expr: "state", Type: "string"},
				"category": {Expr: "category", Type: "string"},
			}},
		},
		Measures: map[string]semanticmodel.MetricMeasure{"order_count": {Fact: "orders", Aggregation: "count", Empty: "zero", Label: "Orders"}},
	}
}

func testFieldRefs(fields ...string) []reportdef.FieldRef {
	refs := make([]reportdef.FieldRef, len(fields))
	for i, field := range fields {
		refs[i] = reportdef.FieldRef{Field: field}
	}
	return refs
}
