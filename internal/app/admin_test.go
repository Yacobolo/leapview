package app

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/testutil/ssetest"
	"github.com/Yacobolo/libredash/internal/ui"
	_ "github.com/duckdb/duckdb-go/v2"
)

func TestAdminRouteRejectsViewer(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	viewer := testPrincipal(t, ctx, store, "viewer@example.com", "Viewer", access.RoleViewer)
	token := testAPIToken(t, ctx, store, viewer.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, DefaultWorkspaceID: "test"})

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/admin"},
		{method: http.MethodGet, path: "/admin/storage"},
		{method: http.MethodGet, path: "/admin/storage/updates"},
		{method: http.MethodPost, path: "/admin/storage/select-table", body: `{}`},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Routes().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s status = %d, want %d body=%s", tc.path, rec.Code, http.StatusForbidden, rec.Body.String())
		}
	}
}

func TestAdminPagesRenderReadOnlyAccessData(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner := testPrincipal(t, ctx, store, "owner@example.com", "Owner", access.RoleOwner)
	analyst := testPrincipal(t, ctx, store, "analyst@example.com", "Analyst", access.RoleViewer)
	repo := testAccessRepository(store)
	group, err := repo.UpsertGroup(ctx, access.GroupInput{ID: "group_finance", WorkspaceID: "test", Provider: "local", ExternalID: "finance", Name: "Finance"})
	if err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := repo.AddGroupMember(ctx, "test", group.ID, analyst.ID); err != nil {
		t.Fatalf("seed group member: %v", err)
	}
	if _, err := repo.CreateRoleBinding(ctx, access.RoleBindingInput{WorkspaceID: "test", SubjectType: access.SubjectGroup, SubjectID: group.ID, Role: access.RoleEditor}); err != nil {
		t.Fatalf("seed group binding: %v", err)
	}
	token := testAPIToken(t, ctx, store, owner.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	duckDBDir := seedAdminStorageDuckDB(t)
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, DefaultWorkspaceID: "test", DuckDBDir: duckDBDir})

	cases := []struct {
		path string
		want []string
	}{
		{path: "/admin", want: []string{"General", "Principals", "Groups", "Role bindings", "Roles"}},
		{path: "/admin/principals", want: []string{"Principals", "ld-data-grid", "Group count", "/admin/principals/" + analyst.ID, "analyst@example.com", "viewer", analyst.ID}},
		{path: "/admin/principals/" + analyst.ID, want: []string{"Principals / Analyst", "Email", "analyst@example.com", "Principal ID", analyst.ID, "Direct roles", "viewer", "Group count", "Groups", "/admin/groups/group_finance", "Finance", "local", "finance", "editor"}},
		{path: "/admin/groups", want: []string{"Groups", "ld-data-grid", "Member count", "/admin/groups/group_finance", "Finance", "local", "finance", "editor"}},
		{path: "/admin/groups/group_finance", want: []string{"Groups / Finance", "Provider", "local", "External ID", "finance", "Group ID", "group_finance", "Members", "Principal ID", "analyst@example.com", "viewer", analyst.ID}},
		{path: "/admin/storage", want: []string{"Storage", "DuckDB directory", "Database files", "Total size", "Tables and views", "ld-storage-explorer", "/admin/storage/updates", "/admin/storage/select-table", "libredash-test.duckdb", "orders", "rowCountLabel", "columnCount", "sizeLabel", "KiB", "customer_id", "VARCHAR", "amount", "DOUBLE"}},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		server.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", tc.path, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %q:\n%s", tc.path, want, body)
			}
		}
		for _, notWant := range []string{"/admin/access", "Assign role", "Remove access", "Refresh", "<form", "data-on:ld-workspace-access-upsert", "refresh-materializations"} {
			if strings.Contains(body, notWant) {
				t.Fatalf("%s rendered write control %q:\n%s", tc.path, notWant, body)
			}
		}
	}
}

func TestAdminStorageDetailRouteIsDropped(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner := testPrincipal(t, ctx, store, "owner@example.com", "Owner", access.RoleOwner)
	token := testAPIToken(t, ctx, store, owner.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, DefaultWorkspaceID: "test", DuckDBDir: seedAdminStorageDuckDB(t)})

	req := httptest.NewRequest(http.MethodGet, "/admin/storage/libredash-test.duckdb/model/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestAdminStorageUpdatesStreamsSignalPatch(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner := testPrincipal(t, ctx, store, "owner@example.com", "Owner", access.RoleOwner)
	token := testAPIToken(t, ctx, store, owner.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, DefaultWorkspaceID: "test", DuckDBDir: seedAdminStorageDuckDB(t)})

	reqCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequestWithContext(reqCtx, http.MethodGet, "/admin/storage/updates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content type = %q, want text/event-stream", got)
	}
	body := rec.Body.String()
	ssetest.RequirePatchSignal(t, body, func(patch map[string]any) bool {
		storage, ok := patch["adminStorage"].(map[string]any)
		if !ok {
			return false
		}
		return storage["selectedKey"] == "libredash-test.duckdb\x00model\x00order_totals" || storage["selectedKey"] == "libredash-test.duckdb\x00model\x00orders"
	})
	if !strings.Contains(body, `"tables"`) || !strings.Contains(body, `"selectedTable"`) {
		t.Fatalf("storage updates missing table signal data:\n%s", body)
	}
}

func TestAdminStorageSelectTablePublishesSelectedTablePatch(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner := testPrincipal(t, ctx, store, "owner@example.com", "Owner", access.RoleOwner)
	token := testAPIToken(t, ctx, store, owner.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, DefaultWorkspaceID: "test", DuckDBDir: seedAdminStorageDuckDB(t)})
	updates, unsubscribe := server.broker.Subscribe("admin-storage:test-client")
	defer unsubscribe()

	body := strings.NewReader(`{"adminStorageCommand":{"databaseId":"libredash-test.duckdb","schema":"model","table":"orders"}}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/storage/select-table", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "ld_client_id", Value: "test-client"})
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	select {
	case patch := <-updates:
		storage, ok := patch["adminStorage"].(map[string]any)
		if !ok {
			t.Fatalf("patch missing adminStorage: %#v", patch)
		}
		if storage["selectedKey"] != "libredash-test.duckdb\x00model\x00orders" {
			t.Fatalf("selectedKey = %#v", storage["selectedKey"])
		}
		table, ok := storage["selectedTable"].(*ui.AdminStorageTableSignal)
		if !ok {
			t.Fatalf("selectedTable = %#v, want *ui.AdminStorageTableSignal", storage["selectedTable"])
		}
		if table.Name != "orders" || table.Schema != "model" || len(table.Columns) != 3 {
			t.Fatalf("selectedTable = %#v", table)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for selected table patch")
	}
}

func TestAdminAccessRouteIsDropped(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner := testPrincipal(t, ctx, store, "owner@example.com", "Owner", access.RoleOwner)
	token := testAPIToken(t, ctx, store, owner.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/admin/access", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestAdminPrincipalDetailReturnsNotFoundForMissingPrincipal(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner := testPrincipal(t, ctx, store, "owner@example.com", "Owner", access.RoleOwner)
	token := testAPIToken(t, ctx, store, owner.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/admin/principals/missing", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestAdminGroupDetailReturnsNotFoundForMissingGroup(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	owner := testPrincipal(t, ctx, store, "owner@example.com", "Owner", access.RoleOwner)
	token := testAPIToken(t, ctx, store, owner.ID, "test")
	auth := testAuth(store, "test", AuthConfig{APITokenOnly: true})
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: auth, DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/admin/groups/missing", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestAdminGeneralRendersWithoutStore(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{DefaultWorkspaceID: "test"})
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"General", "RBAC store is not configured", "Test Workspace"} {
		if !strings.Contains(body, want) {
			t.Fatalf("admin general missing %q:\n%s", want, body)
		}
	}
}

func TestAdminStorageRendersEmptyStateWithoutDuckDBFiles(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{DefaultWorkspaceID: "test", DuckDBDir: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/admin/storage", nil)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Storage", "No DuckDB database files found.", "DuckDB directory"} {
		if !strings.Contains(body, want) {
			t.Fatalf("admin storage missing %q:\n%s", want, body)
		}
	}
}

func seedAdminStorageDuckDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "libredash-test.duckdb")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()
	_, err = db.Exec(`
CREATE SCHEMA model;
CREATE TABLE model.orders (
	id INTEGER NOT NULL,
	customer_id VARCHAR,
	amount DOUBLE DEFAULT 0
);
INSERT INTO model.orders VALUES (1, 'c_1', 10.5), (2, 'c_2', 20.5), (3, 'c_3', 30.5);
CREATE VIEW model.order_totals AS SELECT customer_id, amount FROM model.orders;
`)
	if err != nil {
		t.Fatalf("seed duckdb: %v", err)
	}
	return dir
}
