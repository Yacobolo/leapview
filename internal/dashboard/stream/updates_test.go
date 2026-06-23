package stream

import (
	"context"
	"errors"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yacobolo/libredash/internal/dashboard"
)

type fakeMetrics struct {
	queryErr error
}

func (fakeMetrics) DefaultDashboardID() string { return "dash" }
func (fakeMetrics) DataDir() string            { return ".data" }
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
		Tables:  map[string]reportdef.TableVisual{"orders": {DefaultSort: dashboard.TableSort{Key: "order_id", Direction: "asc"}}},
		Pages: []dashboard.Page{{ID: "overview", Visuals: []dashboard.PageVisual{
			{Kind: "filter_card", Filter: "state"},
			{Kind: "table", Table: "orders"},
		}}},
	}, &semanticmodel.Model{Name: "model"}, true
}
func (m fakeMetrics) QueryDashboardPage(_ context.Context, _, _ string, filters dashboard.Filters) (dashboard.Patch, error) {
	if m.queryErr != nil {
		return dashboard.Patch{}, m.queryErr
	}
	return dashboard.Patch{
		Filters:       filters.WithDefaults(),
		FilterOptions: map[string][]dashboard.FilterOption{"state": {{Value: "SP", Label: "SP"}}},
		Status:        dashboard.Status{DataDirectory: ".data"},
		Visuals:       map[string]dashboard.Visual{"orders_chart": {Title: "Orders"}},
	}, nil
}
func (fakeMetrics) QueryTablePage(_ context.Context, _, _ string, _ dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error) {
	return dashboard.Table{Title: request.Table}, nil
}

func TestUpdatesPatchOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/updates?dashboard=dash&page=overview", nil)
	rec := httptest.NewRecorder()

	Handler{Metrics: fakeMetrics{}, Broker: NewBroker(), TickerInterval: time.Hour}.Updates(rec, req)
	body := rec.Body.String()

	loading := strings.Index(body, `"loading":true`)
	visuals := strings.Index(body, `"visuals":{"orders_chart"`)
	tables := strings.Index(body, `"tables":{"orders"`)
	if loading < 0 || visuals < 0 || tables < 0 {
		t.Fatalf("missing expected patches:\n%s", body)
	}
	if !(loading < visuals && visuals < tables) {
		t.Fatalf("patch order loading=%d visuals=%d tables=%d:\n%s", loading, visuals, tables, body)
	}
}

func TestUpdatesUnsubscribesOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/updates?dashboard=dash&page=overview", nil)
	rec := httptest.NewRecorder()
	broker := NewBroker()
	done := make(chan struct{})

	go func() {
		Handler{Metrics: fakeMetrics{queryErr: errors.New("boom")}, Broker: broker, TickerInterval: time.Hour}.Updates(rec, req)
		close(done)
	}()

	key := "default:dash:overview"
	deadline := time.After(time.Second)
	for broker.SubscriberCount(key) == 0 {
		select {
		case <-deadline:
			t.Fatal("stream did not subscribe")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream did not stop after cancellation")
	}
	if count := broker.SubscriberCount(key); count != 0 {
		t.Fatalf("subscriber count after cancel = %d", count)
	}
}
