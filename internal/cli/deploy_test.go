package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDeployPrintsPlanAndRequiresApprovalBeforeMutation(t *testing.T) {
	var mutations atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces/sales/assets":
			writeCLIJSON(t, w, map[string]any{"items": []map[string]any{}, "page": map[string]any{"nextCursor": ""}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces/sales/asset-edges":
			writeCLIJSON(t, w, map[string]any{"items": []map[string]any{}, "page": map[string]any{"nextCursor": ""}})
		default:
			mutations.Add(1)
			t.Fatalf("deploy mutated server before approval: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	var err error
	output := captureStdout(t, func() {
		err = runDeploy(context.Background(), &rootOptions{
			target:      server.URL,
			token:       "token",
			workspaceID: "sales",
			catalog:     filepath.Join("..", "..", "dashboards", "libredash.yaml"),
		})
	})
	if err == nil || !strings.Contains(err.Error(), "auto-approve") {
		t.Fatalf("runDeploy() error = %v, want approval error", err)
	}
	if mutations.Load() != 0 {
		t.Fatalf("mutations = %d, want 0", mutations.Load())
	}
	for _, want := range []string{"project libredash-showcase", "workspace sales", "changes +"} {
		if !strings.Contains(output, want) {
			t.Fatalf("deploy output missing plan text %q:\n%s", want, output)
		}
	}
}

func TestDeployAutoApproveActivatesAfterPlan(t *testing.T) {
	var sequence []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sequence = append(sequence, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces/sales/assets":
			writeCLIJSON(t, w, map[string]any{"items": []map[string]any{}, "page": map[string]any{"nextCursor": ""}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces/sales/asset-edges":
			writeCLIJSON(t, w, map[string]any{"items": []map[string]any{}, "page": map[string]any{"nextCursor": ""}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workspaces/sales/deployments":
			writeCLIJSON(t, w, map[string]any{"id": "dep_1", "workspaceId": "sales", "environment": "dev", "status": "pending"})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/workspaces/sales/deployments/dep_1/artifact":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workspaces/sales/deployments/dep_1/validate":
			writeCLIJSON(t, w, map[string]any{"id": "dep_1", "workspaceId": "sales", "environment": "dev", "status": "validated", "digest": "sha256:remote"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workspaces/sales/deployments/dep_1/activate":
			writeCLIJSON(t, w, map[string]any{"id": "dep_1", "workspaceId": "sales", "environment": "dev", "status": "active", "digest": "sha256:remote"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	output := captureStdout(t, func() {
		if err := runDeploy(context.Background(), &rootOptions{
			target:      server.URL,
			token:       "token",
			workspaceID: "sales",
			catalog:     filepath.Join("..", "..", "dashboards", "libredash.yaml"),
			autoApprove: true,
		}); err != nil {
			t.Fatalf("runDeploy() error = %v", err)
		}
	})
	if !strings.Contains(output, "workspace sales") || !strings.Contains(output, "deployed dep_1 environment=dev") {
		t.Fatalf("deploy output missing plan or final status:\n%s", output)
	}
	wantPrefix := []string{
		"GET /api/v1/workspaces/sales/assets",
		"GET /api/v1/workspaces/sales/asset-edges",
		"POST /api/v1/workspaces/sales/deployments",
	}
	for i, want := range wantPrefix {
		if len(sequence) <= i || sequence[i] != want {
			t.Fatalf("sequence = %#v, want prefix %#v", sequence, wantPrefix)
		}
	}
}

func TestDeployRequiresExactlyOneWorkspaceSelection(t *testing.T) {
	err := runDeploy(context.Background(), &rootOptions{})
	if err == nil || !strings.Contains(err.Error(), "--workspace or --all-workspaces") {
		t.Fatalf("runDeploy() error = %v, want missing workspace selection error", err)
	}

	err = runDeploy(context.Background(), &rootOptions{workspaceID: "sales", allWorkspaces: true})
	if err == nil || !strings.Contains(err.Error(), "either --workspace or --all-workspaces") {
		t.Fatalf("runDeploy() error = %v, want mutually exclusive selection error", err)
	}
}

func TestDeployAllWorkspacesDeploysSortedWorkspaces(t *testing.T) {
	var sequence []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sequence = append(sequence, r.Method+" "+r.URL.Path)
		workspaceID := workspaceIDFromPath(r.URL.Path)
		deploymentID := "dep_" + workspaceID
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/assets"):
			writeCLIJSON(t, w, map[string]any{"items": []map[string]any{}, "page": map[string]any{"nextCursor": ""}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/asset-edges"):
			writeCLIJSON(t, w, map[string]any{"items": []map[string]any{}, "page": map[string]any{"nextCursor": ""}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/deployments"):
			writeCLIJSON(t, w, map[string]any{"id": deploymentID, "workspaceId": workspaceID, "environment": "dev", "status": "pending"})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/deployments/"+deploymentID+"/artifact"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/deployments/"+deploymentID+"/validate"):
			writeCLIJSON(t, w, map[string]any{"id": deploymentID, "workspaceId": workspaceID, "environment": "dev", "status": "validated", "digest": "sha256:remote"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/deployments/"+deploymentID+"/activate"):
			writeCLIJSON(t, w, map[string]any{"id": deploymentID, "workspaceId": workspaceID, "environment": "dev", "status": "active", "digest": "sha256:remote"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	output := captureStdout(t, func() {
		if err := runDeploy(context.Background(), &rootOptions{
			target:        server.URL,
			token:         "token",
			catalog:       writeDeployProjectFixture(t),
			allWorkspaces: true,
			autoApprove:   true,
		}); err != nil {
			t.Fatalf("runDeploy() error = %v", err)
		}
	})
	for _, want := range []string{"workspace operations", "workspace sales", "deployed dep_operations", "deployed dep_sales"} {
		if !strings.Contains(output, want) {
			t.Fatalf("deploy output missing %q:\n%s", want, output)
		}
	}
	wantPrefix := []string{
		"GET /api/v1/workspaces/operations/assets",
		"GET /api/v1/workspaces/operations/asset-edges",
		"GET /api/v1/workspaces/sales/assets",
		"GET /api/v1/workspaces/sales/asset-edges",
		"POST /api/v1/workspaces/operations/deployments",
		"PUT /api/v1/workspaces/operations/deployments/dep_operations/artifact",
		"POST /api/v1/workspaces/operations/deployments/dep_operations/validate",
		"POST /api/v1/workspaces/operations/deployments/dep_operations/activate",
		"POST /api/v1/workspaces/sales/deployments",
		"PUT /api/v1/workspaces/sales/deployments/dep_sales/artifact",
		"POST /api/v1/workspaces/sales/deployments/dep_sales/validate",
		"POST /api/v1/workspaces/sales/deployments/dep_sales/activate",
	}
	for i, want := range wantPrefix {
		if len(sequence) <= i || sequence[i] != want {
			t.Fatalf("sequence = %#v, want prefix %#v", sequence, wantPrefix)
		}
	}
}

func writeDeployProjectFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"libredash.yaml": `
apiVersion: libredash.dev/v1
kind: Project
metadata:
  name: cli-test
spec:
  connections:
    include:
      - connections/*.yaml
  sources:
    include:
      - sources/*.yaml
  workspaces:
    include:
      - workspaces/*/workspace.yaml
`,
		"connections/olist.yaml": `
apiVersion: libredash.dev/v1
kind: Connection
metadata:
  name: olist
spec:
  kind: local
`,
		"sources/olist.orders.yaml": `
apiVersion: libredash.dev/v1
kind: Source
metadata:
  name: olist.orders
spec:
  connection: olist
  path: orders.csv
  fields:
    order_id:
      type: string
    order_status:
      type: string
`,
		"workspaces/operations/workspace.yaml": deployWorkspaceYAML("operations"),
		"workspaces/operations/models/orders.yaml": `
apiVersion: libredash.dev/v1
kind: ModelTable
metadata:
  workspace: operations
  name: orders
spec:
  primaryKey: order_id
  sources:
    - olist.orders
  fields:
    order_id:
      label: ID
      type: string
  transform:
    sql: |
      SELECT order_id, order_status FROM source."olist.orders"
`,
		"workspaces/operations/semantic-models/operations.yaml": deploySemanticModelYAML("operations"),
		"workspaces/operations/dashboards/operations.yaml":      deployDashboardYAML("operations"),
		"workspaces/sales/workspace.yaml":                       deployWorkspaceYAML("sales"),
		"workspaces/sales/models/orders.yaml": `
apiVersion: libredash.dev/v1
kind: ModelTable
metadata:
  workspace: sales
  name: orders
spec:
  primaryKey: order_id
  sources:
    - olist.orders
  fields:
    order_id:
      label: ID
      type: string
  transform:
    sql: |
      SELECT order_id, order_status FROM source."olist.orders"
`,
		"workspaces/sales/semantic-models/sales.yaml": deploySemanticModelYAML("sales"),
		"workspaces/sales/dashboards/sales.yaml":      deployDashboardYAML("sales"),
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "libredash.yaml")
}

func deployWorkspaceYAML(workspaceID string) string {
	return `
apiVersion: libredash.dev/v1
kind: Workspace
metadata:
  name: ` + workspaceID + `
  title: ` + workspaceID + `
spec:
  uses:
    sources:
      - olist.orders
  models:
    include:
      - models/*.yaml
  semanticModels:
    include:
      - semantic-models/*.yaml
  dashboards:
    include:
      - dashboards/*.yaml
  access:
    include: []
  agentPolicy:
    include: []
`
}

func deploySemanticModelYAML(workspaceID string) string {
	return `
apiVersion: libredash.dev/v1
kind: SemanticModel
metadata:
  workspace: ` + workspaceID + `
  name: ` + workspaceID + `
spec:
  baseTable: orders
  tables:
    - orders
  measures:
    defaults:
      table: orders
    order_count:
      expression: count(orders.order_id)
`
}

func deployDashboardYAML(workspaceID string) string {
	return `
apiVersion: libredash.dev/v1
kind: Dashboard
metadata:
  workspace: ` + workspaceID + `
  name: ` + workspaceID + `
  title: ` + workspaceID + `
spec:
  semanticModel: ` + workspaceID + `
  visuals:
    total:
      kind: kpi
      query:
        measures:
          order_count:
  pages:
    - name: overview
      title: Overview
      visuals:
        - id: total
          kind: kpi_card
          visual: total
          placement:
            col: 1
            row: 1
            col_span: 3
            row_span: 2
`
}

func workspaceIDFromPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "workspaces" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
