package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	agentcore "github.com/Yacobolo/leapview/pkg/agent"
)

func TestAPIGenDefinitionsRequireAndUseExplicitWorkspace(t *testing.T) {
	var authorizedScope Scope
	var dispatchedPath string
	provider := APIGenProvider{
		Authorize: func(_ context.Context, scope Scope, _ string) (agentcore.ToolResult, bool) {
			authorizedScope = scope
			return agentcore.ToolResult{}, true
		},
		Dispatch: func(_ Scope, _ string, request *http.Request) (*http.Response, bool) {
			dispatchedPath = request.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       http.NoBody,
			}, true
		},
	}

	var definition agentcore.ToolDefinition
	for _, candidate := range provider.Definitions(Scope{WorkspaceID: "embedded", PrincipalID: "principal-1"}) {
		if candidate.Name == "query_semantic_model" {
			definition = candidate
			break
		}
	}
	if definition.Name == "" {
		t.Fatal("query_semantic_model definition not found")
	}
	var schema struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	}
	if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
		t.Fatalf("decode input schema: %v", err)
	}
	if _, ok := schema.Properties["workspace"]; !ok || !containsString(schema.Required, "workspace") {
		t.Fatalf("input schema = %s, want required workspace", definition.InputSchema)
	}
	limit, _ := schema.Properties["limit"].(map[string]any)
	if limit["maximum"] != float64(maxAgentQueryRows) {
		t.Fatalf("agent query limit maximum = %#v, want %d", limit["maximum"], maxAgentQueryRows)
	}

	result, err := definition.Handler.Run(context.Background(), agentcore.ToolCall{ID: "call-1", Arguments: json.RawMessage(`{"workspace":"sales","model":"orders"}`)})
	if err != nil {
		t.Fatalf("run tool: %v", err)
	}
	if result.IsError && !strings.Contains(dispatchedPath, "/api/v1/workspaces/sales/") {
		t.Fatalf("tool result = %#v", result)
	}
	if authorizedScope.WorkspaceID != "sales" {
		t.Fatalf("authorized workspace = %q, want sales", authorizedScope.WorkspaceID)
	}
	if dispatchedPath != "/api/v1/workspaces/sales/semantic-models/orders/query" {
		t.Fatalf("dispatched path = %q", dispatchedPath)
	}
}

func TestAPIGenDefinitionsExposeCompactDashboardVisualOutputSchema(t *testing.T) {
	for _, definition := range (APIGenProvider{}).Definitions(Scope{PrincipalID: "principal-1"}) {
		if definition.Name != "query_dashboard_visual" {
			continue
		}
		if len(definition.OutputSchema) >= 24*1024 {
			t.Fatalf("dashboard visual output schema is %d bytes, want compact schema under 24 KiB", len(definition.OutputSchema))
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
		for _, property := range []string{
			"queryId", "servingSnapshot", "visualId", "title", "type", "columns", "rows",
			"appliedFilters", "status", "diagnostics", "completeness", "hasMore", "nextCursor",
		} {
			if _, ok := properties[property]; !ok {
				t.Fatalf("compact output schema missing %q: %s", property, definition.OutputSchema)
			}
		}
		columns, _ := properties["columns"].(map[string]any)
		items, _ := columns["items"].(map[string]any)
		columnProperties, _ := items["properties"].(map[string]any)
		for _, property := range []string{"id", "sourceRef", "label", "role", "dataType", "nullable", "format", "grain"} {
			if _, ok := columnProperties[property]; !ok {
				t.Fatalf("compact column schema missing %q: %#v", property, items)
			}
		}
		for _, property := range []string{"spec", "dataState", "rendererID", "selection"} {
			if _, ok := properties[property]; ok {
				t.Fatalf("compact output schema kept renderer field %q: %s", property, definition.OutputSchema)
			}
		}
		return
	}
	t.Fatal("query_dashboard_visual definition not found")
}

func TestAPIGenDefinitionsExposeSemanticQueryMetadataSchema(t *testing.T) {
	for _, definition := range (APIGenProvider{}).Definitions(Scope{PrincipalID: "principal-1"}) {
		if definition.Name != "query_semantic_model" {
			continue
		}
		var schema map[string]any
		if err := json.Unmarshal(definition.OutputSchema, &schema); err != nil {
			t.Fatalf("decode output schema: %v", err)
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("semantic output schema is not closed: %s", definition.OutputSchema)
		}
		properties, _ := schema["properties"].(map[string]any)
		for _, name := range []string{"queryId", "servingSnapshot", "freshness", "completeness", "columns", "rows", "hasMore"} {
			if _, ok := properties[name]; !ok {
				t.Fatalf("semantic output schema missing %q: %s", name, definition.OutputSchema)
			}
		}
		completeness, _ := properties["completeness"].(map[string]any)
		if completeness["additionalProperties"] != false {
			t.Fatalf("completeness schema is not closed: %#v", completeness)
		}
		completenessProperties, _ := completeness["properties"].(map[string]any)
		for _, name := range []string{"returnedRows", "hasMore"} {
			if _, ok := completenessProperties[name]; !ok {
				t.Fatalf("completeness schema missing %q: %#v", name, completeness)
			}
		}
		columns, _ := properties["columns"].(map[string]any)
		columnItems, _ := columns["items"].(map[string]any)
		if columnItems["additionalProperties"] != false {
			t.Fatalf("column schema is not closed: %#v", columnItems)
		}
		columnProperties, _ := columnItems["properties"].(map[string]any)
		for _, name := range []string{"name", "nullable", "fieldRef", "label", "kind", "dataType", "unit", "format"} {
			if _, ok := columnProperties[name]; !ok {
				t.Fatalf("column schema missing %q: %#v", name, columnItems)
			}
		}
		return
	}
	t.Fatal("query_semantic_model definition not found")
}

func TestCuratedQueryArgumentsAcceptCatalogReferenceIDs(t *testing.T) {
	semantic := normalizeCuratedQueryArguments("query_semantic_model", json.RawMessage(`{
		"model":"sales",
		"dimensions":[{"field":"sales.orders.status"}],
		"measures":[{"field":"sales.order_count"}],
		"filters":[{"fact":"sales.orders","field":"sales.orders.state","groups":[{"filters":[{"field":"sales.orders.city"}]}]}]
	}`))
	var semanticInput map[string]any
	if err := json.Unmarshal(semantic, &semanticInput); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(semanticInput)
	for _, want := range []string{`"field":"orders.status"`, `"field":"order_count"`, `"fact":"orders"`, `"field":"orders.city"`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("normalized semantic arguments missing %s: %s", want, encoded)
		}
	}

	visual := normalizeCuratedQueryArguments("query_dashboard_visual", json.RawMessage(`{
		"dashboard":"executive-sales",
		"page":"executive-sales.overview",
		"visual":"executive-sales.revenue_kpi"
	}`))
	if string(visual) != `{"dashboard":"executive-sales","page":"overview","visual":"revenue_kpi"}` {
		t.Fatalf("normalized dashboard arguments = %s", visual)
	}
}

func TestVisualDefinitionRequiresAndUsesExplicitWorkspace(t *testing.T) {
	var authorizedScope Scope
	provider := VisualProvider{
		Authorize: func(_ context.Context, scope Scope, _ VisualAuthorizationRequest) (agentcore.ToolResult, bool) {
			authorizedScope = scope
			return apigenAgentToolError("authorization_failed", "stop after scope capture"), false
		},
	}
	definition := provider.Definitions(Scope{WorkspaceID: "embedded", PrincipalID: "principal-1"})[0]
	var schema struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	}
	if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
		t.Fatalf("decode input schema: %v", err)
	}
	if _, ok := schema.Properties["workspace"]; !ok || !containsString(schema.Required, "workspace") {
		t.Fatalf("visual schema = %s, want required workspace", definition.InputSchema)
	}
	_, err := definition.Handler.Run(context.Background(), agentcore.ToolCall{
		ID:        "call-visual",
		Arguments: json.RawMessage(`{"workspace":"sales","type":"bar","model":"orders","dataset":"orders"}`),
	})
	if err != nil {
		t.Fatalf("run tool: %v", err)
	}
	if authorizedScope.WorkspaceID != "sales" {
		t.Fatalf("authorized workspace = %q, want sales", authorizedScope.WorkspaceID)
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
