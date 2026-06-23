package datastar

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yacobolo/libredash/internal/dashboard"
)

func TestPatchKeys(t *testing.T) {
	patch := DashboardPatch(dashboard.Patch{
		Filters:       dashboard.Filters{}.WithDefaults(),
		FilterOptions: map[string][]dashboard.FilterOption{"state": {{Value: "SP", Label: "SP"}}},
		Status:        dashboard.Status{Loading: false},
		Visuals:       map[string]dashboard.Visual{"orders": {Title: "Orders"}},
	})

	for _, key := range []string{"filters", "filterOptions", "status", "visuals"} {
		if _, ok := patch[key]; !ok {
			t.Fatalf("dashboard patch missing key %q: %#v", key, patch)
		}
	}
	if _, ok := patch["tables"]; ok {
		t.Fatalf("dashboard patch should not include tables: %#v", patch)
	}
	if _, ok := patch["kpis"]; ok {
		t.Fatalf("dashboard patch should not include legacy kpis: %#v", patch)
	}

	tablePatch := TablePatch("orders", dashboard.Table{Title: "Orders"})
	tables, ok := tablePatch["tables"].(map[string]dashboard.Table)
	if !ok || tables["orders"].Title != "Orders" {
		t.Fatalf("table patch = %#v", tablePatch)
	}

	status, ok := LoadingPatch(".data")["status"].(map[string]any)
	if !ok || status["loading"] != true || status["dataDirectory"] != ".data" {
		t.Fatalf("loading patch = %#v", LoadingPatch(".data"))
	}
}

func TestEnsureClientIDKeepsExistingCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: ClientIDCookieName, Value: "client-1"})
	rec := httptest.NewRecorder()

	if got := EnsureClientID(rec, req); got != "client-1" {
		t.Fatalf("client id = %q", got)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("unexpected replacement cookie: %#v", cookies)
	}
}
