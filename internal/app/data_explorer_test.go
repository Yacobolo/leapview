package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	analyticsmaterialize "github.com/Yacobolo/libredash/internal/analytics/materialize"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	uisignals "github.com/Yacobolo/libredash/internal/ui/signals"
	"github.com/Yacobolo/libredash/internal/workspace"
	_ "github.com/duckdb/duckdb-go/v2"
)

type dataExplorerFixtureMetrics struct {
	fakeMetrics
	dataDir              string
	semanticPreviewError error
	semanticRequests     *[]reportdef.RowQuery
}

func (m dataExplorerFixtureMetrics) DataDir() string {
	return m.dataDir
}

func (m dataExplorerFixtureMetrics) SemanticModel(modelID string) (*semanticmodel.Model, bool) {
	if modelID != "olist" {
		return m.fakeMetrics.SemanticModel(modelID)
	}
	return &semanticmodel.Model{
		Name:              "olist",
		Title:             "Olist",
		DefaultConnection: "local",
		Connections: map[string]semanticmodel.Connection{
			"local": {Kind: "local"},
		},
		Sources: map[string]semanticmodel.Source{
			"orders": {
				Path:       "orders.csv",
				Format:     "csv",
				Connection: "local",
				Schema: semanticmodel.TableSchema{Columns: []semanticmodel.ColumnSchema{
					{Name: "order_id", PhysicalType: "VARCHAR", Ordinal: 1},
					{Name: "status", PhysicalType: "VARCHAR", Ordinal: 2},
				}},
			},
		},
		BaseTable: "orders",
		Tables: map[string]semanticmodel.Table{
			"orders": {
				Kind:       "fact",
				Source:     "orders",
				PrimaryKey: "order_id",
				Columns: map[string]semanticmodel.ModelColumn{
					"order_id": {Name: "order_id", Type: "VARCHAR"},
					"status":   {Name: "status", Type: "VARCHAR"},
				},
				Dimensions: map[string]semanticmodel.MetricDimension{
					"order_id": {Expr: "order_id", Label: "Order ID", Type: "string"},
					"status":   {Expr: "status", Label: "Status", Type: "string"},
				},
				Schema: semanticmodel.TableSchema{Columns: []semanticmodel.ColumnSchema{
					{Name: "order_id", PhysicalType: "VARCHAR", Ordinal: 1},
					{Name: "status", PhysicalType: "VARCHAR", Ordinal: 2},
				}},
			},
		},
		Measures: map[string]semanticmodel.MetricMeasure{
			"order_count": {Table: "orders", Expression: "COUNT(*)", Label: "Orders"},
		},
	}, true
}

func (m dataExplorerFixtureMetrics) PreviewSemantic(_ context.Context, modelID string, request reportdef.RowQuery) (reportdef.QueryRows, error) {
	if m.semanticRequests != nil {
		*m.semanticRequests = append(*m.semanticRequests, request)
	}
	if m.semanticPreviewError != nil {
		return nil, m.semanticPreviewError
	}
	if modelID != "olist" || request.Table != "orders" {
		return nil, nil
	}
	return reportdef.QueryRows{{"order_id": "o1", "status": "delivered"}}, nil
}

func (m dataExplorerFixtureMetrics) WorkspaceAssets(workspaceID, deploymentID string) ([]workspace.Asset, []workspace.AssetEdge, bool) {
	catalog, err := testWorkspaceAsset(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), workspace.AssetTypeCatalog, workspaceID, "", "Catalog", "", "catalog.v1", map[string]any{})
	if err != nil {
		return nil, nil, false
	}
	connection, err := testWorkspaceAsset(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), workspace.AssetTypeConnection, "olist.local", catalog.ID, "local", "", "connection.v1", map[string]any{"Kind": "local"})
	if err != nil {
		return nil, nil, false
	}
	source, err := testWorkspaceAsset(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), workspace.AssetTypeSource, "olist.orders", catalog.ID, "orders source", "", "source.v1", map[string]any{"Connection": "local", "Format": "csv", "Path": "orders.csv"})
	if err != nil {
		return nil, nil, false
	}
	model, err := testWorkspaceAsset(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), workspace.AssetTypeSemanticModel, "olist", catalog.ID, "Olist", "", "semantic_model.v1", map[string]any{})
	if err != nil {
		return nil, nil, false
	}
	table, err := testWorkspaceAsset(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), workspace.AssetTypeModelTable, "olist.orders", model.ID, "orders", "", "model_table.v1", map[string]any{"Source": "orders"})
	if err != nil {
		return nil, nil, false
	}
	return []workspace.Asset{catalog, connection, source, model, table}, []workspace.AssetEdge{
		workspace.NewAssetEdge(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), catalog.ID, connection.ID, workspace.AssetEdgeContains),
		workspace.NewAssetEdge(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), catalog.ID, source.ID, workspace.AssetEdgeContains),
		workspace.NewAssetEdge(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), catalog.ID, model.ID, workspace.AssetEdgeContains),
		workspace.NewAssetEdge(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), model.ID, table.ID, workspace.AssetEdgeContains),
		workspace.NewAssetEdge(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), source.ID, connection.ID, workspace.AssetEdgeUsesConnection),
		workspace.NewAssetEdge(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), table.ID, source.ID, workspace.AssetEdgeReadsSource),
	}, true
}

func TestDataExplorerRouteRendersSignalsAndWiring(t *testing.T) {
	store := testStore(t)
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	metrics := dataExplorerFixtureMetrics{dataDir: seedDataExplorerCSV(t)}
	seedActiveDeploymentFromWorkspaceAssets(t, store, "test", metrics)
	server := NewWithOptions(metrics, Options{Store: store, Auth: auth, DefaultWorkspaceID: "test", DuckDBDir: seedDataExplorerDuckDB(t)})

	req := httptest.NewRequest(http.MethodGet, "/data?workspace=test&object=model_table:model_table:olist.orders", nil)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<ld-data-explorer",
		"/static/data-explorer.js",
		"dataExplorer",
		`"csrfToken":"`,
		"/data/updates",
		"/data/command",
		"X-CSRF-Token",
		"workspaceId",
		"model_table:model_table:olist.orders",
		"semantic_view:olist.orders",
		`"active":"data"`,
		`"primaryAction":{"label":"New chat","href":"/chat/new","icon":"plus"}`,
		`"history":{"label":"Chats"`,
	} {
		body = html.UnescapeString(body)
		if !strings.Contains(body, want) {
			t.Fatalf("data route missing %q:\n%s", want, body)
		}
	}
}

func TestWorkspaceDataExplorerRouteRedirectsToGlobalRoute(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{DefaultWorkspaceID: "test"})

	req := httptest.NewRequest(http.MethodGet, "/workspaces/test/data?object=source:olist.orders", nil)
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if location != "/data?object=source%3Aolist.orders&workspace=test" {
		t.Fatalf("location = %q", location)
	}
}

func TestGlobalDataExplorerSelectsDuplicateKeysByWorkspace(t *testing.T) {
	store := testStore(t)
	metrics := dataExplorerFixtureMetrics{dataDir: seedDataExplorerCSV(t)}
	seedActiveDeploymentFromWorkspaceAssets(t, store, "test", metrics)
	seedActiveDeploymentFromWorkspaceAssets(t, store, "ops", metrics)
	server := NewWithOptions(NewMultiWorkspaceMetrics("test", map[string]QueryMetrics{
		"test": metrics,
		"ops":  metrics,
	}), Options{Store: store, DefaultWorkspaceID: "test", DuckDBDir: seedDataExplorerDuckDB(t)})

	req := httptest.NewRequest(http.MethodGet, "/data?workspace=ops&object=model_table:model_table:olist.orders", nil)
	_, explorer, err := server.globalDataExplorerState(req, dataExplorerCommandFromQuery("ops", "model_table:model_table:olist.orders"))
	if err != nil {
		t.Fatalf("globalDataExplorerState() error = %v", err)
	}
	if explorer.SelectedWorkspaceID != "ops" || explorer.Command.WorkspaceID != "ops" {
		t.Fatalf("selected workspace = %#v command=%#v", explorer.SelectedWorkspaceID, explorer.Command)
	}
	if explorer.SelectedKey != "model_table:model_table:olist.orders" || explorer.SelectedObject == nil || explorer.SelectedObject.WorkspaceID != "ops" {
		t.Fatalf("selected object = %#v key=%q", explorer.SelectedObject, explorer.SelectedKey)
	}
	if len(explorer.Objects) != 6 {
		t.Fatalf("object count = %d, want both workspaces' three objects", len(explorer.Objects))
	}
}

func TestGlobalDataExplorerFallsBackToRuntimeCatalogWithoutActiveAssetDeployment(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{DefaultWorkspaceID: "test-workspace"})
	req := httptest.NewRequest(http.MethodGet, "/data", nil)

	page, explorer, err := server.globalDataExplorerState(req, dataExplorerCommandFromQuery("", ""))
	if err != nil {
		t.Fatalf("globalDataExplorerState() error = %v", err)
	}
	if page.SelectedWorkspaceID != "test-workspace" || explorer.SelectedWorkspaceID != "test-workspace" {
		t.Fatalf("selected workspace page=%q explorer=%q", page.SelectedWorkspaceID, explorer.SelectedWorkspaceID)
	}
	rendered := fmtSprint(explorer)
	for _, want := range []string{"model_table:model_table:test.orders", "semantic_view:test.orders"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("fallback explorer missing %q:\n%#v", want, explorer)
		}
	}
	if strings.Contains(rendered, "assets were not found") {
		t.Fatalf("runtime catalog fallback should not expose missing asset deployment warning:\n%#v", explorer.Warnings)
	}
}

func TestDataExplorerPreviewsSourceModelTableAndSemanticRows(t *testing.T) {
	store := testStore(t)
	dataDir := seedDataExplorerCSV(t)
	duckDBDir := seedDataExplorerDuckDB(t)
	metrics := dataExplorerFixtureMetrics{dataDir: dataDir}
	seedActiveDeploymentFromWorkspaceAssets(t, store, "test", metrics)
	server := NewWithOptions(metrics, Options{Store: store, DefaultWorkspaceID: "test", DuckDBDir: duckDBDir})

	cases := []struct {
		name   string
		object string
		want   string
	}{
		{name: "source", object: "source:source:olist.orders", want: "delivered"},
		{name: "model table", object: "model_table:model_table:olist.orders", want: "shipped"},
		{name: "semantic", object: "semantic_view:olist.orders", want: "Order ID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/data?workspace=test&object="+tc.object, nil)
			_, explorer, err := server.globalDataExplorerState(req, dataExplorerCommandFromQuery("test", tc.object))
			if err != nil {
				t.Fatalf("globalDataExplorerState() error = %v", err)
			}
			if explorer.Preview.Error != "" {
				t.Fatalf("preview error = %q", explorer.Preview.Error)
			}
			if explorer.Preview.ChunkSize != dataExplorerDefaultLimit || explorer.Preview.RowHeight != dataExplorerRowHeight {
				t.Fatalf("preview window defaults = chunk %d rowHeight %d", explorer.Preview.ChunkSize, explorer.Preview.RowHeight)
			}
			if len(explorer.Preview.Blocks) == 0 || len(explorer.Preview.Blocks["a"].Rows) == 0 {
				t.Fatalf("preview did not seed initial block rows:\n%#v", explorer.Preview)
			}
			rendered := fmtSprint(explorer)
			if !strings.Contains(rendered, tc.want) {
				t.Fatalf("preview missing %q:\n%#v", tc.want, explorer.Preview)
			}
			if strings.Contains(rendered, "order_count") {
				t.Fatalf("semantic preview included aggregate measure:\n%#v", explorer.Preview)
			}
		})
	}
}

func TestDataExplorerCommandPublishesPatch(t *testing.T) {
	store := testStore(t)
	metrics := dataExplorerFixtureMetrics{dataDir: seedDataExplorerCSV(t)}
	seedActiveDeploymentFromWorkspaceAssets(t, store, "test", metrics)
	server := NewWithOptions(metrics, Options{Store: store, DefaultWorkspaceID: "test", DuckDBDir: seedDataExplorerDuckDB(t)})
	updates, unsubscribe := server.broker.Subscribe("data-explorer:test-client")
	defer unsubscribe()

	body := strings.NewReader(`{"dataExplorerCommand":{"workspaceId":"test","objectKey":"semantic_view:olist.orders","block":"b","start":100,"count":100,"requestSeq":7,"resetVersion":2,"sort":{"column":"status","direction":"asc"}}}`)
	req := httptest.NewRequest(http.MethodPost, "/data/command", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "ld_client_id", Value: "test-client"})
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	select {
	case patch := <-updates:
		explorer, ok := patch["dataExplorer"].(uisignals.DataExplorerSignal)
		if !ok {
			t.Fatalf("patch missing dataExplorer: %#v", patch)
		}
		if explorer.SelectedWorkspaceID != "test" || explorer.SelectedKey != "semantic_view:olist.orders" || explorer.Preview.Error != "" {
			t.Fatalf("unexpected explorer patch: %#v", explorer)
		}
		block := explorer.Preview.Blocks["b"]
		if block.Start != 100 || block.RequestSeq != 7 || block.ResetVersion != 2 || block.Sort.Column != "status" || block.Sort.Direction != "asc" {
			t.Fatalf("unexpected preview block: %#v", block)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for data explorer patch")
	}
}

func TestDataExplorerSemanticPreviewIgnoresInvalidSortColumn(t *testing.T) {
	requests := []reportdef.RowQuery{}
	store := testStore(t)
	metrics := dataExplorerFixtureMetrics{dataDir: seedDataExplorerCSV(t), semanticRequests: &requests}
	seedActiveDeploymentFromWorkspaceAssets(t, store, "test", metrics)
	server := NewWithOptions(metrics, Options{Store: store, DefaultWorkspaceID: "test", DuckDBDir: seedDataExplorerDuckDB(t)})

	req := httptest.NewRequest(http.MethodGet, "/data?workspace=test&object=semantic_view:olist.orders", nil)
	command := dataExplorerCommandFromQuery("test", "semantic_view:olist.orders")
	command.Sort = uisignals.DataPreviewSortSignal{Column: "order_count", Direction: "desc"}
	_, explorer, err := server.globalDataExplorerState(req, command)
	if err != nil {
		t.Fatalf("globalDataExplorerState() error = %v", err)
	}
	if explorer.Preview.Error != "" {
		t.Fatalf("preview error = %q", explorer.Preview.Error)
	}
	if len(requests) == 0 {
		t.Fatal("semantic preview was not requested")
	}
	for _, sort := range requests[len(requests)-1].Sort {
		if sort.Field == "order_count" {
			t.Fatalf("invalid semantic sort was forwarded to planner: %#v", requests[len(requests)-1].Sort)
		}
	}
}

func TestDataExplorerSemanticPreviewAcceptsExposedSortColumn(t *testing.T) {
	requests := []reportdef.RowQuery{}
	store := testStore(t)
	metrics := dataExplorerFixtureMetrics{dataDir: seedDataExplorerCSV(t), semanticRequests: &requests}
	seedActiveDeploymentFromWorkspaceAssets(t, store, "test", metrics)
	server := NewWithOptions(metrics, Options{Store: store, DefaultWorkspaceID: "test", DuckDBDir: seedDataExplorerDuckDB(t)})

	req := httptest.NewRequest(http.MethodGet, "/data?workspace=test&object=semantic_view:olist.orders", nil)
	command := dataExplorerCommandFromQuery("test", "semantic_view:olist.orders")
	command.Sort = uisignals.DataPreviewSortSignal{Column: "status", Direction: "asc"}
	_, explorer, err := server.globalDataExplorerState(req, command)
	if err != nil {
		t.Fatalf("globalDataExplorerState() error = %v", err)
	}
	if explorer.Preview.Error != "" {
		t.Fatalf("preview error = %q", explorer.Preview.Error)
	}
	if len(requests) == 0 || len(requests[len(requests)-1].Sort) != 1 {
		t.Fatalf("semantic preview did not receive valid sort: %#v", requests)
	}
	if got := requests[len(requests)-1].Sort[0]; got.Field != "status" || got.Direction != "asc" {
		t.Fatalf("semantic sort = %#v", got)
	}
}

func TestDataExplorerCommandReusesPostedPreviewTotalsForScroll(t *testing.T) {
	store := testStore(t)
	metrics := dataExplorerFixtureMetrics{dataDir: seedDataExplorerCSV(t)}
	seedActiveDeploymentFromWorkspaceAssets(t, store, "test", metrics)
	server := NewWithOptions(metrics, Options{Store: store, DefaultWorkspaceID: "test", DuckDBDir: seedDataExplorerDuckDB(t)})
	updates, unsubscribe := server.broker.Subscribe("data-explorer:test-client")
	defer unsubscribe()

	object := uisignals.DataExplorerObjectSignal{
		Key:         "semantic_view:olist.orders",
		WorkspaceID: "test",
		Layer:       "semantic_view",
		ModelID:     "olist",
		Table:       "orders",
		Title:       "orders semantic view",
		Columns: []uisignals.DataPreviewColumnSignal{
			{Key: "order_id", Label: "Order ID", Type: "string"},
			{Key: "status", Label: "Status", Type: "string"},
		},
	}
	currentCommand := uisignals.DataExplorerCommand{
		WorkspaceID:  "test",
		ObjectKey:    object.Key,
		Block:        "a",
		Start:        0,
		Limit:        100,
		Count:        100,
		RequestSeq:   1,
		ResetVersion: 3,
		Sort:         uisignals.DataPreviewSortSignal{Column: "status", Direction: "asc"},
	}
	currentExplorer := uisignals.DataExplorerSignal{
		Objects:             []uisignals.DataExplorerObjectSignal{object},
		SelectedWorkspaceID: "test",
		SelectedKey:         object.Key,
		SelectedObject:      &object,
		Preview: uisignals.DataPreviewSignal{
			Columns:       object.Columns,
			TotalRows:     250,
			AvailableRows: 250,
			ChunkSize:     100,
			RowHeight:     dataExplorerRowHeight,
			ResetVersion:  3,
			Blocks:        emptyDataPreviewBlocks(100, currentCommand.Sort, 3),
			TotalRowLabel: "250",
			Sort:          currentCommand.Sort,
		},
		Command: currentCommand,
	}
	nextCommand := currentCommand
	nextCommand.Block = "b"
	nextCommand.Start = 100
	nextCommand.Offset = 100
	nextCommand.RequestSeq = 2
	bodyBytes, err := json.Marshal(map[string]any{
		"dataExplorer":        currentExplorer,
		"dataExplorerCommand": nextCommand,
	})
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/data/command", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "ld_client_id", Value: "test-client"})
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	select {
	case patch := <-updates:
		explorer, ok := patch["dataExplorer"].(uisignals.DataExplorerSignal)
		if !ok {
			t.Fatalf("patch missing dataExplorer: %#v", patch)
		}
		if explorer.Preview.Error != "" {
			t.Fatalf("scroll command unexpectedly counted preview: %#v", explorer.Preview)
		}
		if explorer.Preview.TotalRows != 250 || explorer.Preview.AvailableRows != 250 || explorer.Preview.TotalRowLabel != "250" {
			t.Fatalf("preview totals were not reused: %#v", explorer.Preview)
		}
		if block := explorer.Preview.Blocks["b"]; block.Start != 100 || block.RequestSeq != 2 {
			t.Fatalf("requested scroll block missing: %#v", block)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for data explorer patch")
	}
}

func TestDataExplorerCommandDoesNotPublishCanceledPreview(t *testing.T) {
	store := testStore(t)
	metrics := dataExplorerFixtureMetrics{dataDir: seedDataExplorerCSV(t), semanticPreviewError: context.Canceled}
	seedActiveDeploymentFromWorkspaceAssets(t, store, "test", metrics)
	server := NewWithOptions(metrics, Options{Store: store, DefaultWorkspaceID: "test", DuckDBDir: seedDataExplorerDuckDB(t)})
	updates, unsubscribe := server.broker.Subscribe("data-explorer:test-client")
	defer unsubscribe()

	body := strings.NewReader(`{"dataExplorerCommand":{"workspaceId":"test","objectKey":"semantic_view:olist.orders","block":"b","start":100,"count":100,"requestSeq":7,"resetVersion":2}}`)
	req := httptest.NewRequest(http.MethodPost, "/data/command", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "ld_client_id", Value: "test-client"})
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	select {
	case patch := <-updates:
		t.Fatalf("received canceled data explorer patch: %#v", patch)
	default:
	}
}

func TestDataExplorerCommandColumnWidthsReuseCurrentPreview(t *testing.T) {
	store := testStore(t)
	metrics := dataExplorerFixtureMetrics{dataDir: seedDataExplorerCSV(t), semanticPreviewError: errors.New("preview should not run for column widths")}
	seedActiveDeploymentFromWorkspaceAssets(t, store, "test", metrics)
	server := NewWithOptions(metrics, Options{Store: store, DefaultWorkspaceID: "test", DuckDBDir: seedDataExplorerDuckDB(t)})
	updates, unsubscribe := server.broker.Subscribe("data-explorer:test-client")
	defer unsubscribe()

	object := uisignals.DataExplorerObjectSignal{
		Key:         "semantic_view:olist.orders",
		WorkspaceID: "test",
		Layer:       "semantic_view",
		ModelID:     "olist",
		Table:       "orders",
		Title:       "orders semantic view",
		Columns: []uisignals.DataPreviewColumnSignal{
			{Key: "order_id", Label: "Order ID", Type: "string"},
			{Key: "status", Label: "Status", Type: "string"},
		},
	}
	currentCommand := uisignals.DataExplorerCommand{WorkspaceID: "test", ObjectKey: object.Key, Block: "all", Limit: 100, Count: 100, Sort: uisignals.DataPreviewSortSignal{}, VisibleColumns: []string{}}
	currentExplorer := uisignals.DataExplorerSignal{
		Objects:             []uisignals.DataExplorerObjectSignal{object},
		SelectedWorkspaceID: "test",
		SelectedKey:         object.Key,
		SelectedObject:      &object,
		Preview: uisignals.DataPreviewSignal{
			Columns:       object.Columns,
			TotalRows:     1,
			AvailableRows: 1,
			ChunkSize:     100,
			RowHeight:     dataExplorerRowHeight,
			Blocks: map[string]uisignals.DataPreviewBlockSignal{
				"a": {Start: 0, Rows: []map[string]any{{"order_id": "o1", "status": "delivered"}}},
			},
			TotalRowLabel: "1",
		},
		Command: currentCommand,
	}
	nextCommand := currentCommand
	nextCommand.ColumnWidths = map[string]float64{"order_id": 260}
	bodyBytes, err := json.Marshal(map[string]any{
		"dataExplorer":        currentExplorer,
		"dataExplorerCommand": nextCommand,
	})
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/data/command", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "ld_client_id", Value: "test-client"})
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	select {
	case patch := <-updates:
		explorer, ok := patch["dataExplorer"].(uisignals.DataExplorerSignal)
		if !ok {
			t.Fatalf("patch missing dataExplorer: %#v", patch)
		}
		if explorer.Preview.Error != "" {
			t.Fatalf("resize-only command re-ran preview: %#v", explorer.Preview)
		}
		if got := explorer.Command.ColumnWidths["order_id"]; got != 260 {
			t.Fatalf("column width was not patched: %#v", explorer.Command.ColumnWidths)
		}
		if len(explorer.Preview.Blocks["a"].Rows) != 1 {
			t.Fatalf("current preview rows were not preserved: %#v", explorer.Preview.Blocks)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for data explorer patch")
	}
}

func TestDataExplorerBrowserCommandRequiresAndAcceptsCSRF(t *testing.T) {
	store := testStore(t)
	auth := testAuth(store, "test", AuthConfig{DevBypass: true})
	metrics := dataExplorerFixtureMetrics{dataDir: seedDataExplorerCSV(t)}
	seedActiveDeploymentFromWorkspaceAssets(t, store, "test", metrics)
	server := NewWithOptions(metrics, Options{Store: store, Auth: auth, DefaultWorkspaceID: "test", DuckDBDir: seedDataExplorerDuckDB(t)})
	updates, unsubscribe := server.broker.Subscribe("data-explorer:test-client")
	defer unsubscribe()

	commandBody := `{"dataExplorerCommand":{"workspaceId":"test","objectKey":"semantic_view:olist.orders","block":"b","start":100,"count":100,"requestSeq":7,"resetVersion":2,"sort":{"column":"status","direction":"asc"}}}`
	forbiddenReq := httptest.NewRequest(http.MethodPost, "http://localhost:8150/data/command", strings.NewReader(commandBody))
	forbiddenReq.Header.Set("Content-Type", "application/json")
	forbiddenReq.Header.Set("Accept", "application/json")
	forbiddenReq.Header.Set("Referer", "http://localhost:8150/data?workspace=test")
	forbiddenReq.AddCookie(&http.Cookie{Name: "ld_client_id", Value: "test-client"})
	forbiddenRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(forbiddenRec, forbiddenReq)
	if forbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF status = %d, want %d body=%s", forbiddenRec.Code, http.StatusForbidden, forbiddenRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "http://localhost:8150/data?workspace=test&object=semantic_view:olist.orders", nil)
	getRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", getRec.Code, getRec.Body.String())
	}
	token := dataExplorerCSRFToken(t, getRec.Body.String())

	allowedReq := httptest.NewRequest(http.MethodPost, "http://localhost:8150/data/command", strings.NewReader(commandBody))
	allowedReq.Header.Set("Content-Type", "application/json")
	allowedReq.Header.Set("Accept", "application/json")
	allowedReq.Header.Set("X-CSRF-Token", token)
	allowedReq.Header.Set("Referer", "http://localhost:8150/data?workspace=test")
	allowedReq.AddCookie(&http.Cookie{Name: "ld_client_id", Value: "test-client"})
	for _, cookie := range getRec.Result().Cookies() {
		allowedReq.AddCookie(cookie)
	}
	allowedRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(allowedRec, allowedReq)
	if allowedRec.Code != http.StatusNoContent {
		t.Fatalf("POST with CSRF status = %d, want %d body=%s", allowedRec.Code, http.StatusNoContent, allowedRec.Body.String())
	}

	select {
	case patch := <-updates:
		explorer, ok := patch["dataExplorer"].(uisignals.DataExplorerSignal)
		if !ok {
			t.Fatalf("patch missing dataExplorer: %#v", patch)
		}
		block := explorer.Preview.Blocks["b"]
		if block.Start != 100 || block.RequestSeq != 7 || block.ResetVersion != 2 || block.Sort.Column != "status" || block.Sort.Direction != "asc" {
			t.Fatalf("unexpected preview block: %#v", block)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for data explorer patch")
	}
}

func dataExplorerCSRFToken(t *testing.T, body string) string {
	t.Helper()
	matches := regexp.MustCompile(`"csrfToken":"([^"]+)"`).FindStringSubmatch(html.UnescapeString(body))
	if len(matches) != 2 || strings.TrimSpace(matches[1]) == "" {
		t.Fatalf("data route did not render csrfToken signal:\n%s", body)
	}
	return matches[1]
}

func seedDataExplorerCSV(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "orders.csv"), []byte("order_id,status\no1,delivered\no2,shipped\n"), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	return dir
}

func seedDataExplorerDuckDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("duckdb", analyticsmaterialize.DatabasePath(dir, "olist"))
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE SCHEMA model"); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE model.orders(order_id VARCHAR, status VARCHAR)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO model.orders VALUES ('o1', 'delivered'), ('o2', 'shipped')"); err != nil {
		t.Fatalf("insert rows: %v", err)
	}
	return dir
}

func fmtSprint(value any) string {
	return strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%#v", value), "\n", " "), "\t", " ")
}
