package ui

import (
	"net/url"
	"strings"

	uisignals "github.com/Yacobolo/leapview/internal/admin/ui/signals"
	"github.com/Yacobolo/leapview/internal/platform/web/staticasset"
	catalog "github.com/Yacobolo/leapview/internal/workspace/navigation"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

const appRootClass = "min-h-svh bg-app text-fg-default"

type ChromeOption func(*uisignals.ChromeSignal)

type AgentConversation struct {
	ID           string
	Title        string
	TitlePending *bool
}

type AgentChrome struct {
	ActiveConversationID string
	Conversations        []AgentConversation
}

func WithAgentChrome(state AgentChrome) ChromeOption {
	return func(chrome *uisignals.ChromeSignal) {
		items := make([]uisignals.SidebarHistoryItemSignal, 0, len(state.Conversations))
		for _, conversation := range state.Conversations {
			title := conversation.Title
			if title == "" {
				title = "Conversation"
			}
			items = append(items, uisignals.SidebarHistoryItemSignal{
				ID: conversation.ID, Title: title, Href: "/chats/" + url.PathEscape(conversation.ID),
				Active: conversation.ID == state.ActiveConversationID, Pending: conversation.TitlePending,
			})
		}
		chrome.Sidebar.PrimaryAction = &uisignals.SidebarActionSignal{Label: "New chat", Href: "/chats/new", Icon: "plus"}
		chrome.Sidebar.History = &uisignals.SidebarHistorySignal{
			Label: "Chats", EmptyText: uisignals.Optional("No chats yet."), Items: items,
		}
	}
}

func applyChromeOptions(chrome *uisignals.ChromeSignal, options []ChromeOption) {
	for _, option := range options {
		if option != nil {
			option(chrome)
		}
	}
}

func adminChromeSidebar(catalog catalog.Catalog, roleLabel string) uisignals.SidebarSignal {
	workspaceTitle := strings.TrimSpace(catalog.Workspace.Title)
	if workspaceTitle == "" {
		workspaceTitle = strings.TrimSpace(catalog.Workspace.ID)
	}
	if workspaceTitle == "" {
		workspaceTitle = "LeapView"
	}
	return uisignals.SidebarSignal{
		WorkspaceTitle: workspaceTitle,
		Active:         "admin",
		DashboardTitle: "Workspace",
		PageTitle:      "Published assets",
		UserRole:       uisignals.Optional(roleLabel),
		Groups: []uisignals.SidebarGroupSignal{{
			Label: "Navigation",
			Items: []uisignals.SidebarItemSignal{
				{ID: "dashboards", Label: "Dashboards", Href: "/", Icon: "dashboard", Meta: uisignals.Optional("Reports")},
				{ID: "chat", Label: "Chats", Href: "/chats", Icon: "chat", Meta: uisignals.Optional("Agent interface")},
				{ID: "workspaces", Label: "Workspaces", Href: "/workspaces", Icon: "catalog", Meta: uisignals.Optional("Published assets")},
				{ID: "data", Label: "Data", Href: "/data", Icon: "cache", Meta: uisignals.Optional("Inspect rows")},
				{ID: "connections", Label: "Connections", Href: "/connections", Icon: "data", Meta: uisignals.Optional("Data access")},
				{ID: "admin", Label: "Admin", Href: "/admin", Icon: "settings", Meta: uisignals.Optional("Read-only administration")},
			},
		}},
	}
}

func updatesURL(route uisignals.RouteKind, pairs ...string) string {
	values := url.Values{}
	values.Set("route", string(route))
	for i := 0; i+1 < len(pairs); i += 2 {
		if strings.TrimSpace(pairs[i+1]) != "" {
			values.Set(pairs[i], pairs[i+1])
		}
	}
	return "/updates?" + values.Encode()
}

func staticAsset(path string) string {
	return staticasset.URL(path)
}

func datastarScriptURL() string {
	return staticAsset(staticasset.DatastarScriptPath)
}

func pageHead(extra ...g.Node) []g.Node {
	nodes := []g.Node{
		h.Link(h.Rel("icon"), h.Href(staticAsset("/static/favicon.svg")), h.Type("image/svg+xml")),
		h.Link(h.Rel("stylesheet"), h.Href(staticAsset("/static/app.css"))),
		h.Script(h.Src(staticAsset("/static/theme.js"))),
		h.Script(h.Type("module"), h.Src(staticAsset("/static/command.js"))),
	}
	return append(nodes, extra...)
}

func csrfMeta(token string) g.Node {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return h.Meta(h.Name("csrf-token"), h.Content(token))
}

func inspectorScript() g.Node {
	if staticasset.Production() {
		return nil
	}
	return h.Script(h.Type("module"), h.Src(staticAsset("/static/datastar-inspector.js")))
}

func inspectorElement() g.Node {
	if staticasset.Production() {
		return nil
	}
	return g.El("datastar-inspector", g.Attr("signals-url", "/__dev/pagestream/signals"))
}
