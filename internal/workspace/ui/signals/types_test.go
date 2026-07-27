package signals

import (
	"encoding/json"
	"strings"
	"testing"

	semanticmodel "github.com/Yacobolo/leapview/internal/analytics/model"
	"github.com/Yacobolo/leapview/internal/dashboard"
	dashboarddefinition "github.com/Yacobolo/leapview/internal/dashboard/definition"
	reportdef "github.com/Yacobolo/leapview/internal/dashboard/report"
	dashboardsignals "github.com/Yacobolo/leapview/internal/dashboard/ui/signals"
	visualizationdefinition "github.com/Yacobolo/leapview/internal/dashboard/visualization/definition"
	workspacecompiler "github.com/Yacobolo/leapview/internal/project/compiler"
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
	if _, ok := envelope.Filters.Controls["state"]; !ok {
		t.Fatalf("page filter control missing: %#v", envelope.Filters)
	}
	if _, ok := envelope.Filters.Controls["category"]; ok {
		t.Fatalf("off-page filter control was emitted: %#v", envelope.Filters)
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
		Filters: map[string]reportdef.FilterDefinition{
			"state":    {Type: "multi_select", Label: "State", Dimension: "orders.state", URLParam: "state", Operator: "in"},
			"category": {Type: "text", Label: "Category", Dimension: "orders.category", URLParam: "category", DefaultOperator: "contains"},
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
				Visuals: []dashboard.PageVisual{
					{ID: "state-filter", Kind: "filter", Filter: "state", X: 0, Y: 0, Width: 100, Height: 40},
					{ID: "chart", Kind: "visual", Visual: "active_chart", X: 0, Y: 48, Width: 100, Height: 100},
				},
			},
			{
				ID:     "detail",
				Title:  "Detail",
				Canvas: dashboard.PageCanvas{Width: 1200, Height: 800},
				Visuals: []dashboard.PageVisual{
					{ID: "orders", Kind: "visual", Visual: "orders", X: 0, Y: 0, Width: 100, Height: 100},
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
			"orders": {Source: "orders", PrimaryKey: "order_id", Grain: "order_id", Dimensions: map[string]semanticmodel.MetricDimension{"order_id": {Expr: "order_id"}, "status": {Expr: "status"}, "state": {Expr: "state"}, "category": {Expr: "category"}}},
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
