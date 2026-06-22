package ui

import (
	"github.com/Yacobolo/libredash/internal/api"
	"github.com/Yacobolo/libredash/internal/dashboard"
	g "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

func ChatPage(catalog dashboard.Catalog, csrfToken, roleLabel string, signal api.AgentChatSignal) g.Node {
	return c.HTML5(c.HTML5Props{
		Title:    "LibreDash Chat",
		Language: "en",
		HTMLAttrs: []g.Node{
			g.Attr("data-color-mode", "auto"),
			g.Attr("data-light-theme", "light"),
			g.Attr("data-dark-theme", "dark"),
		},
		Head: pageHead(
			h.Script(h.Type("module"), h.Src(staticAsset("/static/sidebar.js"))),
			h.Script(h.Type("module"), h.Src(staticAsset("/static/chat.js"))),
			inspectorScript(),
			h.Script(h.Type("module"), h.Src("https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.2/bundles/datastar.js")),
		),
		Body: []g.Node{
			h.Main(h.Class(appRootClass),
				ds.Signals(map[string]any{
					"csrfToken": csrfToken,
					"agent":     signal,
				}),
				h.Div(h.Class(appShellClass),
					sidebar(sidebarConfigForWorkspace(catalog, "chat", roleLabel)),
					h.Section(h.Class("grid h-svh min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden bg-app"), h.Aria("label", "LibreDash chat"),
						workspaceHeader("Agent", "Chat", "Ask read-only questions about dashboards, metric views, and semantic models.", nil),
						h.Div(h.Class("grid min-h-0 min-w-0 grid-cols-[auto_minmax(0,1fr)] overflow-hidden max-md:grid-cols-1"),
							g.El("ld-chat-conversation-sidebar",
								h.Class("block min-h-0 border-r border-outline-variant bg-app max-md:hidden"),
								g.Attr("data-attr:conversations", "$agent.conversations"),
								g.Attr("data-attr:active-conversation-id", "$agent.activeConversationId"),
								g.Attr("data-attr:status", "$agent.status"),
								g.Attr("data-on:ld-chat-conversation-select", "$agent.activeConversationId = evt.detail.conversationId; "+postAction("/chat/conversations/select")),
							),
							h.Div(h.Class("grid min-h-0 min-w-0 grid-rows-[minmax(0,1fr)_auto] overflow-hidden bg-app"),
								g.El("ld-chat-thread",
									h.Class("block min-h-0 min-w-0 overflow-hidden"),
									g.Attr("data-attr:events", "$agent.events"),
									g.Attr("data-attr:status", "$agent.status"),
									g.Attr("data-attr:conversation-id", "$agent.activeConversationId"),
									g.Text(signal.Status.Error),
								),
								g.El("ld-chat-composer",
									h.Class("block border-t border-outline-variant bg-app"),
									g.Attr("data-attr:value", "$agent.composer.value"),
									g.Attr("data-attr:disabled", "$agent.status.running || $agent.composer.disabled"),
									g.Attr("data-attr:placeholder", "$agent.composer.placeholder"),
									g.Attr("data-on:ld-chat-submit", "$agent.composer.value = evt.detail.input; "+postAction("/chat/turns")),
								),
							),
						),
					),
				),
				inspectorElement(),
			),
		},
	})
}
