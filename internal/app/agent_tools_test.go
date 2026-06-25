package app

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Yacobolo/libredash/internal/agentapp"
	"github.com/Yacobolo/libredash/pkg/agent"
)

func TestAPIGenAgentToolsExposeTaggedReadOperationsOnly(t *testing.T) {
	server := NewWithOptions(fakeMetrics{}, Options{DefaultWorkspaceID: "test"})
	tools := server.agentAPIGenToolDefinitions(agentapp.Scope{WorkspaceID: "test", PrincipalID: "principal"})
	names := map[string]agent.ToolDefinition{}
	for _, tool := range tools {
		names[tool.Name] = tool
	}

	for _, want := range []string{
		"get_deployment",
		"get_materialization_run",
		"list_deployments",
		"list_materialization_runs",
		"list_workspace_asset_edges",
		"list_workspace_assets",
		"list_workspaces",
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

func toolNames(tools []agent.ToolDefinition) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}
