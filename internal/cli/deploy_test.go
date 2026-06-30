package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
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
