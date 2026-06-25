package app

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/agentapp"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/workspace"
	"github.com/Yacobolo/libredash/pkg/agent"
)

func TestAPIGenAgentToolsExposeTaggedReadOperationsOnly(t *testing.T) {
	server := NewWithOptions(manyRowsMetrics{}, Options{DefaultWorkspaceID: "test"})
	tools := server.agentAPIGenToolDefinitions(agentapp.Scope{WorkspaceID: "test", PrincipalID: "principal"})
	names := map[string]agent.ToolDefinition{}
	for _, tool := range tools {
		names[tool.Name] = tool
	}

	for _, want := range []string{
		"describe_dashboard",
		"describe_dashboard_visual",
		"describe_model",
		"get_deployment",
		"get_materialization_run",
		"list_dashboard_components",
		"list_dashboard_filter_options",
		"list_dashboards",
		"list_deployments",
		"list_materialization_runs",
		"list_semantic_datasets",
		"list_semantic_fields",
		"list_semantic_models",
		"list_workspace_asset_edges",
		"list_workspace_assets",
		"list_workspaces",
		"preview_semantic_dataset",
		"query_dashboard_page",
		"query_dashboard_table_data",
		"query_dashboard_visual_data",
		"query_semantic_dataset",
		"search_workspace",
		"query_table",
		"explain_semantic_preview",
		"explain_semantic_query",
	} {
		if _, ok := names[want]; !ok {
			t.Fatalf("missing APIGen agent tool %q in %#v", want, toolNames(tools))
		}
	}
	for _, forbidden := range []string{
		"activate_deployment",
		"create_agent_turn",
		"create_deployment",
		"create_role_binding",
		"revoke_current_api_token",
		"upload_deployment_artifact",
	} {
		if _, ok := names[forbidden]; ok {
			t.Fatalf("risky operation exposed as agent tool %q", forbidden)
		}
	}

	var schema struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	}
	if err := json.Unmarshal(names["list_workspace_assets"].InputSchema, &schema); err != nil {
		t.Fatalf("decode list_workspace_assets schema: %v", err)
	}
	if _, ok := schema.Properties["workspace"]; ok {
		t.Fatalf("workspace should be injected from agent scope, not model arguments: %s", names["list_workspace_assets"].InputSchema)
	}
	for _, want := range []string{"type", "q", "limit", "pageToken"} {
		if _, ok := schema.Properties[want]; !ok {
			t.Fatalf("schema missing query parameter %q: %s", want, names["list_workspace_assets"].InputSchema)
		}
	}
	if err := json.Unmarshal(names["search_workspace"].InputSchema, &schema); err != nil {
		t.Fatalf("decode search_workspace schema: %v", err)
	}
	if _, ok := schema.Properties["workspace"]; ok {
		t.Fatalf("workspace should be injected from agent scope, not model arguments: %s", names["search_workspace"].InputSchema)
	}
	for _, want := range []string{"q", "types", "limit", "pageToken"} {
		if _, ok := schema.Properties[want]; !ok {
			t.Fatalf("search_workspace schema missing query parameter %q: %s", want, names["search_workspace"].InputSchema)
		}
	}
}

func TestAPIGenAgentSearchToolInjectsDefaultLimit(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{DefaultWorkspaceID: "test"})
	tools := server.agentAPIGenToolDefinitions(agentapp.Scope{WorkspaceID: "test", PrincipalID: "principal"})
	var search agent.ToolDefinition
	for _, tool := range tools {
		if tool.Name == "search_workspace" {
			search = tool
			break
		}
	}
	if search.Handler == nil {
		t.Fatal("search_workspace tool missing")
	}
	result, err := search.Handler.Run(context.Background(), agent.ToolCall{
		ID:        "call_1",
		Name:      "search_workspace",
		Arguments: json.RawMessage(`{"q":"orders"}`),
	})
	if err != nil {
		t.Fatalf("run tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %#v", result.Content)
	}
	body, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatalf("marshal result content: %v", err)
	}
	var decoded struct {
		Items []map[string]any `json:"items"`
		Page  struct {
			NextCursor string `json:"nextCursor"`
		} `json:"page"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode result: %v body=%s", err, body)
	}
	if len(decoded.Items) == 0 || len(decoded.Items) > 10 {
		t.Fatalf("search result count = %d, want 1..10: %#v", len(decoded.Items), decoded.Items)
	}
	for _, item := range decoded.Items {
		if _, ok := item["name"]; !ok {
			t.Fatalf("search item missing concise name: %#v", item)
		}
		if _, ok := item["description"]; !ok {
			t.Fatalf("search item missing concise description: %#v", item)
		}
		if _, ok := item["type"]; !ok {
			t.Fatalf("search item missing concise type: %#v", item)
		}
	}
}

func TestAPIGenAgentToolsExposeTypeSpecArgumentNamesAndBodyFields(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{DefaultWorkspaceID: "test"})
	tools := server.agentAPIGenToolDefinitions(agentapp.Scope{WorkspaceID: "test", PrincipalID: "principal"})
	names := map[string]agent.ToolDefinition{}
	for _, tool := range tools {
		names[tool.Name] = tool
	}
	for toolName, wantProps := range map[string][]string{
		"describe_dashboard":            {"dashboard"},
		"describe_dashboard_visual":     {"dashboard", "page", "visual"},
		"describe_model":                {"model"},
		"list_dashboard_components":     {"dashboard", "page", "limit", "pageToken"},
		"list_dashboard_filter_options": {"dashboard", "page", "filter", "filters", "limit", "pageToken"},
		"query_dashboard_page":          {"dashboard", "page", "filters"},
		"query_dashboard_table_data":    {"dashboard", "page", "table", "count", "filters"},
		"query_dashboard_visual_data":   {"dashboard", "page", "visual", "filters"},
		"query_semantic_dataset":        {"model", "dataset", "dimensions", "measures", "filters", "sort", "limit", "pageToken"},
		"query_table":                   {"dashboard", "table", "count", "filters", "pageId"},
	} {
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal(names[toolName].InputSchema, &schema); err != nil {
			t.Fatalf("decode %s schema: %v", toolName, err)
		}
		for _, want := range wantProps {
			if _, ok := schema.Properties[want]; !ok {
				t.Fatalf("%s schema missing %q: %s", toolName, want, names[toolName].InputSchema)
			}
		}
		for _, forbidden := range []string{"dashboard_id", "model_id", "page_id", "table_id"} {
			if _, ok := schema.Properties[forbidden]; ok {
				t.Fatalf("%s schema exposes rewritten arg %q: %s", toolName, forbidden, names[toolName].InputSchema)
			}
		}
	}
}

func TestAPIGenAgentToolDispatchesThroughGeneratedOperation(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{DefaultWorkspaceID: "test"})
	tools := server.agentAPIGenToolDefinitions(agentapp.Scope{WorkspaceID: "test", PrincipalID: "principal"})
	var listAssets agent.ToolDefinition
	for _, tool := range tools {
		if tool.Name == "list_workspace_assets" {
			listAssets = tool
			break
		}
	}
	if listAssets.Handler == nil {
		t.Fatal("list_workspace_assets tool missing")
	}

	result, err := listAssets.Handler.Run(context.Background(), agent.ToolCall{
		ID:        "call_1",
		Name:      "list_workspace_assets",
		Arguments: json.RawMessage(`{"type":"dashboard","limit":1}`),
	})
	if err != nil {
		t.Fatalf("run tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %#v", result.Content)
	}
	body, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatalf("marshal result content: %v", err)
	}
	var decoded struct {
		Items []struct {
			ID          string `json:"id"`
			WorkspaceID string `json:"workspaceId"`
			Type        string `json:"type"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode result: %v\n%s", err, body)
	}
	if len(decoded.Items) != 1 || decoded.Items[0].ID != "dashboard:executive-sales" || decoded.Items[0].WorkspaceID != "test" || decoded.Items[0].Type != "dashboard" {
		t.Fatalf("tool result = %#v", decoded.Items)
	}
}

func TestAPIGenAgentListToolInjectsDefaultLimit(t *testing.T) {
	server := NewWithOptions(manyEdgesMetrics{}, Options{DefaultWorkspaceID: "test"})
	tools := server.agentAPIGenToolDefinitions(agentapp.Scope{WorkspaceID: "test", PrincipalID: "principal"})
	var listEdges agent.ToolDefinition
	for _, tool := range tools {
		if tool.Name == "list_workspace_asset_edges" {
			listEdges = tool
			break
		}
	}
	if listEdges.Handler == nil {
		t.Fatal("list_workspace_asset_edges tool missing")
	}
	result, err := listEdges.Handler.Run(context.Background(), agent.ToolCall{
		ID:        "call_1",
		Name:      "list_workspace_asset_edges",
		Arguments: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("run tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %#v", result.Content)
	}
	body, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatalf("marshal result content: %v", err)
	}
	var decoded struct {
		Items []map[string]any `json:"items"`
		Page  struct {
			NextCursor string `json:"nextCursor"`
		} `json:"page"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode result: %v body=%s", err, body)
	}
	if len(decoded.Items) != 25 || decoded.Page.NextCursor == "" {
		t.Fatalf("default-limited edge result = count %d cursor %q", len(decoded.Items), decoded.Page.NextCursor)
	}

	result, err = listEdges.Handler.Run(context.Background(), agent.ToolCall{
		ID:        "call_2",
		Name:      "list_workspace_asset_edges",
		Arguments: json.RawMessage(`{"limit":3}`),
	})
	if err != nil {
		t.Fatalf("run explicit limit tool: %v", err)
	}
	body, _ = json.Marshal(result.Content)
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode explicit result: %v body=%s", err, body)
	}
	if len(decoded.Items) != 3 {
		t.Fatalf("explicit-limited edge count = %d, want 3", len(decoded.Items))
	}
}

func TestAPIGenAgentToolDispatchesJSONBodyOperation(t *testing.T) {
	server := NewWithOptions(manyRowsMetrics{}, Options{DefaultWorkspaceID: "test"})
	tools := server.agentAPIGenToolDefinitions(agentapp.Scope{WorkspaceID: "test", PrincipalID: "principal"})
	var queryTable agent.ToolDefinition
	for _, tool := range tools {
		if tool.Name == "query_table" {
			queryTable = tool
			break
		}
	}
	if queryTable.Handler == nil {
		t.Fatal("query_table tool missing")
	}
	result, err := queryTable.Handler.Run(context.Background(), agent.ToolCall{
		ID:        "call_1",
		Name:      "query_table",
		Arguments: json.RawMessage(`{"dashboard":"executive-sales","pageId":"overview","table":"orders","count":500}`),
	})
	if err != nil {
		t.Fatalf("run tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %#v", result.Content)
	}
	body, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatalf("marshal result content: %v", err)
	}
	var table struct {
		AvailableRows int `json:"availableRows"`
		Blocks        map[string]struct {
			Rows []map[string]any `json:"rows"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(body, &table); err != nil {
		t.Fatalf("decode table result: %v\n%s", err, body)
	}
	if table.AvailableRows != 50 || len(table.Blocks["a"].Rows) != 50 {
		t.Fatalf("table result was not capped to 50: %#v", table)
	}
}

func TestAPIGenAgentToolFetchesSingleDashboardVisualData(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{DefaultWorkspaceID: "test"})
	tools := server.agentAPIGenToolDefinitions(agentapp.Scope{WorkspaceID: "test", PrincipalID: "principal"})
	var queryVisual agent.ToolDefinition
	for _, tool := range tools {
		if tool.Name == "query_dashboard_visual_data" {
			queryVisual = tool
			break
		}
	}
	if queryVisual.Handler == nil {
		t.Fatal("query_dashboard_visual_data tool missing")
	}
	result, err := queryVisual.Handler.Run(context.Background(), agent.ToolCall{
		ID:        "call_1",
		Name:      "query_dashboard_visual_data",
		Arguments: json.RawMessage(`{"dashboard":"executive-sales","page":"overview","visual":"orders"}`),
	})
	if err != nil {
		t.Fatalf("run tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %#v", result.Content)
	}
	body, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatalf("marshal result content: %v", err)
	}
	var visual struct {
		Title string           `json:"title"`
		Data  []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &visual); err != nil {
		t.Fatalf("decode visual result: %v body=%s", err, body)
	}
	if visual.Title != "Orders" || len(visual.Data) != 1 {
		t.Fatalf("visual result = %#v", visual)
	}
}

func TestAPIGenAgentSemanticQueryToolInjectsBodyDefaultLimit(t *testing.T) {
	server := NewWithOptions(manySemanticRowsMetrics{}, Options{DefaultWorkspaceID: "test"})
	tools := server.agentAPIGenToolDefinitions(agentapp.Scope{WorkspaceID: "test", PrincipalID: "principal"})
	var querySemantic agent.ToolDefinition
	for _, tool := range tools {
		if tool.Name == "query_semantic_dataset" {
			querySemantic = tool
			break
		}
	}
	if querySemantic.Handler == nil {
		t.Fatal("query_semantic_dataset tool missing")
	}
	result, err := querySemantic.Handler.Run(context.Background(), agent.ToolCall{
		ID:        "call_1",
		Name:      "query_semantic_dataset",
		Arguments: json.RawMessage(`{"model":"test","dataset":"orders","dimensions":[{"field":"orders.status","alias":"status"}],"measures":[{"field":"order_count"}],"sort":[{"field":"status","direction":"asc"}]}`),
	})
	if err != nil {
		t.Fatalf("run tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %#v", result.Content)
	}
	body, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatalf("marshal result content: %v", err)
	}
	var decoded struct {
		Items []map[string]any `json:"items"`
		Page  struct {
			NextCursor string `json:"nextCursor"`
		} `json:"page"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode semantic result: %v body=%s", err, body)
	}
	if len(decoded.Items) != 25 || decoded.Page.NextCursor == "" {
		t.Fatalf("semantic default-limited result = %#v", decoded)
	}
}

func TestAPIGenAgentToolEnforcesCredentialPermissionAllowlistAndWorkspace(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	principal := testPrincipal(t, ctx, store, "agent-token@example.com", "Agent Token", access.RoleOwner)
	agentOnlyToken := access.APIToken{WorkspaceID: "test", Permissions: []string{access.PermissionAgentUse}}
	assetToken := access.APIToken{WorkspaceID: "test", Permissions: []string{access.PermissionAgentUse, access.PermissionAssetRead}}
	foreignToken := access.APIToken{WorkspaceID: "other", Permissions: []string{access.PermissionAssetRead}}
	server := NewWithOptions(fakeMetrics{}, Options{Store: store, AccessRepo: testAccessRepository(store), DefaultWorkspaceID: "test"})

	run := func(token access.APIToken) agent.ToolResult {
		scope := agentapp.Scope{
			WorkspaceID: "test",
			PrincipalID: principal.ID,
			Credential: agentapp.CredentialScope{
				WorkspaceID: token.WorkspaceID,
				Permissions: append([]string(nil), token.Permissions...),
				Restricted:  token.Permissions != nil,
			},
		}
		tools := server.agentAPIGenToolDefinitions(scope)
		for _, tool := range tools {
			if tool.Name == "list_dashboards" {
				result, err := tool.Handler.Run(ctx, agent.ToolCall{ID: "call_1", Name: "list_dashboards", Arguments: json.RawMessage(`{}`)})
				if err != nil {
					t.Fatalf("run list_dashboards: %v", err)
				}
				return result
			}
		}
		t.Fatal("list_dashboards tool missing")
		return agent.ToolResult{}
	}

	if result := run(agentOnlyToken); !result.IsError {
		t.Fatalf("agent-only token unexpectedly called asset tool: %#v", result.Content)
	}
	if result := run(foreignToken); !result.IsError {
		t.Fatalf("foreign workspace token unexpectedly called asset tool: %#v", result.Content)
	}
	if result := run(assetToken); result.IsError {
		t.Fatalf("asset token was rejected: %#v", result.Content)
	}
}

func toolNames(tools []agent.ToolDefinition) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

type manyEdgesMetrics struct {
	fakeMetrics
}

type manySemanticRowsMetrics struct {
	fakeMetrics
}

func (manySemanticRowsMetrics) QuerySemantic(_ context.Context, _ string, request reportdef.AggregateQuery) (reportdef.QueryRows, error) {
	rows := make(reportdef.QueryRows, 0, request.Limit)
	for i := 0; i < request.Limit; i++ {
		rows = append(rows, reportdef.QueryRow{"status": "s" + strconv.Itoa(i), "order_count": i})
	}
	return rows, nil
}

func (manyEdgesMetrics) WorkspaceAssets(workspaceID, deploymentID string) ([]workspace.Asset, []workspace.AssetEdge, bool) {
	root, err := workspace.NewAsset(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), workspace.AssetTypeCatalog, "catalog", "", "Catalog", "", map[string]any{})
	if err != nil {
		return nil, nil, false
	}
	assets := []workspace.Asset{root}
	edges := make([]workspace.AssetEdge, 0, 30)
	for i := 0; i < 30; i++ {
		key := "dashboard-" + strconv.Itoa(i)
		child, err := workspace.NewAsset(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), workspace.AssetTypeDashboard, key, root.ID, "Dashboard", "", map[string]any{"index": i})
		if err != nil {
			return nil, nil, false
		}
		assets = append(assets, child)
		edges = append(edges, workspace.NewAssetEdge(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), root.ID, child.ID, workspace.AssetEdgeContains))
	}
	return assets, edges, true
}
