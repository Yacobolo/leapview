package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yacobolo/libredash/internal/access"
	apigenapi "github.com/Yacobolo/libredash/internal/api/gen"
)

func TestAPIGenUsesTypeSpecV030(t *testing.T) {
	root := projectRoot(t)
	manifest, err := os.ReadFile(filepath.Join(root, "api", "apigen.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	manifestText := string(manifest)
	if !strings.Contains(manifestText, "typespec_dir: typespec") {
		t.Fatalf("manifest should use TypeSpec source, got:\n%s", manifestText)
	}
	if strings.Contains(manifestText, "cue_dir:") {
		t.Fatalf("manifest should not use cue_dir after APIGen v0.3.0 migration")
	}

	taskfile, err := os.ReadFile(filepath.Join(root, "Taskfile.yml"))
	if err != nil {
		t.Fatalf("read Taskfile.yml: %v", err)
	}
	taskText := string(taskfile)
	for _, want := range []string{
		"github.com/Yacobolo/toolbelt/apigen/cmd/apigen@v0.3.0 typespec-compile",
		"github.com/Yacobolo/toolbelt/apigen/cmd/apigen@v0.3.0 all",
	} {
		if !strings.Contains(taskText, want) {
			t.Fatalf("Taskfile.yml missing generation command %q", want)
		}
	}
	for _, forbidden := range []string{"cue-compile", "apigen@v0.2.0"} {
		if strings.Contains(taskText, forbidden) {
			t.Fatalf("Taskfile.yml should not contain %q after APIGen v0.3.0 migration", forbidden)
		}
	}
}

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
		"/api/v1/me",
		"/api/v1/me/permissions",
		"/api/v1/me/api-tokens",
		"/api/v1/me/api-tokens/{token}",
		"/api/v1/me/sessions",
		"/api/v1/me/sessions/{session}",
		"/api/v1/principals",
		"/api/v1/principals/{principal}",
		"/api/v1/workspaces",
		"/api/v1/workspaces/{workspace}/assets",
		"/api/v1/workspaces/{workspace}/asset-edges",
		"/api/v1/workspaces/{workspace}/dashboards",
		"/api/v1/workspaces/{workspace}/dashboards/{dashboard}",
		"/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/components",
		"/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/visuals/{visual}",
		"/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/visuals/{visual}/data",
		"/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/tables/{table}/data",
		"/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/filters/{filter}/options",
		"/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/query",
		"/api/v1/workspaces/{workspace}/dashboards/{dashboard}/tables/{table}/query",
		"/api/v1/workspaces/{workspace}/semantic-models",
		"/api/v1/workspaces/{workspace}/semantic-models/{model}",
		"/api/v1/workspaces/{workspace}/deployments",
		"/api/v1/workspaces/{workspace}/deployments/{deployment}",
		"/api/v1/workspaces/{workspace}/deployments/{deployment}/artifact",
		"/api/v1/workspaces/{workspace}/deployments/{deployment}/validate",
		"/api/v1/workspaces/{workspace}/deployments/{deployment}/activate",
		"/api/v1/workspaces/{workspace}/materialization-runs",
		"/api/v1/workspaces/{workspace}/materialization-runs/{run}",
		"/api/v1/workspaces/{workspace}/agent/conversations",
		"/api/v1/workspaces/{workspace}/agent/conversations/{conversation}",
		"/api/v1/workspaces/{workspace}/agent/conversations/{conversation}/messages",
		"/api/v1/workspaces/{workspace}/agent/conversations/{conversation}/turns",
		"/api/v1/workspaces/{workspace}/agent/conversations/{conversation}/runs",
		"/api/v1/workspaces/{workspace}/agent/conversations/{conversation}/runs/{run}",
		"/api/v1/workspaces/{workspace}/agent/conversations/{conversation}/runs/{run}/events",
		"/api/v1/workspaces/{workspace}/roles",
		"/api/v1/workspaces/{workspace}/groups",
		"/api/v1/workspaces/{workspace}/groups/{group}",
		"/api/v1/workspaces/{workspace}/groups/{group}/members",
		"/api/v1/workspaces/{workspace}/groups/{group}/members/{principal}",
		"/api/v1/workspaces/{workspace}/role-bindings",
		"/api/v1/workspaces/{workspace}/role-bindings/{binding}",
		"/api/v1/workspaces/{workspace}/audit-events",
	} {
		if _, ok := paths[path]; !ok {
			t.Fatalf("generated OpenAPI missing path %s", path)
		}
	}

	for _, path := range []string{"/api/workspaces", "/api/deployments", "/api/v1/workspaces/{workspace}/deployments/{deployment}/rollback", "/updates", "/commands/select", "/chat/updates", "/dashboards/{dashboard}"} {
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

func TestAPIGenOperationExtensions(t *testing.T) {
	contracts := apigenapi.GetAPIGenOperationContracts()
	agentTools := map[string]string{
		"getDeployment":              "get_deployment",
		"getDashboard":               "describe_dashboard",
		"getDashboardVisual":         "describe_dashboard_visual",
		"getSemanticModel":           "describe_model",
		"getMaterializationRun":      "get_materialization_run",
		"listDashboardComponents":    "list_dashboard_components",
		"listDashboards":             "list_dashboards",
		"listDeployments":            "list_deployments",
		"listDashboardFilterOptions": "list_dashboard_filter_options",
		"listMaterializationRuns":    "list_materialization_runs",
		"listSemanticModels":         "list_semantic_models",
		"listWorkspaceAssetEdges":    "list_workspace_asset_edges",
		"listWorkspaceAssets":        "list_workspace_assets",
		"listWorkspaces":             "list_workspaces",
		"queryDashboardTableData":    "query_dashboard_table_data",
		"queryDashboardVisualData":   "query_dashboard_visual_data",
		"queryDashboardPage":         "query_dashboard_page",
		"queryDashboardTable":        "query_table",
	}
	for operationID, contract := range contracts {
		authz, ok := contract.Extensions["x-authz"].(map[string]any)
		if !ok {
			t.Fatalf("%s missing generated x-authz extension: %#v", operationID, contract.Extensions["x-authz"])
		}
		if got := authz["mode"]; got != "permission" {
			t.Fatalf("%s x-authz mode = %#v, want permission", operationID, got)
		}
		if got := authz["permission"]; got != apigenOperationPermissions[operationID] {
			t.Fatalf("%s x-authz permission = %#v, want %q", operationID, got, apigenOperationPermissions[operationID])
		}
		agentExtension, hasAgentExtension := contract.Extensions["x-agent"].(map[string]any)
		if wantName, ok := agentTools[operationID]; ok {
			if !hasAgentExtension {
				t.Fatalf("%s missing x-agent extension", operationID)
			}
			if got := agentExtension["enabled"]; got != true {
				t.Fatalf("%s x-agent enabled = %#v, want true", operationID, got)
			}
			if got := agentExtension["name"]; got != wantName {
				t.Fatalf("%s x-agent name = %#v, want %q", operationID, got, wantName)
			}
			if got := agentExtension["risk"]; got != "read" {
				t.Fatalf("%s x-agent risk = %#v, want read", operationID, got)
			}
		} else if hasAgentExtension {
			t.Fatalf("%s should not have x-agent metadata", operationID)
		}
		if operationID != "uploadDeploymentArtifact" {
			if _, ok := contract.Extensions["x-libredash-dispatch"]; ok {
				t.Fatalf("%s should not have raw-body dispatch extension", operationID)
			}
		}
	}

	upload, ok := contracts["uploadDeploymentArtifact"]
	if !ok {
		t.Fatal("uploadDeploymentArtifact contract missing")
	}
	if got := upload.Extensions["x-libredash-dispatch"]; got != "raw-body" {
		t.Fatalf("uploadDeploymentArtifact x-libredash-dispatch = %#v, want raw-body", got)
	}
}

func TestAPIGenListOperationsUseStandardEnvelope(t *testing.T) {
	spec, err := apigenapi.GetEmbeddedOpenAPISpec()
	if err != nil {
		t.Fatalf("embedded openapi: %v", err)
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatalf("openapi paths missing: %#v", spec["paths"])
	}
	components, _ := spec["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	for _, tc := range []struct {
		path   string
		method string
	}{
		{"/api/v1/workspaces", "get"},
		{"/api/v1/workspaces/{workspace}/assets", "get"},
		{"/api/v1/workspaces/{workspace}/asset-edges", "get"},
		{"/api/v1/workspaces/{workspace}/dashboards", "get"},
		{"/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/components", "get"},
		{"/api/v1/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}/filters/{filter}/options", "post"},
		{"/api/v1/workspaces/{workspace}/semantic-models", "get"},
		{"/api/v1/workspaces/{workspace}/deployments", "get"},
		{"/api/v1/workspaces/{workspace}/materialization-runs", "get"},
		{"/api/v1/workspaces/{workspace}/agent/conversations", "get"},
	} {
		operation := mustOpenAPIOperation(t, paths, tc.path, tc.method)
		for _, want := range []string{"limit", "pageToken"} {
			if !openAPIOperationHasQueryParam(operation, want) {
				t.Fatalf("%s %s missing query param %s", tc.method, tc.path, want)
			}
		}
		schemaName := responseSchemaName(operation, "200")
		if schemaName == "" {
			t.Fatalf("%s %s missing 200 response schema", tc.method, tc.path)
		}
		schema, _ := schemas[schemaName].(map[string]any)
		properties, _ := schema["properties"].(map[string]any)
		if _, ok := properties["items"]; !ok {
			t.Fatalf("%s %s schema %s missing items property: %#v", tc.method, tc.path, schemaName, properties)
		}
		if _, ok := properties["page"]; !ok {
			t.Fatalf("%s %s schema %s missing page property: %#v", tc.method, tc.path, schemaName, properties)
		}
		if _, ok := properties["dashboards"]; ok {
			t.Fatalf("%s %s schema %s has legacy dashboards property", tc.method, tc.path, schemaName)
		}
		if _, ok := properties["models"]; ok {
			t.Fatalf("%s %s schema %s has legacy models property", tc.method, tc.path, schemaName)
		}
	}
}

func mustOpenAPIOperation(t *testing.T, paths map[string]any, path, method string) map[string]any {
	t.Helper()
	pathItem, ok := paths[path].(map[string]any)
	if !ok {
		t.Fatalf("path %s missing", path)
	}
	operation, ok := pathItem[method].(map[string]any)
	if !ok {
		t.Fatalf("%s operation missing for %s", method, path)
	}
	return operation
}

func openAPIOperationHasQueryParam(operation map[string]any, name string) bool {
	parameters, _ := operation["parameters"].([]any)
	for _, raw := range parameters {
		parameter, _ := raw.(map[string]any)
		if parameter["name"] == name && parameter["in"] == "query" {
			return true
		}
	}
	return false
}

func responseSchemaName(operation map[string]any, status string) string {
	responses, _ := operation["responses"].(map[string]any)
	response, _ := responses[status].(map[string]any)
	content, _ := response["content"].(map[string]any)
	jsonContent, _ := content["application/json"].(map[string]any)
	schema, _ := jsonContent["schema"].(map[string]any)
	ref, _ := schema["$ref"].(string)
	return strings.TrimPrefix(ref, "#/components/schemas/")
}

func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatalf("could not find project root from %s", dir)
		}
		dir = next
	}
}
