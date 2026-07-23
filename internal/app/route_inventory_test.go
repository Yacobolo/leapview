package app

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	"github.com/go-chi/chi/v5"
)

// TestRouteInventory is the migration contract for moving route ownership out
// of process composition. Generated public API routes come from the TypeSpec
// contract; UI and operational routes are deliberately enumerated here.
func TestRouteInventory(t *testing.T) {
	server := assembleRuntime(fakeMetrics{}, assemblyConfig{})
	server.persistenceConfigured = true
	routes, ok := server.Routes().(chi.Routes)
	if !ok {
		t.Fatal("application handler does not expose chi routes")
	}
	got := map[string]int{}
	if err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method != "*" {
			got[method+" "+route]++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	want := map[string]struct{}{}
	for _, route := range strings.Split(strings.TrimSpace(nonAPIRouteInventory), "\n") {
		want[strings.TrimSpace(route)] = struct{}{}
	}
	for _, contract := range apigenapi.GetAPIGenOperationContracts() {
		want[contract.Method+" "+contract.Path] = struct{}{}
	}

	var problems []string
	for route, count := range got {
		if count != 1 {
			problems = append(problems, route+" registered more than once")
		}
		if _, ok := want[route]; !ok {
			problems = append(problems, "unexpected "+route)
		}
	}
	for route := range want {
		if got[route] == 0 {
			problems = append(problems, "missing "+route)
		}
	}
	sort.Strings(problems)
	if len(problems) != 0 {
		t.Fatalf("route inventory changed:\n%s", strings.Join(problems, "\n"))
	}
}

const nonAPIRouteInventory = `
CONNECT /metrics
CONNECT /static/*
DELETE /metrics
DELETE /static/*
GET /
GET /.well-known/oauth-authorization-server
GET /.well-known/oauth-protected-resource
GET /.well-known/oauth-protected-resource/mcp
GET /__dev/pagestream/signals
GET /__dev/pagestream/traces
GET /admin
GET /admin/agent
GET /admin/groups
GET /admin/groups/{group}
GET /admin/principals
GET /admin/principals/{principal}
GET /admin/publications
GET /admin/queries
GET /admin/storage
GET /api/docs
GET /api/openapi.json
GET /auth/{provider}
GET /auth/{provider}/callback
GET /chat
GET /chat/*
GET /chat/updates
GET /chats
GET /chats/new
GET /chats/references/search
GET /chats/restore
GET /chats/{conversation}
GET /connections
GET /connections/{asset}
GET /connections/{asset}/{section}
GET /connections/{connection}/sources/{source}
GET /connections/{connection}/sources/{source}/{section}
GET /data
GET /embed/dashboards/{publicId}
GET /embed/dashboards/{publicId}/pages/{page}
GET /favicon.ico
GET /healthz
GET /login
GET /metrics
GET /public/dashboards/{publicId}
GET /public/dashboards/{publicId}/pages/{page}
GET /public/dashboards/{publicId}/updates
GET /readyz
GET /static/*
GET /updates
GET /workspaces
GET /workspaces/{workspace}
GET /workspaces/{workspace}/access/search
GET /workspaces/{workspace}/assets/{asset}
GET /workspaces/{workspace}/assets/{asset}/{section}
GET /workspaces/{workspace}/dashboards/{dashboard}
GET /workspaces/{workspace}/dashboards/{dashboard}/pages/{page}
GET /workspaces/{workspace}/data
HEAD /metrics
HEAD /static/*
OPTIONS /metrics
OPTIONS /static/*
PATCH /admin/agent/config
PATCH /metrics
PATCH /static/*
POST /admin/publications/command
POST /admin/queries/command
POST /admin/storage/select-table
POST /auth/local/login
POST /auth/local/password
POST /auth/logout
POST /chat/turns
POST /chats/turns
POST /data/command
POST /metrics
POST /oauth/register
POST /oauth/revoke
POST /oauth/token
POST /public/dashboards/{publicId}/commands/clear-selection
POST /public/dashboards/{publicId}/commands/reload
POST /public/dashboards/{publicId}/commands/reset-filters
POST /public/dashboards/{publicId}/commands/select
POST /public/dashboards/{publicId}/commands/spatial-select
POST /public/dashboards/{publicId}/commands/visual-spatial-window
POST /public/dashboards/{publicId}/commands/visual-window
POST /static/*
POST /workspaces/{workspace}/access/remove
POST /workspaces/{workspace}/access/upsert
POST /workspaces/{workspace}/assets/{asset}/access/remove
POST /workspaces/{workspace}/assets/{asset}/access/upsert
POST /workspaces/{workspace}/assets/{asset}/refresh
POST /workspaces/{workspace}/commands/clear-selection
POST /workspaces/{workspace}/commands/reload
POST /workspaces/{workspace}/commands/reset-filters
POST /workspaces/{workspace}/commands/select
POST /workspaces/{workspace}/commands/spatial-select
POST /workspaces/{workspace}/commands/visual-spatial-window
POST /workspaces/{workspace}/commands/visual-window
PUT /metrics
PUT /static/*
TRACE /metrics
TRACE /static/*
`
