package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Yacobolo/libredash/internal/api"
	apigenapi "github.com/Yacobolo/libredash/internal/api/gen"
	servingstatefs "github.com/Yacobolo/libredash/internal/servingstate/filesystem"
)

func TestDataDeployPreparesEveryCandidateBeforeOneAtomicRollout(t *testing.T) {
	projectPath := writeDataDeployProject(t)
	targetRevision := "sha256:" + strings.Repeat("a", 64)
	otherRevision := "sha256:" + strings.Repeat("b", 64)
	workspaces := []string{"operations", "sales"}
	var mu sync.Mutex
	var sequence []string
	artifactPins := map[string]map[string]string{}
	artifactDigests := map[string]string{}
	rolloutKeys := map[string]string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sequence = append(sequence, r.Method+" "+r.URL.Path)
		mu.Unlock()
		workspaceID := workspaceIDFromAPIPath(r.URL.Path)
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/active-asset-graph"):
			writeCLIJSON(t, w, activeGraphResponse(nil, nil))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/project-a/data-connections/customers/environments/prod/revision":
			writeCLIJSON(t, w, apigenapi.ManagedDataEnvironmentRevisionResponse{
				Environment: "prod",
				Revision: &apigenapi.ManagedDataRevisionSummaryResponse{
					Id: otherRevision, Status: apigenapi.ManagedDataRevisionStatusAvailable,
					CreatedAt: "2026-01-01T00:00:00Z", UploadSessionId: "upload-customers",
				},
			})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/data-connections/orders/environments/"):
			t.Fatalf("bootstrap deploy queried a current target revision: %s", r.URL.Path)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/publishes"):
			writeCLIJSON(t, w, api.PublishResponse{ID: "state-" + workspaceID, WorkspaceID: workspaceID, Environment: "prod", Status: "pending"})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/publishes/") && strings.HasSuffix(r.URL.Path, "/artifact"):
			artifactPins[workspaceID], artifactDigests[workspaceID] = readManagedDataPinsFromUpload(t, r.Body)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/validate"):
			writeCLIJSON(t, w, api.PublishResponse{
				ID: "state-" + workspaceID, WorkspaceID: workspaceID, Environment: "prod",
				Status: "validated", Digest: artifactDigests[workspaceID],
			})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/publishes/") && strings.HasSuffix(r.URL.Path, "/activate"):
			t.Fatalf("data deploy activated an individual publish: %s", r.URL.Path)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/project-a/data-connections/orders/rollouts":
			rolloutKeys["create"] = r.Header.Get("Idempotency-Key")
			assertAllCandidatesValidatedBeforeRollout(t, sequence, workspaces)
			var request apigenapi.ManagedDataRolloutCreateRequest
			decodeJSONTest(t, r, &request)
			if request.Environment != "prod" || request.RevisionId != targetRevision {
				t.Fatalf("rollout request scope = %#v", request)
			}
			if got := rolloutTargetPairs(request.Targets); strings.Join(got, ",") != "operations=state-operations,sales=state-sales" {
				t.Fatalf("rollout targets = %v", got)
			}
			writeJSONTest(t, w, http.StatusCreated, rolloutResponse("rollout-1", targetRevision, "prod", apigenapi.ManagedDataRolloutStatusDraft, request.Targets))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/project-a/data-connections/orders/rollouts/rollout-1/activate":
			rolloutKeys["activate"] = r.Header.Get("Idempotency-Key")
			writeJSONTest(t, w, http.StatusAccepted, rolloutResponse("rollout-1", targetRevision, "prod", apigenapi.ManagedDataRolloutStatusActive, []apigenapi.ManagedDataRolloutTargetRequest{
				{Workspace: "operations", ServingStateId: "state-operations"},
				{Workspace: "sales", ServingStateId: "state-sales"},
			}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	err := runDataDeploy(context.Background(), dataDeployRequest{
		ProjectPath: projectPath, Connection: "orders", Revision: targetRevision, Environment: "prod",
		Target: server.URL, Token: "secret-token", AutoApprove: true, Out: &out, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("runDataDeploy() error = %v", err)
	}
	if got := artifactPins["operations"]; len(got) != 1 || got["orders"] != targetRevision {
		t.Fatalf("operations pins = %#v", got)
	}
	if got := artifactPins["sales"]; len(got) != 2 || got["orders"] != targetRevision || got["customers"] != otherRevision {
		t.Fatalf("sales pins = %#v", got)
	}
	if rolloutKeys["create"] == "" || rolloutKeys["activate"] == "" || rolloutKeys["create"] == rolloutKeys["activate"] {
		t.Fatalf("rollout idempotency keys = %#v", rolloutKeys)
	}
	if strings.Contains(out.String(), "secret-token") || !strings.Contains(out.String(), "deployed "+targetRevision+" rollout=rollout-1 environment=prod status=active") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestDataDeployDoesNotCreateRolloutWhenCandidatePreparationFails(t *testing.T) {
	projectPath := writeDataDeployProject(t)
	revision := "sha256:" + strings.Repeat("a", 64)
	var rolloutCreated bool
	artifactDigests := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workspaceID := workspaceIDFromAPIPath(r.URL.Path)
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/active-asset-graph"):
			writeCLIJSON(t, w, activeGraphResponse(nil, nil))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/data-connections/customers/"):
			other := "sha256:" + strings.Repeat("b", 64)
			writeCLIJSON(t, w, apigenapi.ManagedDataEnvironmentRevisionResponse{Environment: "prod", Revision: &apigenapi.ManagedDataRevisionSummaryResponse{Id: other, Status: apigenapi.ManagedDataRevisionStatusAvailable, CreatedAt: "2026-01-01T00:00:00Z", UploadSessionId: "upload-1"}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/publishes"):
			writeCLIJSON(t, w, api.PublishResponse{ID: "state-" + workspaceID, WorkspaceID: workspaceID, Environment: "prod", Status: "pending"})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/artifact"):
			_, artifactDigests[workspaceID] = readManagedDataPinsFromUpload(t, r.Body)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/validate") && workspaceID == "operations":
			writeCLIJSON(t, w, api.PublishResponse{ID: "state-operations", WorkspaceID: "operations", Environment: "prod", Status: "validated", Digest: artifactDigests[workspaceID]})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/validate") && workspaceID == "sales":
			http.Error(w, "database password must not be returned", http.StatusInternalServerError)
		case strings.Contains(r.URL.Path, "/rollouts"):
			rolloutCreated = true
			t.Fatalf("rollout created after preparation failure: %s %s", r.Method, r.URL.Path)
		case strings.HasSuffix(r.URL.Path, "/activate"):
			t.Fatalf("individual publish activated after preparation failure: %s", r.URL.Path)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	err := runDataDeploy(context.Background(), dataDeployRequest{
		ProjectPath: projectPath, Connection: "orders", Revision: revision, Environment: "prod",
		Target: server.URL, Token: "secret-token", AutoApprove: true, Out: io.Discard, HTTPClient: server.Client(),
	})
	if err == nil || !strings.Contains(err.Error(), "sales") {
		t.Fatalf("runDataDeploy() error = %v, want sanitized sales preparation failure", err)
	}
	if strings.Contains(err.Error(), "database password") || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked server or token detail: %v", err)
	}
	if rolloutCreated {
		t.Fatal("rollout was created")
	}
}

func TestDataDeployRejectsInvalidRevisionBeforeNetworkMutation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid revision reached server")
	}))
	defer server.Close()
	err := runDataDeploy(context.Background(), dataDeployRequest{
		ProjectPath: writeDataDeployProject(t), Connection: "orders", Revision: "sha256:BAD", Environment: "prod",
		Target: server.URL, Token: "token", AutoApprove: true, Out: io.Discard,
	})
	if err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("runDataDeploy() error = %v", err)
	}
}

func readManagedDataPinsFromUpload(t *testing.T, body io.Reader) (map[string]string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "artifact.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, digest), body); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := servingstatefs.ExtractArtifact(path, root); err != nil {
		t.Fatal(err)
	}
	compiled, _, err := servingstatefs.LoadCompiledWorkspaceArtifact(root)
	if err != nil {
		t.Fatal(err)
	}
	return compiled.ManagedDataRevisions, hex.EncodeToString(digest.Sum(nil))
}

func assertAllCandidatesValidatedBeforeRollout(t *testing.T, sequence, workspaces []string) {
	t.Helper()
	for _, workspaceID := range workspaces {
		want := "POST /api/v1/workspaces/" + workspaceID + "/publishes/state-" + workspaceID + "/validate"
		found := false
		for _, request := range sequence {
			if request == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("rollout created before %s was validated; sequence=%#v", workspaceID, sequence)
		}
	}
}

func rolloutTargetPairs(targets []apigenapi.ManagedDataRolloutTargetRequest) []string {
	result := make([]string, len(targets))
	for i, target := range targets {
		result[i] = target.Workspace + "=" + target.ServingStateId
	}
	sort.Strings(result)
	return result
}

func rolloutResponse(id, revision, environment string, status apigenapi.ManagedDataRolloutStatus, targets []apigenapi.ManagedDataRolloutTargetRequest) apigenapi.ManagedDataRolloutResponse {
	result := apigenapi.ManagedDataRolloutResponse{Id: id, RevisionId: revision, Environment: environment, Status: status, CreatedAt: "2026-01-01T00:00:00Z"}
	for _, target := range targets {
		targetStatus := apigenapi.ManagedDataRolloutTargetStatusPending
		if status == apigenapi.ManagedDataRolloutStatusActive {
			targetStatus = apigenapi.ManagedDataRolloutTargetStatusActive
		}
		result.Targets = append(result.Targets, apigenapi.ManagedDataRolloutTargetResponse{Workspace: target.Workspace, ServingStateId: target.ServingStateId, Status: targetStatus})
	}
	return result
}

func writeDataDeployProject(t *testing.T) string {
	t.Helper()
	files := map[string]string{
		"libredash.yaml": `
apiVersion: libredash.dev/v1
kind: Project
metadata:
  name: project-a
spec:
  connections:
    include: [connections/*.yaml]
  sources:
    include: [sources/*.yaml]
  workspaces:
    include: [workspaces/*/workspace.yaml]
`,
		"connections/orders.yaml":                         managedConnectionYAML("orders"),
		"connections/customers.yaml":                      managedConnectionYAML("customers"),
		"sources/orders.orders.yaml":                      managedSourceYAML("orders.orders", "orders", "orders.csv", "order_id"),
		"sources/customers.customers.yaml":                managedSourceYAML("customers.customers", "customers", "customers.csv", "customer_id"),
		"workspaces/operations/workspace.yaml":            workspaceYAML("operations", "orders.orders", "orders", "order_id"),
		"workspaces/operations/models/orders.yaml":        modelYAML("operations", "orders", "orders.orders", "order_id"),
		"workspaces/operations/semantic-models/main.yaml": semanticModelYAML("operations", "orders"),
		"workspaces/sales/workspace.yaml":                 workspaceYAML("sales", "orders.orders, customers.customers", "orders, customers", "order_id"),
		"workspaces/sales/models/orders.yaml":             modelYAML("sales", "orders", "orders.orders", "order_id"),
		"workspaces/sales/models/customers.yaml":          modelYAML("sales", "customers", "customers.customers", "customer_id"),
		"workspaces/sales/semantic-models/main.yaml":      semanticModelYAML("sales", "orders"),
	}
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "libredash.yaml")
}

func managedConnectionYAML(name string) string {
	return "apiVersion: libredash.dev/v1\nkind: Connection\nmetadata:\n  name: " + name + "\nspec:\n  kind: managed\n  credentials:\n    provider: none\n"
}

func managedSourceYAML(name, connection, path, field string) string {
	return "apiVersion: libredash.dev/v1\nkind: Source\nmetadata:\n  name: " + name + "\nspec:\n  connection: " + connection + "\n  path: " + path + "\n  format: csv\n  fields:\n    " + field + ":\n      type: string\n"
}

func workspaceYAML(name, sources, models, _ string) string {
	return "apiVersion: libredash.dev/v1\nkind: Workspace\nmetadata:\n  name: " + name + "\nspec:\n  uses:\n    sources: [" + sources + "]\n  models:\n    include: [models/*.yaml]\n  semanticModels:\n    include: [semantic-models/*.yaml]\n  dashboards:\n    include: []\n  access:\n    include: []\n  agentPolicy:\n    include: []\n"
}

func modelYAML(workspace, name, source, field string) string {
	return "apiVersion: libredash.dev/v1\nkind: ModelTable\nmetadata:\n  workspace: " + workspace + "\n  name: " + name + "\nspec:\n  source: " + source + "\n  primaryKey: " + field + "\n  fields:\n    " + field + ":\n      label: ID\n"
}

func semanticModelYAML(workspace, table string) string {
	return "apiVersion: libredash.dev/v1\nkind: SemanticModel\nmetadata:\n  workspace: " + workspace + "\n  name: main\nspec:\n  baseTable: " + table + "\n  tables: [" + table + "]\n  measures: {}\n"
}
