package command

import (
	"context"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yacobolo/libredash/internal/dashboard"
	"github.com/Yacobolo/libredash/internal/dashboard/stream"
)

type fakeMetrics struct {
	canceledTable bool
	refreshErr    error
	queries       []string
}

func (fakeMetrics) DefaultDashboardID() string                            { return "dash" }
func (fakeMetrics) ModelIDForDashboard(string) string                     { return "model" }
func (fakeMetrics) DataDir() string                                       { return ".data" }
func (fakeMetrics) RefreshMaterializations(context.Context, string) error { return nil }
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
		Tables:  map[string]reportdef.TableVisual{"orders": {}},
		Pages:   []dashboard.Page{{ID: "overview", Visuals: []dashboard.PageVisual{{Kind: "table", Table: "orders"}}}},
	}, &semanticmodel.Model{Name: "model"}, true
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

func TestTableWindowPublishesTablePatch(t *testing.T) {
	metrics := &fakeMetrics{}
	broker := stream.NewBroker()
	updates, unsubscribe := broker.Subscribe("client:dash:overview")
	defer unsubscribe()

	req := httptest.NewRequest(http.MethodPost, "/commands/table-window", strings.NewReader(`{"runtime":{"clientId":"client","dashboardId":"dash","pageId":"overview"},"tableCommand":{"table":"orders","block":"a","start":50,"count":50}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	Handler{Metrics: metrics, Broker: broker}.TableWindow(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	select {
	case patch := <-updates:
		tables, ok := patch["tables"].(map[string]dashboard.Table)
		if !ok || tables["orders"].Title != "orders" {
			t.Fatalf("patch = %#v", patch)
		}
	default:
		t.Fatal("missing table patch")
	}
}

func TestTableWindowSkipsCanceledTablePatch(t *testing.T) {
	broker := stream.NewBroker()
	updates, unsubscribe := broker.Subscribe("client:dash:overview")
	defer unsubscribe()

	req := httptest.NewRequest(http.MethodPost, "/commands/table-window", strings.NewReader(`{"runtime":{"clientId":"client","dashboardId":"dash","pageId":"overview"},"tableCommand":{"table":"orders","block":"a","start":50,"count":50}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	Handler{Metrics: &fakeMetrics{canceledTable: true}, Broker: broker}.TableWindow(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	select {
	case patch := <-updates:
		t.Fatalf("unexpected canceled table patch: %#v", patch)
	default:
	}
}

func TestChartSelectPublishesReloadPatchesForActivePage(t *testing.T) {
	metrics := &fakeMetrics{}
	broker := stream.NewBroker()
	updates, unsubscribe := broker.Subscribe("client:dash:overview")
	defer unsubscribe()

	req := httptest.NewRequest(http.MethodPost, "/commands/chart-select", strings.NewReader(`{"runtime":{"clientId":"client","dashboardId":"dash","pageId":"overview"},"filters":{"visualSelections":[]},"visualCommand":{"visualId":"chart","field":"state","value":"SP","label":"SP"},"tableCommand":{"table":"orders","block":"a","start":50,"count":50}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	Handler{Metrics: metrics, Broker: broker}.ChartSelect(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	for _, key := range []string{"status", "filters", "tables"} {
		patch := <-updates
		if _, ok := patch[key]; !ok {
			t.Fatalf("patch missing %q: %#v", key, patch)
		}
	}
	if len(metrics.queries) != 1 || metrics.queries[0] != "overview" {
		t.Fatalf("queries = %#v", metrics.queries)
	}
}
