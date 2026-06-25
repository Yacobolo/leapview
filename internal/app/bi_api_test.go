package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yacobolo/libredash/internal/dashboard"
)

func TestBIAPIListResponsesUseStandardEnvelope(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{Store: testStore(t), DefaultWorkspaceID: "test"})

	for _, tc := range []struct {
		path string
		name string
	}{
		{path: "/api/v1/workspaces/test/dashboards?limit=1", name: "dashboards"},
		{path: "/api/v1/workspaces/test/semantic-models?limit=1", name: "semantic models"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		server.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
		var response struct {
			Items []map[string]any `json:"items"`
			Page  struct {
				NextCursor string `json:"nextCursor"`
			} `json:"page"`
			Dashboards any `json:"dashboards"`
			Models     any `json:"models"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode %s response: %v body=%s", tc.name, err, rec.Body.String())
		}
		if len(response.Items) != 1 {
			t.Fatalf("%s items = %#v", tc.name, response.Items)
		}
		if response.Dashboards != nil || response.Models != nil {
			t.Fatalf("%s response leaked legacy wrapper: %s", tc.name, rec.Body.String())
		}
	}

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/api/v1/workspaces/test/dashboards/executive-sales", want: `"detail_tools"`},
		{path: "/api/v1/workspaces/test/semantic-models/test", want: `"model_tables"`},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		server.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s status=%d body=%s", tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestBIAPIListPaginationRejectsMalformedLimit(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{Store: testStore(t), DefaultWorkspaceID: "test"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/test/dashboards?limit=oops", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertAPIError(t, rec, http.StatusBadRequest, "limit")
}

func TestBIAPIQueriesBoundRowsAndPageData(t *testing.T) {
	server := NewWithOptions(manyRowsMetrics{}, Options{Store: testStore(t), DefaultWorkspaceID: "test"})

	pageReq := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/test/dashboards/executive-sales/pages/overview/query", strings.NewReader(`{"filters":{"controls":{"state":{"type":"multi_select","operator":"in","values":["SP"]}}}}`))
	pageReq.Header.Set("Accept", "application/json")
	pageReq.Header.Set("Content-Type", "application/json")
	pageRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK || !strings.Contains(pageRec.Body.String(), `"visuals"`) {
		t.Fatalf("page query status=%d body=%s", pageRec.Code, pageRec.Body.String())
	}

	tableReq := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/test/dashboards/executive-sales/tables/orders/query", strings.NewReader(`{"pageId":"overview","count":500}`))
	tableReq.Header.Set("Accept", "application/json")
	tableReq.Header.Set("Content-Type", "application/json")
	tableRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(tableRec, tableReq)
	if tableRec.Code != http.StatusOK {
		t.Fatalf("table query status=%d body=%s", tableRec.Code, tableRec.Body.String())
	}
	var table dashboard.Table
	if err := json.Unmarshal(tableRec.Body.Bytes(), &table); err != nil {
		t.Fatalf("decode table: %v body=%s", err, tableRec.Body.String())
	}
	if table.AvailableRows != 50 || len(table.Blocks["a"].Rows) != 50 {
		t.Fatalf("table not capped to 50 rows: %#v", table)
	}
}

type manyRowsMetrics struct {
	fakeMetrics
}

func (manyRowsMetrics) QueryTablePage(_ context.Context, _ string, _ string, _ dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error) {
	rows := make([]map[string]any, 0, request.Count)
	for i := 0; i < request.Count; i++ {
		rows = append(rows, map[string]any{"order_id": i})
	}
	return dashboard.Table{
		Title:         "Orders",
		AvailableRows: len(rows),
		Blocks:        map[string]dashboard.TableBlock{"a": {Rows: rows}},
	}, nil
}
