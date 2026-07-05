package app

import (
	"context"
	"net/http"

	"github.com/Yacobolo/libredash/internal/access"
	agenthttp "github.com/Yacobolo/libredash/internal/agent/http"
)

func (s *Server) agentHTTPHandler() *agenthttp.Handler {
	var settings agenthttp.Settings
	if s.store != nil {
		settings = s.store
	}
	return agenthttp.NewHandler(agenthttp.Options{
		Service:             s.agent,
		Settings:            settings,
		DefaultWorkspace:    s.defaultWorkspaceID,
		WorkspaceID:         s.workspaceID,
		Broker:              s.broker,
		CatalogForWorkspace: s.catalogForWorkspace,
		CSRFToken: func(r *http.Request) string {
			return csrfToken(r, s.auth)
		},
		CurrentRoleLabel:       s.currentRoleLabel,
		ChatSignal:             s.chatSignal,
		ChatSignalWith:         s.chatSignalWith,
		QueueMissingTitle:      s.queueMissingChatTitle,
		ExecuteStartedChatTurn: s.executeStartedChatTurn,
		CurrentPrincipal: func(r *http.Request) (agenthttp.Principal, bool) {
			if s.auth == nil {
				return agenthttp.Principal{}, false
			}
			principal, ok := s.auth.Principal(r)
			return agenthttp.Principal{ID: principal.ID, DevAuthBypass: principal.DevBypass}, ok
		},
		CurrentCredential: func(r *http.Request) (access.APICredential, bool) {
			if s.auth == nil {
				return access.APICredential{}, false
			}
			return s.auth.APICredential(r)
		},
	})
}

func (s *Server) agentSystemPrompt(ctx context.Context) (string, error) {
	return s.agentHTTPHandler().SystemPrompt(ctx)
}
