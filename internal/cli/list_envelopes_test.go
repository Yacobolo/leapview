package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDeploymentsListDecodesEnvelopePreservingTableOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/test/deployments" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("Authorization = %q", got)
		}
		writeCLIJSON(t, w, map[string]any{
			"items": []map[string]any{{
				"id":          "dep_1",
				"workspaceId": "test",
				"status":      "active",
				"digest":      "sha256:1234567890abcdef",
				"createdAt":   "2026-01-02T15:04:05Z",
				"activatedAt": "2026-01-02T15:05:05Z",
			}},
			"page": map[string]any{"nextCursor": ""},
		})
	}))
	defer server.Close()

	output := captureStdout(t, func() {
		err := runDeploymentsList(context.Background(), &rootOptions{target: server.URL, token: "token", workspaceID: "test"})
		if err != nil {
			t.Fatalf("run list: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("output lines = %d, want 2:\n%s", len(lines), output)
	}
	if got := strings.Fields(lines[0]); strings.Join(got, "|") != "ID|STATUS|DIGEST|CREATED|ACTIVATED" {
		t.Fatalf("header fields = %#v output=\n%s", got, output)
	}
	if got := strings.Fields(lines[1]); strings.Join(got, "|") != "dep_1|active|sha256:12345|2026-01-02T15:04:05Z|2026-01-02T15:05:05Z" {
		t.Fatalf("row fields = %#v output=\n%s", got, output)
	}
	if strings.Contains(output, "items") || strings.Contains(output, "nextCursor") {
		t.Fatalf("output leaked envelope:\n%s", output)
	}
}

func TestAgentConversationsDecodesEnvelopePreservingJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/test/agent/conversations" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		writeCLIJSON(t, w, map[string]any{
			"items": []map[string]any{{
				"id":          "conv_1",
				"workspaceId": "test",
				"principalId": "prn_1",
				"title":       "Ask",
				"status":      "active",
				"createdAt":   "2026-01-02T15:04:05Z",
				"updatedAt":   "2026-01-02T15:05:05Z",
			}},
			"page": map[string]any{"nextCursor": "opaque"},
		})
	}))
	defer server.Close()

	output := captureStdout(t, func() {
		err := runAgentConversations(context.Background(), &rootOptions{target: server.URL, token: "token", workspaceID: "test", jsonOutput: true})
		if err != nil {
			t.Fatalf("run conversations: %v", err)
		}
	})

	var rows []map[string]any
	if err := json.Unmarshal([]byte(output), &rows); err != nil {
		t.Fatalf("decode output: %v output=%s", err, output)
	}
	if len(rows) != 1 || rows[0]["id"] != "conv_1" || rows[0]["title"] != "Ask" {
		t.Fatalf("rows = %#v", rows)
	}
	if strings.Contains(output, "nextCursor") || strings.Contains(output, `"items"`) {
		t.Fatalf("output leaked envelope:\n%s", output)
	}
}

func TestFriendlyListCommandsPassPaginationQuery(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command func(context.Context, *rootOptions) *cobra.Command
		args    []string
		path    string
	}{
		{
			name:    "workspaces",
			command: workspacesCommand,
			args:    []string{"list"},
			path:    "/api/v1/workspaces",
		},
		{
			name:    "dashboards",
			command: dashboardsCommand,
			args:    []string{"list"},
			path:    "/api/v1/workspaces/test/dashboards",
		},
		{
			name:    "semantic-models",
			command: semanticModelsCommand,
			args:    []string{"list"},
			path:    "/api/v1/workspaces/test/semantic-models",
		},
		{
			name:    "deployments",
			command: deploymentsCommand,
			args:    []string{"list"},
			path:    "/api/v1/workspaces/test/deployments",
		},
		{
			name:    "agent conversations",
			command: agentCommand,
			args:    []string{"conversations"},
			path:    "/api/v1/workspaces/test/agent/conversations",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					t.Fatalf("path = %s want %s", r.URL.Path, tc.path)
				}
				if got := r.URL.Query().Get("limit"); got != "7" {
					t.Fatalf("limit = %q", got)
				}
				if got := r.URL.Query().Get("pageToken"); got != "cursor" {
					t.Fatalf("pageToken = %q", got)
				}
				writeCLIJSON(t, w, map[string]any{
					"items": []map[string]any{},
					"page":  map[string]any{"nextCursor": ""},
				})
			}))
			defer server.Close()

			opts := &rootOptions{workspaceID: "test"}
			cmd := tc.command(context.Background(), opts)
			args := append([]string{}, tc.args...)
			args = append(args, "--target", server.URL, "--token", "token", "--limit", "7", "--page-token", "cursor")
			cmd.SetArgs(args)
			captureStdout(t, func() {
				if err := cmd.Execute(); err != nil {
					t.Fatalf("run command: %v", err)
				}
			})
		})
	}
}

func TestDashboardDataCommandsUseGeneratedURLsAndBodies(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		method   string
		path     string
		wantBody []string
		response any
	}{
		{
			name:     "components",
			args:     []string{"components", "executive-sales", "overview"},
			method:   http.MethodGet,
			path:     "/api/v1/workspaces/test/dashboards/executive-sales/pages/overview/components",
			response: map[string]any{"items": []map[string]any{}, "page": map[string]any{"nextCursor": ""}},
		},
		{
			name:     "visual describe",
			args:     []string{"visual", "executive-sales", "overview", "orders"},
			method:   http.MethodGet,
			path:     "/api/v1/workspaces/test/dashboards/executive-sales/pages/overview/visuals/orders",
			response: map[string]any{"id": "orders", "title": "Orders"},
		},
		{
			name:     "visual data",
			args:     []string{"visual-data", "executive-sales", "overview", "orders", "--filters-json", `{"controls":{"state":{"values":["SP"]}}}`},
			method:   http.MethodPost,
			path:     "/api/v1/workspaces/test/dashboards/executive-sales/pages/overview/visuals/orders/data",
			wantBody: []string{`"filters"`, `"state"`},
			response: map[string]any{"id": "orders", "data": []map[string]any{}},
		},
		{
			name:     "table data",
			args:     []string{"table-data", "executive-sales", "overview", "orders", "--count", "7"},
			method:   http.MethodPost,
			path:     "/api/v1/workspaces/test/dashboards/executive-sales/pages/overview/tables/orders/data",
			wantBody: []string{`"count":7`},
			response: map[string]any{"title": "Orders", "blocks": map[string]any{}},
		},
		{
			name:     "filter options",
			args:     []string{"filter-options", "executive-sales", "overview", "state", "--limit", "7", "--page-token", "cursor"},
			method:   http.MethodPost,
			path:     "/api/v1/workspaces/test/dashboards/executive-sales/pages/overview/filters/state/options",
			wantBody: []string{},
			response: map[string]any{"items": []map[string]any{}, "page": map[string]any{"nextCursor": ""}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method {
					t.Fatalf("method=%s want=%s", r.Method, tc.method)
				}
				if r.URL.Path != tc.path {
					t.Fatalf("path=%s want=%s", r.URL.Path, tc.path)
				}
				if tc.name == "filter options" {
					if got := r.URL.Query().Get("limit"); got != "7" {
						t.Fatalf("limit=%q", got)
					}
					if got := r.URL.Query().Get("pageToken"); got != "cursor" {
						t.Fatalf("pageToken=%q", got)
					}
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				for _, want := range tc.wantBody {
					if !strings.Contains(string(body), want) {
						t.Fatalf("body missing %q: %s", want, body)
					}
				}
				writeCLIJSON(t, w, tc.response)
			}))
			defer server.Close()

			opts := &rootOptions{workspaceID: "test"}
			cmd := dashboardsCommand(context.Background(), opts)
			args := append([]string{}, tc.args...)
			args = append(args, "--target", server.URL, "--token", "token")
			cmd.SetArgs(args)
			captureStdout(t, func() {
				if err := cmd.Execute(); err != nil {
					t.Fatalf("run command: %v", err)
				}
			})
		})
	}
}

func TestAgentToolsCommandListsGeneratedTools(t *testing.T) {
	output := captureStdout(t, func() {
		cmd := agentCommand(context.Background(), &rootOptions{})
		cmd.SetArgs([]string{"tools"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("agent tools: %v", err)
		}
	})
	for _, want := range []string{"NAME", "PERMISSION", "list_dashboards", "asset:read", "list_workspace_assets", "query_dashboard_visual_data"} {
		if !strings.Contains(output, want) {
			t.Fatalf("agent tools output missing %q:\n%s", want, output)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = write
	defer func() {
		os.Stdout = original
	}()
	fn()
	if err := write.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	bytes, err := io.ReadAll(read)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(bytes)
}

func writeCLIJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
}
