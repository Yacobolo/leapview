package tools

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/Yacobolo/toolbelt/apigen/runtime/agenttool"
)

func TestAPIGenOperationsUseGeneratedReadOnlyToolContracts(t *testing.T) {
	operations := APIGenOperations()
	if len(operations) != 2 {
		t.Fatalf("APIGenOperations() count = %d, want 2", len(operations))
	}
	operationsByName := make(map[string]APIGenOperation, len(operations))
	for _, operation := range operations {
		operationsByName[operation.Tool.Name] = operation
		if operation.Tool.Effect != agenttool.EffectRead {
			t.Fatalf("tool %q effect = %q, want read", operation.Tool.Name, operation.Tool.Effect)
		}
		if operation.Tool.OperationID != operation.Contract.OperationID {
			t.Fatalf("tool %q operation = %q, registry operation = %q", operation.Tool.Name, operation.Tool.OperationID, operation.Contract.OperationID)
		}
	}
	for name, operationID := range map[string]string{
		"query_semantic_model":   "querySemanticModel",
		"query_dashboard_visual": "queryDashboardVisualData",
	} {
		operation, ok := operationsByName[name]
		if !ok {
			t.Fatalf("APIGenOperations() missing generated tool %q", name)
		}
		if operation.Tool.OperationID != operationID {
			t.Fatalf("tool %q operation = %q, want %q", name, operation.Tool.OperationID, operationID)
		}
		if operation.Tool.Effect != agenttool.EffectRead {
			t.Fatalf("tool %q effect = %q, want read", name, operation.Tool.Effect)
		}
	}
	if slices.Contains(APIGenToolNames(), "query_dashboard_page") {
		t.Fatalf("APIGenToolNames() = %#v, must not contain query_dashboard_page", APIGenToolNames())
	}
}

func TestToolNamesAreTheCuratedSurface(t *testing.T) {
	want := []string{
		"catalog_get",
		"catalog_list",
		"catalog_search",
		"docs_read",
		"docs_search",
		"query_dashboard_visual",
		"query_semantic_model",
		"query_visual",
	}
	if got := ToolNames(); !slices.Equal(got, want) {
		t.Fatalf("ToolNames() = %#v, want %#v", got, want)
	}
}

func TestReferenceCatalogComesFromCanonicalProviderDefinitions(t *testing.T) {
	reference, err := ReferenceCatalog()
	if err != nil {
		t.Fatalf("ReferenceCatalog(): %v", err)
	}
	if len(reference) != len(ToolNames()) {
		t.Fatalf("ReferenceCatalog() count = %d, want %d", len(reference), len(ToolNames()))
	}
	definitions := (ProviderSet{}).Definitions(Scope{})
	if len(definitions) != len(reference) {
		t.Fatalf("ProviderSet definitions = %d, reference = %d", len(definitions), len(reference))
	}
	wantDefaults := map[string]map[string]any{
		"catalog_get": {}, "catalog_list": {"limit": 25}, "catalog_search": {"limit": 10},
		"docs_read": {"limit": 200, "offset": 1}, "docs_search": {"limit": 8},
		"query_dashboard_visual": {}, "query_semantic_model": {"limit": 25}, "query_visual": {"limit": 50},
	}
	for index, tool := range reference {
		definition := definitions[index]
		if tool.Name != definition.Name {
			t.Fatalf("reference[%d].Name = %q, definition = %q", index, tool.Name, definition.Name)
		}
		if !json.Valid(tool.InputSchema) || !json.Valid(tool.OutputSchema) {
			t.Fatalf("tool %q has invalid generated schemas", tool.Name)
		}
		if string(tool.InputSchema) != string(definition.InputSchema) || string(tool.OutputSchema) != string(definition.OutputSchema) {
			t.Fatalf("tool %q reference schemas drifted from provider definitions", tool.Name)
		}
		if tool.Effect != "read" || !tool.Annotations.ReadOnlyHint || !tool.Annotations.IdempotentHint || tool.Annotations.DestructiveHint || tool.Annotations.OpenWorldHint {
			t.Fatalf("tool %q annotations = %#v", tool.Name, tool.Annotations)
		}
		if tool.Privilege == "" || tool.OperationID == "" {
			t.Fatalf("tool %q metadata = %#v", tool.Name, tool)
		}
		gotDefaults, _ := json.Marshal(tool.Defaults)
		expectedDefaults, _ := json.Marshal(wantDefaults[tool.Name])
		if string(gotDefaults) != string(expectedDefaults) {
			t.Fatalf("tool %q defaults = %#v, want %#v", tool.Name, tool.Defaults, wantDefaults[tool.Name])
		}
	}
}
