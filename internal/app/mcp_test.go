package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yacobolo/libredash/internal/access"
	agentcap "github.com/Yacobolo/libredash/internal/agent"
)

func TestMCPRequiresBearerAndSupportsInitializeAndTools(t *testing.T) {
	store := testStore(t)
	server := NewWithOptions(fakeMetrics{}, Options{
		Store: store,
		Auth:  testAuth(store, "test", AuthConfig{DevBypass: true, DevAPIToken: "mcp-secret"}),
	})
	handler := server.Routes()

	unauthorized := mcpRequest(t, handler, "", "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if unauthorized.Code != http.StatusUnauthorized || !strings.Contains(unauthorized.Header().Get("WWW-Authenticate"), "Bearer") {
		t.Fatalf("unauthorized response = %d headers=%v body=%s", unauthorized.Code, unauthorized.Header(), unauthorized.Body.String())
	}

	initialized := mcpRequest(t, handler, "mcp-secret", "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	if initialized.Code != http.StatusOK {
		t.Fatalf("initialize = %d body=%s", initialized.Code, initialized.Body.String())
	}
	var initialize map[string]any
	if err := json.Unmarshal(initialized.Body.Bytes(), &initialize); err != nil {
		t.Fatalf("decode initialize: %v", err)
	}
	result := initialize["result"].(map[string]any)
	if result["protocolVersion"] != "2025-11-25" {
		t.Fatalf("protocol version = %#v", result["protocolVersion"])
	}

	listed := mcpRequest(t, handler, "mcp-secret", "2025-11-25", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if listed.Code != http.StatusOK {
		t.Fatalf("tools/list = %d body=%s", listed.Code, listed.Body.String())
	}
	var listResponse struct {
		Result struct {
			Tools []struct {
				Name         string         `json:"name"`
				Description  string         `json:"description"`
				InputSchema  map[string]any `json:"inputSchema"`
				OutputSchema map[string]any `json:"outputSchema"`
				Annotations  struct {
					ReadOnly bool `json:"readOnlyHint"`
				} `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	builtIn := map[string]struct {
		description string
		input       map[string]any
		output      map[string]any
		effect      string
	}{}
	for _, definition := range server.agentToolDefinitions(agentcap.Scope{PrincipalID: "dev", DevAuthBypass: true}) {
		var input, output map[string]any
		if err := json.Unmarshal(definition.InputSchema, &input); err != nil {
			t.Fatalf("decode built-in input schema %s: %v", definition.Name, err)
		}
		if err := json.Unmarshal(definition.OutputSchema, &output); err != nil {
			t.Fatalf("decode built-in output schema %s: %v", definition.Name, err)
		}
		builtIn[definition.Name] = struct {
			description string
			input       map[string]any
			output      map[string]any
			effect      string
		}{definition.Description, input, output, definition.Effect}
	}
	if len(listResponse.Result.Tools) != len(builtIn) {
		t.Fatalf("MCP tool count = %d, built-in count = %d", len(listResponse.Result.Tools), len(builtIn))
	}
	foundVisual := false
	for _, tool := range listResponse.Result.Tools {
		expected, ok := builtIn[tool.Name]
		if !ok {
			t.Fatalf("MCP exposed tool absent from built-in catalog: %s", tool.Name)
		}
		if tool.Description != expected.description || !jsonObjectsEqual(tool.InputSchema, expected.input) || !jsonObjectsEqual(tool.OutputSchema, expected.output) || tool.Annotations.ReadOnly != (expected.effect == "read") {
			t.Fatalf("MCP metadata differs for %s", tool.Name)
		}
		if tool.Name == "query_visual" {
			foundVisual = true
			if !tool.Annotations.ReadOnly || tool.InputSchema["type"] != "object" || tool.OutputSchema["type"] != "object" {
				t.Fatalf("query_visual metadata = %#v", tool)
			}
			properties := tool.InputSchema["properties"].(map[string]any)
			if _, ok := properties["workspace"]; !ok {
				t.Fatalf("global query_visual schema does not require a workspace: %#v", tool.InputSchema)
			}
		}
	}
	if !foundVisual {
		t.Fatalf("tools/list omitted query_visual: %s", listed.Body.String())
	}

	called := mcpRequest(t, handler, "mcp-secret", "2025-11-25", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_workspaces","arguments":{}}}`)
	if called.Code != http.StatusOK {
		t.Fatalf("tools/call = %d body=%s", called.Code, called.Body.String())
	}
	var callResponse struct {
		Result struct {
			IsError           bool           `json:"isError"`
			StructuredContent map[string]any `json:"structuredContent"`
			Content           []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(called.Body.Bytes(), &callResponse); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	if callResponse.Result.IsError || len(callResponse.Result.Content) != 1 {
		t.Fatalf("tools/call result = %#v body=%s", callResponse.Result, called.Body.String())
	}
	var textContent map[string]any
	if err := json.Unmarshal([]byte(callResponse.Result.Content[0].Text), &textContent); err != nil {
		t.Fatalf("decode tools/call text: %v", err)
	}
	if !jsonObjectsEqual(callResponse.Result.StructuredContent, textContent) {
		t.Fatalf("structured and text output differ: structured=%#v text=%#v", callResponse.Result.StructuredContent, textContent)
	}
}

func TestMCPReturnsValidationFailuresAsToolErrorsAndRejectsOrigins(t *testing.T) {
	store := testStore(t)
	server := NewWithOptions(fakeMetrics{}, Options{
		Store: store,
		Auth:  testAuth(store, "test", AuthConfig{DevBypass: true, DevAPIToken: "mcp-secret"}),
	})
	handler := server.Routes()

	invalid := mcpRequest(t, handler, "mcp-secret", "2025-11-25", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"query_visual","arguments":{}}}`)
	if invalid.Code != http.StatusOK {
		t.Fatalf("invalid call = %d body=%s", invalid.Code, invalid.Body.String())
	}
	var response struct {
		Result struct {
			IsError           bool           `json:"isError"`
			StructuredContent map[string]any `json:"structuredContent"`
			Content           []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(invalid.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode invalid call: %v", err)
	}
	if !response.Result.IsError || response.Result.StructuredContent["error"] == nil || len(response.Result.Content) != 1 || !json.Valid([]byte(response.Result.Content[0].Text)) {
		t.Fatalf("validation result = %#v body=%s", response.Result, invalid.Body.String())
	}

	origin := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	origin.Header.Set("Authorization", "Bearer mcp-secret")
	origin.Header.Set("Content-Type", "application/json")
	origin.Header.Set("Origin", "https://attacker.example")
	originRec := httptest.NewRecorder()
	handler.ServeHTTP(originRec, origin)
	if originRec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want 403 body=%s", originRec.Code, originRec.Body.String())
	}
}

func TestMCPBearerCredentialRestrictions(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	if _, err := store.SQLDB().ExecContext(ctx, `INSERT INTO workspaces (id, title) VALUES ('other', 'Other')`); err != nil {
		t.Fatalf("seed other workspace: %v", err)
	}
	principal := testPrincipal(t, ctx, store, "mcp@example.com", "MCP User", "viewer")
	repo := testAccessRepository(store)
	validSecret, _, err := repo.CreateAPITokenWithMetadata(ctx, access.APITokenInput{
		PrincipalID: principal.ID,
		WorkspaceID: "test",
		Name:        "mcp-valid",
		Privileges:  []access.Privilege{access.PrivilegeUseAgent, access.PrivilegeViewItem},
	})
	if err != nil {
		t.Fatalf("create valid token: %v", err)
	}
	restrictedSecret, _, err := repo.CreateAPITokenWithMetadata(ctx, access.APITokenInput{
		PrincipalID: principal.ID,
		WorkspaceID: "test",
		Name:        "mcp-restricted",
		Privileges:  []access.Privilege{access.PrivilegeViewItem},
	})
	if err != nil {
		t.Fatalf("create restricted token: %v", err)
	}
	revokedSecret, revoked, err := repo.CreateAPITokenWithMetadata(ctx, access.APITokenInput{
		PrincipalID: principal.ID,
		Name:        "mcp-revoked",
	})
	if err != nil {
		t.Fatalf("create revoked token: %v", err)
	}
	if err := repo.RevokeAPIToken(ctx, revoked.ID); err != nil {
		t.Fatalf("revoke token: %v", err)
	}
	expiredSecret, expired, err := repo.CreateAPITokenWithMetadata(ctx, access.APITokenInput{
		PrincipalID: principal.ID,
		Name:        "mcp-expired",
	})
	if err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	if _, err := store.SQLDB().ExecContext(ctx, `UPDATE api_tokens SET expires_at = '2000-01-01T00:00:00Z' WHERE id = ?`, expired.ID); err != nil {
		t.Fatalf("expire token: %v", err)
	}

	server := NewWithOptions(fakeMetrics{}, Options{Store: store, Auth: testAuth(store, "test", AuthConfig{APITokenOnly: true})})
	handler := server.Routes()
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	for name, token := range map[string]struct {
		token string
		want  int
	}{
		"valid":      {validSecret, http.StatusOK},
		"invalid":    {"invalid", http.StatusUnauthorized},
		"restricted": {restrictedSecret, http.StatusForbidden},
		"revoked":    {revokedSecret, http.StatusUnauthorized},
		"expired":    {expiredSecret, http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			response := mcpRequest(t, handler, token.token, "", initialize)
			if response.Code != token.want {
				t.Fatalf("status = %d, want %d body=%s", response.Code, token.want, response.Body.String())
			}
		})
	}

	foreignWorkspace := mcpRequest(t, handler, validSecret, "2025-11-25", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_dashboards","arguments":{"workspace":"other"}}}`)
	if foreignWorkspace.Code != http.StatusOK || !strings.Contains(foreignWorkspace.Body.String(), `"isError":true`) || !strings.Contains(foreignWorkspace.Body.String(), "credential is not allowed") {
		t.Fatalf("foreign workspace response = %d body=%s", foreignWorkspace.Code, foreignWorkspace.Body.String())
	}
	audits, err := repo.ListAuditEvents(ctx, access.AuditEventFilter{WorkspaceID: "other", Action: "agent_tool.called"})
	if err != nil {
		t.Fatalf("list MCP tool audits: %v", err)
	}
	if len(audits) != 1 || audits[0].Status != "denied" || audits[0].TargetID != "listDashboards" {
		t.Fatalf("MCP credential denial was not audited: %#v", audits)
	}

	cookieOnly := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(initialize))
	cookieOnly.Header.Set("Content-Type", "application/json")
	cookieOnly.Header.Set("Accept", "application/json, text/event-stream")
	cookieOnly.AddCookie(&http.Cookie{Name: "ld_session", Value: "browser-session"})
	cookieOnlyRec := httptest.NewRecorder()
	handler.ServeHTTP(cookieOnlyRec, cookieOnly)
	if cookieOnlyRec.Code != http.StatusUnauthorized {
		t.Fatalf("browser session status = %d, want 401", cookieOnlyRec.Code)
	}
}

func TestMCPUsesAPIRateAndBodyLimits(t *testing.T) {
	store := testStore(t)
	server := NewWithOptions(fakeMetrics{}, Options{
		Store: store,
		Auth:  testAuth(store, "test", AuthConfig{DevBypass: true, DevAPIToken: "mcp-secret"}),
		RateLimits: RateLimitConfig{
			Enabled:   true,
			APILimit:  1,
			APIWindow: time.Minute,
		},
		RequestBodyLimit: RequestBodyLimitConfig{Enabled: true, MaxBytes: 512},
	})
	handler := server.Routes()
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	if first := mcpRequest(t, handler, "mcp-secret", "", initialize); first.Code != http.StatusOK {
		t.Fatalf("first MCP request = %d body=%s", first.Code, first.Body.String())
	}
	if second := mcpRequest(t, handler, "mcp-secret", "", initialize); second.Code != http.StatusTooManyRequests {
		t.Fatalf("second MCP request = %d, want 429 body=%s", second.Code, second.Body.String())
	}

	bodyStore := testStore(t)
	bodyLimited := NewWithOptions(fakeMetrics{}, Options{
		Store:            bodyStore,
		Auth:             testAuth(bodyStore, "test", AuthConfig{DevBypass: true, DevAPIToken: "mcp-secret"}),
		RequestBodyLimit: RequestBodyLimitConfig{Enabled: true, MaxBytes: 16},
	})
	oversized := mcpRequest(t, bodyLimited.Routes(), "mcp-secret", "", initialize)
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized MCP request = %d, want 413 body=%s", oversized.Code, oversized.Body.String())
	}
}

func mcpRequest(t *testing.T, handler http.Handler, token, protocolVersion, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if protocolVersion != "" {
		req.Header.Set("Mcp-Protocol-Version", protocolVersion)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func jsonObjectsEqual(left, right map[string]any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return bytes.Equal(leftJSON, rightJSON)
}
