package ui

import (
	"net/url"
	"strings"

	uiactions "github.com/Yacobolo/leapview/internal/platform/web/actions"
	"github.com/Yacobolo/leapview/internal/platform/web/staticasset"
	"github.com/Yacobolo/leapview/internal/workspace/navigation"
	"github.com/Yacobolo/leapview/pkg/pagestream"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func ChatPage(catalog navigation.Catalog, workspaceID, csrfToken, roleLabel, view string, state ChatViewState) g.Node {
	return pagestream.RenderPage(pagestream.PageSpec{
		Title:             "LeapView Chat",
		DatastarScriptURL: staticasset.URL(staticasset.DatastarScriptPath),
		HTMLAttrs: []g.Node{
			g.Attr("data-color-mode", "auto"),
			g.Attr("data-light-theme", "light"),
			g.Attr("data-dark-theme", "dark"),
		},
		Head: []g.Node{
			csrfMeta(csrfToken),
			h.Link(h.Rel("icon"), h.Href(staticasset.URL("/static/favicon.svg")), h.Type("image/svg+xml")),
			h.Link(h.Rel("stylesheet"), h.Href(staticasset.URL("/static/app.css"))),
			h.Script(h.Src(staticasset.URL("/static/theme.js"))),
			h.Script(h.Type("module"), h.Src(staticasset.URL("/static/command.js"))),
			h.Script(h.Type("module"), h.Src(staticasset.URL("/static/app-shell.js"))),
			h.Script(h.Type("module"), h.Src(staticasset.URL("/static/chat-page.js"))),
			inspectorScript(),
		},
		MainAttrs:  []g.Node{h.Class("min-h-svh bg-app text-fg-default")},
		UpdatesURL: chatUpdatesURL(workspaceID, view, state.Agent.ActiveConversationID),
		Body: []g.Node{
			g.El("lv-app-shell",
				g.Attr("data-on:lv-chat-reference-search__debounce.200ms", "$agentReferenceSearch.query = evt.detail.query; $agentReferenceSearch.requestId = evt.detail.requestId; "+uiactions.Get("/chats/references/search", "agentReferenceSearch", "agentContext")),
				g.El("lv-chat-page",
					g.Attr("slot", "page"),
					g.Attr("workspace-id", workspaceID),
					g.Attr("view", view),
					g.Attr("data-indicator", "agentTurnPending"),
					g.Attr("data-on:lv-chat-submit", "$agent.composer.value = evt.detail.input; $agentContext.references = evt.detail.references; "+uiactions.Post("/chats/turns", "agent", "agentContext")),
				),
			),
			inspectorElement(),
		},
	})
}

func ChatBootstrapSignals(catalog navigation.Catalog, workspaceID, roleLabel, view string, state ChatViewState) map[string]any {
	return chatInitialSignals(catalog, workspaceID, roleLabel, view, state)
}

func ChatSignalPatch(state ChatViewState) pagestream.SignalPatch {
	patch := ChatConversationsPatch(state.Agent.Conversations, state.Agent.ActiveConversationID)
	patch["agent"] = state.Agent
	patch["visuals"] = state.Visuals
	return patch
}

func ChatConversationsPatch(conversations []ChatConversationSummary, activeConversationID string) pagestream.SignalPatch {
	return pagestream.SignalPatch{
		"agent": map[string]any{"conversations": conversations},
		"chrome": map[string]any{"sidebar": map[string]any{"history": map[string]any{
			"items": chatHistoryItems(ChatSignal{ActiveConversationID: activeConversationID, Conversations: conversations}),
		}}},
	}
}

func chatUpdatesURL(workspaceID, view, conversationID string) string {
	values := url.Values{}
	values.Set("route", string(RouteChat))
	for key, value := range map[string]string{
		"workspace": workspaceID, "view": view, "conversation": conversationID,
	} {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}
	return "/updates?" + values.Encode()
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
	return h.Script(h.Type("module"), h.Src(staticasset.URL("/static/datastar-inspector.js")))
}

func inspectorElement() g.Node {
	if staticasset.Production() {
		return nil
	}
	return g.El("datastar-inspector", g.Attr("signals-url", "/__dev/pagestream/signals"))
}
