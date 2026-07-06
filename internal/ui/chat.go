package ui

import (
	"github.com/Yacobolo/libredash/internal/dashboard"
	uisignals "github.com/Yacobolo/libredash/internal/ui/signals"
	"github.com/Yacobolo/libredash/pkg/pagestream"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func ChatPage(catalog dashboard.Catalog, workspaceID, csrfToken, roleLabel, view string, signal ChatSignal) g.Node {
	envelope := uisignals.ChatInitialEnvelope(catalog, workspaceID, csrfToken, roleLabel, view, signal)
	chatUpdatesURL := updatesURL(uisignals.RouteChat)
	envelope.Runtime = runtimeSignal(uisignals.RouteChat, chatUpdatesURL)
	envelope.Runtime.WorkspaceID = workspaceID
	chatBasePath := "/chat"
	return pagestream.RenderPage(pagestream.PageSpec{
		Title: "LibreDash Chat",
		HTMLAttrs: []g.Node{
			g.Attr("data-color-mode", "auto"),
			g.Attr("data-light-theme", "light"),
			g.Attr("data-dark-theme", "dark"),
		},
		Head: pageHead(
			h.Script(h.Type("module"), h.Src(staticAsset("/static/app-shell.js"))),
			h.Script(h.Type("module"), h.Src(staticAsset("/static/chat-page.js"))),
			inspectorScript(),
		),
		MainAttrs: []g.Node{h.Class(appRootClass)},
		Signals: map[string]any{
			"chrome":    envelope.Chrome,
			"page":      envelope.Page,
			"runtime":   envelope.Runtime,
			"csrfToken": envelope.CSRFToken,
			"agent":     envelope.Agent,
			"visuals":   envelope.Visuals,
			"tables":    envelope.Tables,
		},
		UpdatesURL: chatUpdatesURL,
		Body: []g.Node{
			g.El("ld-app-shell",
				g.El("ld-chat-page",
					g.Attr("slot", "page"),
					g.Attr("data-indicator", "agentTurnPending"),
					g.Attr("data-on:ld-chat-submit", "$agent.composer.value = evt.detail.input; "+postAction(chatBasePath+"/turns")),
				),
			),
			inspectorElement(),
		},
	})
}
