package module

import (
	"net/http"

	"github.com/Yacobolo/leapview/internal/agent"
	"github.com/Yacobolo/leapview/internal/ui"
)

func (m *Module) ChromeOption(r *http.Request) ui.ChromeOption {
	return ui.WithChatSidebar(m.ChromeSignal(r))
}

func (m *Module) ChromeSignal(r *http.Request) ui.ChatSignal {
	if m == nil {
		return ui.ChatSignal{}
	}
	scope := agent.Scope{}
	if m.handler != nil {
		scope = m.handler.Scope(r)
	}
	return m.ChatSignalWith(r.Context(), scope, "", nil, agent.ChatArtifactSignals{}, "", false).Agent
}
