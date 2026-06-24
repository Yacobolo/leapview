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
