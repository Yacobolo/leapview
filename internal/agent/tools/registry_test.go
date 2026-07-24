package tools

import (
	"slices"
	"testing"

	"github.com/Yacobolo/toolbelt/apigen/runtime/agenttool"
)

func TestAPIGenOperationsUseGeneratedReadOnlyToolContracts(t *testing.T) {
	operations := testAPIGenOperations()
	if len(operations) != 1 {
		t.Fatalf("BuildAPIGenOperations() count = %d, want 1", len(operations))
	}
	if operation := operations[0]; operation.Tool.OperationID != operation.Contract.OperationID {
		t.Fatalf("tool operation = %q, registry operation = %q", operation.Tool.OperationID, operation.Contract.OperationID)
	}
	if !slices.Contains(APIGenToolNames(operations), "list_dashboards") {
		t.Fatalf("APIGenToolNames() = %#v, want list_dashboards", APIGenToolNames(operations))
	}
}

func TestWorkspaceBindingIsTrustedContext(t *testing.T) {
	for _, operation := range testAPIGenOperations() {
		if operation.Tool.Name != "list_dashboards" {
			continue
		}
		for _, binding := range operation.Tool.Bindings {
			if binding.WireName == "workspace" {
				if binding.Mode != "context" || binding.ContextKey != "workspace" {
					t.Fatalf("workspace binding = %#v", binding)
				}
				return
			}
		}
		t.Fatal("list_dashboards has no workspace binding")
	}
	t.Fatal("list_dashboards tool not found")
}

func testAPIGenOperations() []APIGenOperation {
	contracts := map[string]OperationContract{
		"listDashboards": {
			OperationID: "listDashboards", Method: "GET", Path: "/api/v1/workspaces/{workspace}/dashboards",
			Protected: true, AuthzMode: "privilege",
			Extensions: map[string]any{"x-authz": map[string]any{"privilege": "VIEW_ITEM"}},
		},
		"mutateDashboard": {
			OperationID: "mutateDashboard", Method: "DELETE", Path: "/api/v1/workspaces/{workspace}/dashboards/{dashboard}",
			Protected: true, AuthzMode: "privilege",
			Extensions: map[string]any{"x-authz": map[string]any{"privilege": "VIEW_ITEM"}},
		},
	}
	tools := map[string]agenttool.Contract{
		"list_dashboards": {
			Name: "list_dashboards", OperationID: "listDashboards", Effect: agenttool.EffectRead,
			Bindings: []agenttool.Binding{{WireName: "workspace", Mode: "context", ContextKey: "workspace"}},
		},
		"mutate_dashboard": {Name: "mutate_dashboard", OperationID: "mutateDashboard", Effect: agenttool.EffectRead},
	}
	return BuildAPIGenOperations(contracts, tools)
}
