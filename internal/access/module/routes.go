package module

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (m *Module) MountLoginPage(r chi.Router) {
	if m != nil {
		r.Get("/login", m.Login)
	}
}

func (m *Module) MountAuthenticatedBrowser(r chi.Router) {
	if m == nil {
		return
	}
	r.Post("/auth/logout", m.Logout)
	r.Post("/auth/local/password", m.LocalPassword)
}

func (m *Module) MountLocalLogin(r chi.Router) {
	if m != nil {
		r.Post("/auth/local/login", m.LocalLogin)
	}
}

func (m *Module) MountOAuthEndpoints(r chi.Router) {
	if m == nil {
		return
	}
	r.Get("/auth/{provider}", m.Begin)
	r.Get("/auth/{provider}/callback", m.Callback)
	r.Post("/oauth/token", m.OAuthToken)
	r.Post("/oauth/register", m.MCPOAuthRegister)
	r.Post("/oauth/revoke", m.MCPOAuthRevoke)
}

func (m *Module) MountOAuthMetadata(r chi.Router) {
	if m == nil {
		return
	}
	r.Get("/.well-known/oauth-protected-resource", m.MCPProtectedResourceMetadata)
	r.Get("/.well-known/oauth-protected-resource/mcp", m.MCPProtectedResourceMetadata)
	r.Get("/.well-known/oauth-authorization-server", m.MCPAuthorizationServerMetadata)
	if m.auth != nil {
		authorize := m.auth.Middleware("", http.HandlerFunc(m.MCPOAuthAuthorize))
		r.Method(http.MethodGet, "/oauth/authorize", m.CSRFMiddleware(authorize))
		r.Method(http.MethodPost, "/oauth/authorize", m.CSRFMiddleware(authorize))
	}
}
