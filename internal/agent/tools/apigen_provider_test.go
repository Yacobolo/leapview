package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	agentcore "github.com/Yacobolo/leapview/pkg/agent"
	"github.com/Yacobolo/toolbelt/apigen/runtime/agenttool"
)

func TestGlobalAPIGenDefinitionsRequireWorkspaceForWorkspaceRoutes(t *testing.T) {
	var authorizedScope Scope
	var dispatchedPath string
	provider := APIGenProvider{
		Operations: testProviderOperations(),
		Authorize: func(_ context.Context, scope Scope, _ string) (agentcore.ToolResult, bool) {
			authorizedScope = scope
			return agentcore.ToolResult{}, true
		},
		Dispatch: func(_ Scope, _ string, writer http.ResponseWriter, request *http.Request) bool {
			dispatchedPath = request.URL.Path
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusOK)
			return true
		},
	}

	var definition agentcore.ToolDefinition
	for _, candidate := range provider.Definitions(Scope{PrincipalID: "principal-1"}) {
		if candidate.Name == "list_dashboards" {
			definition = candidate
			break
		}
	}
	if definition.Name == "" {
		t.Fatal("list_dashboards definition not found")
	}
	var schema struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	}
	if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
		t.Fatalf("decode input schema: %v", err)
	}
	if _, ok := schema.Properties["workspace"]; !ok || !containsString(schema.Required, "workspace") {
		t.Fatalf("global input schema = %s, want required workspace", definition.InputSchema)
	}

	result, err := definition.Handler.Run(context.Background(), agentcore.ToolCall{ID: "call-1", Arguments: json.RawMessage(`{"workspace":"sales"}`)})
	if err != nil {
		t.Fatalf("run tool: %v", err)
	}
	if result.IsError && !strings.Contains(dispatchedPath, "/api/v1/workspaces/sales/") {
		t.Fatalf("tool result = %#v", result)
	}
	if authorizedScope.WorkspaceID != "sales" {
		t.Fatalf("authorized workspace = %q, want sales", authorizedScope.WorkspaceID)
	}
	if dispatchedPath != "/api/v1/workspaces/sales/dashboards" {
		t.Fatalf("dispatched path = %q", dispatchedPath)
	}
}

func TestAPIGenDefinitionsExposeClosedVisualizationEnvelopeOutputSchemas(t *testing.T) {
	for _, definition := range (APIGenProvider{Operations: testProviderOperations()}).Definitions(Scope{PrincipalID: "principal-1"}) {
		if definition.Name != "query_dashboard_visual" {
			continue
		}
		var schema map[string]any
		if err := json.Unmarshal(definition.OutputSchema, &schema); err != nil {
			t.Fatalf("decode output schema: %v", err)
		}
		if schema["type"] != "object" {
			t.Fatalf("output schema type = %#v, want object: %s", schema["type"], definition.OutputSchema)
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("output schema is not closed: %s", definition.OutputSchema)
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("output schema properties = %#v", schema["properties"])
		}
		for _, property := range []string{"spec", "dataState"} {
			propertySchema, ok := properties[property].(map[string]any)
			if !ok {
				t.Fatalf("output schema property %q = %#v", property, properties[property])
			}
			if _, ok := propertySchema["oneOf"]; !ok {
				t.Fatalf("output schema property %q lost its discriminated union: %s", property, definition.OutputSchema)
			}
		}
		return
	}
	t.Fatal("query_dashboard_visual definition not found")
}

func testProviderOperations() []APIGenOperation {
	return []APIGenOperation{
		{
			Contract: OperationContract{OperationID: "listDashboards", Method: "GET", Path: "/api/v1/workspaces/{workspace}/dashboards"},
			Tool: agenttool.Contract{
				Name: "list_dashboards", OperationID: "listDashboards", Method: "GET",
				Path: "/api/v1/workspaces/{workspace}/dashboards", Effect: agenttool.EffectRead,
				InputSchema: json.RawMessage(`{"additionalProperties":false,"properties":{},"type":"object"}`),
				Bindings: []agenttool.Binding{{
					Source: "path", WireName: "workspace", Mode: "context", ContextKey: "workspace",
					Required: true, Schema: agenttool.ValueSchema{Type: "string"},
				}},
			},
		},
		{
			Contract: OperationContract{OperationID: "queryDashboardVisual", Method: "POST", Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/visuals/{visual}/query"},
			Tool: agenttool.Contract{
				Name: "query_dashboard_visual", OperationID: "queryDashboardVisual", Method: "POST",
				Path:   "/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/visuals/{visual}/query",
				Effect: agenttool.EffectRead, InputSchema: json.RawMessage(`{"type":"object"}`),
				OutputSchema: json.RawMessage(`{"additionalProperties":false,"properties":{"spec":{"oneOf":[{"type":"object"}]},"dataState":{"oneOf":[{"type":"object"}]}},"required":["spec","dataState"],"type":"object"}`),
			},
		},
	}
}

func TestGlobalVisualDefinitionRequiresWorkspace(t *testing.T) {
	definition := (VisualProvider{}).Definitions(Scope{PrincipalID: "principal-1"})[0]
	var schema struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	}
	if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
		t.Fatalf("decode input schema: %v", err)
	}
	if _, ok := schema.Properties["workspace"]; !ok || !containsString(schema.Required, "workspace") {
		t.Fatalf("global visual schema = %s, want required workspace", definition.InputSchema)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
