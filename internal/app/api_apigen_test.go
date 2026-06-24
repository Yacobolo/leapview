package app

import (
	"testing"

	"github.com/Yacobolo/libredash/internal/access"
	apigenapi "github.com/Yacobolo/libredash/internal/api/gen"
)

func TestAPIGenRoutesCoverHeadlessAPINotUITransports(t *testing.T) {
	spec, err := apigenapi.GetEmbeddedOpenAPISpec()
	if err != nil {
		t.Fatalf("embedded openapi: %v", err)
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatalf("openapi paths missing: %#v", spec["paths"])
	}

	for _, path := range []string{
		"/api/workspaces",
		"/api/workspaces/{workspace}/assets",
		"/api/workspaces/{workspace}/asset-edges",
		"/api/workspaces/{workspace}/agent/conversations",
		"/api/workspaces/{workspace}/agent/conversations/{conversation}/messages",
		"/api/workspaces/{workspace}/agent/conversations/{conversation}/turns",
		"/api/workspaces/{workspace}/agent/runs/{run}/events",
		"/api/workspaces/{workspace}/roles",
		"/api/workspaces/{workspace}/role-bindings",
		"/api/workspaces/{workspace}/role-bindings/{principal}",
		"/api/deployments",
		"/api/deployments/{deployment}",
		"/api/deployments/{deployment}/artifact",
		"/api/deployments/{deployment}/validate",
		"/api/deployments/{deployment}/activate",
		"/api/deployments/{deployment}/rollback",
	} {
		if _, ok := paths[path]; !ok {
			t.Fatalf("generated OpenAPI missing path %s", path)
		}
	}

	for _, path := range []string{"/updates", "/commands/select", "/chat/updates", "/dashboards/{dashboard}"} {
		if _, ok := paths[path]; ok {
			t.Fatalf("generated OpenAPI should not include UI transport path %s", path)
		}
	}
}

func TestAPIGenOperationAuthCoverage(t *testing.T) {
	contracts := apigenapi.GetAPIGenOperationContracts()
	if len(contracts) == 0 {
		t.Fatal("no generated operation contracts")
	}
	for operationID, contract := range contracts {
		if contract.AuthzMode != "permission" || !contract.Protected {
			t.Fatalf("%s auth contract = mode %q protected %t, want permission/protected", operationID, contract.AuthzMode, contract.Protected)
		}
		if _, ok := apigenOperationPermissions[operationID]; !ok {
			t.Fatalf("%s missing app permission mapping", operationID)
		}
	}
	for operationID := range apigenOperationPermissions {
		if _, ok := contracts[operationID]; !ok {
			t.Fatalf("%s has app permission mapping but no generated contract", operationID)
		}
	}
	if got := apigenOperationPermissions["uploadDeploymentArtifact"]; got != access.PermissionDeploymentCreate {
		t.Fatalf("uploadDeploymentArtifact permission = %q, want %q", got, access.PermissionDeploymentCreate)
	}
}
