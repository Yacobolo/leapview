package ui

import (
	"strings"
	"testing"

	uisignals "github.com/Yacobolo/leapview/internal/admin/ui/signals"
	workspaceview "github.com/Yacobolo/leapview/internal/workspace"
	catalog "github.com/Yacobolo/leapview/internal/workspace/navigation"
)

func TestAdminBootstrapSignalsUseAdminOwnedContracts(t *testing.T) {
	signals := AdminBootstrapSignals(
		catalog.Catalog{Workspace: catalog.Workspace{ID: "workspace", Title: "Workspace"}},
		"general",
		"Platform admin",
		AdminData{
			Workspace:         workspaceview.WorkspaceView{ID: "platform", Title: "Platform"},
			AuthConfigured:    true,
			AccessConfigured:  true,
			AccessStatusLabel: "Configured",
		},
		WithAgentChrome(AgentChrome{
			ActiveConversationID: "conversation-1",
			Conversations: []AgentConversation{{
				ID: "conversation-1", Title: "Analysis", TitlePending: uisignals.Pointer(true),
			}},
		}),
	)

	chrome, ok := signals["chrome"].(uisignals.ChromeSignal)
	if !ok {
		t.Fatalf("chrome = %T, want admin signal contract", signals["chrome"])
	}
	if chrome.Sidebar.Active != "admin" || chrome.Sidebar.History == nil || len(chrome.Sidebar.History.Items) != 1 {
		t.Fatalf("chrome = %#v", chrome)
	}
	page, ok := signals["page"].(uisignals.AdminPageSignal)
	if !ok || page.Kind != uisignals.RouteAdmin || page.Active != "general" {
		t.Fatalf("page = %#v", signals["page"])
	}
	runtime, ok := signals["runtime"].(uisignals.RouteRuntimeSignal)
	if !ok || runtime.Kind != uisignals.RouteAdmin {
		t.Fatalf("runtime = %#v", signals["runtime"])
	}
}

func TestAdminPageRendersAdminRouteShell(t *testing.T) {
	var output strings.Builder
	err := AdminPage(
		catalog.Catalog{Workspace: catalog.Workspace{ID: "workspace", Title: "Workspace"}},
		"general",
		"Platform admin",
		AdminData{Workspace: workspaceview.WorkspaceView{ID: "platform", Title: "Platform"}},
	).Render(&output)
	if err != nil {
		t.Fatal(err)
	}
	html := output.String()
	for _, expected := range []string{"<lv-app-shell", "<lv-admin-page", "route=admin", "/static/admin-page.js"} {
		if !strings.Contains(html, expected) {
			t.Fatalf("admin page is missing %q:\n%s", expected, html)
		}
	}
	if strings.Contains(html, "data-signals=") {
		t.Fatalf("admin page embedded bootstrap signals:\n%s", html)
	}
}
