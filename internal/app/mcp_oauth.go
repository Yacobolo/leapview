package app

import (
	"encoding/json"
	"net/http"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/ui"
	"github.com/gorilla/csrf"
)

func (s *Server) mcpProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if s.mcpOAuthResource == nil {
		http.Error(w, "MCP OAuth is unavailable", http.StatusServiceUnavailable)
		return
	}
	s.mcpOAuthResource.ProtectedResourceMetadata(w, r)
}

func (s *Server) mcpAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	if s.mcpOAuth == nil {
		http.Error(w, "MCP OAuth is unavailable", http.StatusServiceUnavailable)
		return
	}
	s.mcpOAuth.AuthorizationServerMetadata(w, r)
}

func (s *Server) mcpOAuthRegister(w http.ResponseWriter, r *http.Request) {
	if s.mcpOAuth == nil {
		http.Error(w, "MCP OAuth is unavailable", http.StatusServiceUnavailable)
		return
	}
	s.mcpOAuth.Register(w, r)
}

func (s *Server) mcpOAuthToken(w http.ResponseWriter, r *http.Request) {
	if s.mcpOAuth == nil {
		http.Error(w, "MCP OAuth is unavailable", http.StatusServiceUnavailable)
		return
	}
	s.mcpOAuth.Token(w, r)
}

func (s *Server) mcpOAuthRevoke(w http.ResponseWriter, r *http.Request) {
	if s.mcpOAuth == nil {
		http.Error(w, "MCP OAuth is unavailable", http.StatusServiceUnavailable)
		return
	}
	s.mcpOAuth.Revoke(w, r)
}

func (s *Server) mcpOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if s.mcpOAuth == nil || s.auth == nil {
		http.Error(w, "MCP OAuth is unavailable", http.StatusServiceUnavailable)
		return
	}
	principal, ok := s.auth.Principal(r)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if !principal.DevBypass {
		allowed, err := s.authorizeGlobalAgentPrivilege(r.Context(), principal.ID, nil, access.PrivilegeUseAgent)
		if err != nil {
			writeAuthError(w, r, err, http.StatusInternalServerError)
			return
		}
		if !allowed {
			writeAuthError(w, r, errForbidden, http.StatusForbidden)
			return
		}
	}
	consent, err := s.mcpOAuth.Consent(r)
	if err != nil {
		http.Error(w, "Invalid OAuth authorization request", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodPost {
		approved := r.FormValue("decision") == "approve"
		s.mcpOAuth.Authorize(w, r, principal.ID, approved)
		s.recordMCPOAuthAuthorization(r, principal.ID, consent.ClientID, approved)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := ui.OAuthConsentPage(consent, r.URL.Query(), csrf.Token(r)).Render(w); err != nil {
		s.logger.ErrorContext(r.Context(), "render MCP OAuth consent failed", "error", err)
	}
}

func (s *Server) recordMCPOAuthAuthorization(r *http.Request, principalID, clientID string, approved bool) {
	if s.accessRepo == nil {
		return
	}
	status := "denied"
	if approved {
		status = "success"
	}
	metadata, _ := json.Marshal(map[string]any{"clientId": clientID, "approved": approved})
	_ = access.PersistAuditEvent(r.Context(), s.accessRepo, access.AuditEventInput{
		PrincipalID: principalID, Action: "mcp_oauth.authorization", TargetType: "oauth_client",
		TargetID: clientID, Privilege: access.PrivilegeUseAgent, Status: status,
		RequestID: r.Header.Get("X-Request-ID"), CorrelationID: r.Header.Get("X-Correlation-ID"),
		MetadataJSON: string(metadata),
	})
}
