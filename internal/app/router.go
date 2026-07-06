package app

import (
	"context"
	"net/http"
	"strings"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/access/scimprov"
	agenthttp "github.com/Yacobolo/libredash/internal/agent/http"
	"github.com/go-chi/chi/v5"
)

func (s *Server) Routes() http.Handler {
	mux := chi.NewRouter()
	if s.requestLogging {
		mux.Use(requestLogger(s.logger))
	}
	mux.Use(securityHeaders(s.securityHeaders))
	mux.Get("/favicon.ico", favicon)
	mux.Get("/login", s.login)
	mux.Group(func(r chi.Router) {
		r.Use(s.csrf)
		r.Get("/", s.protected(access.PrivilegeViewItem, s.home))
		workspaceHTTP := s.workspaceHTTPHandler()
		r.Get("/data", s.protected(access.PrivilegeViewItem, workspaceHTTP.DataExplorer))
		r.Get("/data/updates", s.protected(access.PrivilegeViewItem, workspaceHTTP.DataExplorerUpdates))
		r.Post("/data/command", s.protected(access.PrivilegeViewItem, workspaceHTTP.DataExplorerCommand))
		r.Get("/workspaces", s.protected(access.PrivilegeViewItem, workspaceHTTP.WorkspaceCatalog))
		r.Get("/workspaces/{workspace}", s.protected(access.PrivilegeViewItem, workspaceHTTP.WorkspaceAssets))
		r.Get("/workspaces/{workspace}/assets/{asset}", s.protected(access.PrivilegeViewItem, workspaceHTTP.WorkspaceAsset))
		r.Get("/workspaces/{workspace}/assets/{asset}/updates", s.protected(access.PrivilegeViewItem, workspaceHTTP.AssetUpdatesStream))
		r.Get("/workspaces/{workspace}/assets/{asset}/{section}", s.protected(access.PrivilegeViewItem, workspaceHTTP.WorkspaceAssetSection))
		r.Post("/workspaces/{workspace}/assets/{asset}/refresh", s.protected(access.PrivilegeRefreshData, workspaceHTTP.RefreshAsset))
		r.Post("/workspaces/{workspace}/assets/{asset}/refresh-materializations", s.protected(access.PrivilegeRefreshData, workspaceHTTP.RefreshAssetMaterializations))
		r.Get("/workspaces/{workspace}/data", s.protected(access.PrivilegeViewItem, workspaceHTTP.WorkspaceDataExplorerRedirect))
		agentHTTP := s.agentHTTPHandler()
		r.Get("/chat", s.protected(access.PrivilegeViewAgent, agentHTTP.Chat))
		r.Get("/chat/new", s.protected(access.PrivilegeViewAgent, agentHTTP.ChatNew))
		r.Get("/chat/updates", s.protected(access.PrivilegeViewAgent, agentHTTP.ChatUpdates))
		r.Get("/chat/{conversation}", s.protected(access.PrivilegeViewAgent, agentHTTP.ChatConversation))
		r.Post("/chat/turns", s.protected(access.PrivilegeUseAgent, agentHTTP.ChatTurn))
		r.Get("/workspaces/{workspace}/chat", s.protected(access.PrivilegeViewItem, agenthttp.LegacyChatRedirect))
		r.Get("/workspaces/{workspace}/chat/new", s.protected(access.PrivilegeViewItem, agenthttp.LegacyChatRedirect))
		r.Get("/workspaces/{workspace}/chat/updates", s.protected(access.PrivilegeViewItem, agenthttp.LegacyChatRedirect))
		r.Get("/workspaces/{workspace}/chat/{conversation}", s.protected(access.PrivilegeViewItem, agenthttp.LegacyChatRedirect))
		r.Post("/workspaces/{workspace}/chat/turns", s.protected(access.PrivilegeViewItem, agenthttp.LegacyChatRedirect))
		adminHTTP := s.adminHTTPHandler()
		r.Get("/admin", s.protected(access.PrivilegeManageGrants, adminHTTP.General))
		r.Get("/admin/principals", s.protected(access.PrivilegeManageGrants, adminHTTP.Principals))
		r.Get("/admin/principals/{principal}", s.protected(access.PrivilegeManageGrants, adminHTTP.PrincipalDetail))
		r.Get("/admin/groups", s.protected(access.PrivilegeManageGrants, adminHTTP.Groups))
		r.Get("/admin/groups/{group}", s.protected(access.PrivilegeManageGrants, adminHTTP.GroupDetail))
		r.Get("/admin/agent", s.protected(access.PrivilegeManageGrants, adminHTTP.Agent))
		r.Get("/admin/storage", s.protected(access.PrivilegeManageGrants, adminHTTP.Storage))
		r.Get("/admin/storage/updates", s.protected(access.PrivilegeManageGrants, adminHTTP.StorageSignalUpdates))
		r.Post("/admin/storage/select-table", s.protected(access.PrivilegeManageGrants, adminHTTP.StorageTableSelect))
		r.Get("/admin/queries", s.protected(access.PrivilegeViewAudit, adminHTTP.Queries))
		r.Get("/admin/queries/updates", s.protected(access.PrivilegeViewAudit, adminHTTP.QueryUpdates))
		r.Post("/admin/queries/command", s.protected(access.PrivilegeViewAudit, adminHTTP.QueryCommand))
		r.Post("/workspaces/{workspace}/access/upsert", s.protected(access.PrivilegeManageGrants, workspaceHTTP.AccessUpsert))
		r.Post("/workspaces/{workspace}/access/remove", s.protected(access.PrivilegeManageGrants, workspaceHTTP.AccessRemove))
		r.Post("/workspaces/{workspace}/assets/{asset}/access/upsert", s.protected(access.PrivilegeManageGrants, workspaceHTTP.AccessUpsert))
		r.Post("/workspaces/{workspace}/assets/{asset}/access/remove", s.protected(access.PrivilegeManageGrants, workspaceHTTP.AccessRemove))
		r.Get("/connections", s.protected(access.PrivilegeViewItem, workspaceHTTP.Connections))
		r.Get("/connections/{connection}/sources/{source}", s.protected(access.PrivilegeViewItem, workspaceHTTP.ConnectionSource))
		r.Get("/connections/{connection}/sources/{source}/{section}", s.protected(access.PrivilegeViewItem, workspaceHTTP.ConnectionSourceSection))
		r.Get("/connections/{asset}", s.protected(access.PrivilegeViewItem, workspaceHTTP.ConnectionAsset))
		r.Get("/connections/{asset}/{section}", s.protected(access.PrivilegeViewItem, workspaceHTTP.ConnectionAssetSection))
		dashboardHTTP := s.dashboardHTTP()
		r.Get("/workspaces/{workspace}/dashboards/{dashboard}", s.protected(access.PrivilegeViewItem, dashboardHTTP.Dashboard))
		r.Get("/workspaces/{workspace}/dashboards/{dashboard}/pages/{page}", s.protected(access.PrivilegeViewItem, dashboardHTTP.Page))
		r.With(s.rateLimits.updatesMiddleware()).Get("/workspaces/{workspace}/updates", s.protected(access.PrivilegeViewItem, dashboardHTTP.Updates))
		r.Post("/workspaces/{workspace}/commands/table-window", s.protected(access.PrivilegeViewItem, dashboardHTTP.TableWindow))
		r.Post("/workspaces/{workspace}/commands/select", s.protected(access.PrivilegeViewItem, dashboardHTTP.Select))
		r.Post("/workspaces/{workspace}/commands/clear-selection", s.protected(access.PrivilegeViewItem, dashboardHTTP.ClearSelection))
		r.Post("/workspaces/{workspace}/commands/reset-filters", s.protected(access.PrivilegeViewItem, dashboardHTTP.ResetFilters))
		r.Post("/auth/logout", s.authLogout)
	})
	mux.Group(func(r chi.Router) {
		r.Use(s.rateLimits.authMiddleware())
		r.Get("/auth/{provider}", s.authBegin)
		r.Get("/auth/{provider}/callback", s.authCallback)
		r.Post("/oauth/token", s.accessHTTPHandler().OAuthToken)
	})
	if s.store != nil {
		if strings.TrimSpace(s.scimBearerToken) != "" {
			if repo, err := s.accessRepository(); err == nil && repo != nil {
				if handler, err := scimprov.NewHandler(scimprov.Options{Repository: repo, BearerToken: s.scimBearerToken}); err == nil {
					mux.Handle("/scim/*", http.StripPrefix("/scim", handler))
				}
			}
		}
		mux.Group(func(r chi.Router) {
			r.Use(s.rateLimits.apiMiddleware())
			r.Use(s.csrf)
			agentHTTP := s.agentHTTPHandler()
			r.Get("/api/v1/agent/conversations", s.protected(access.PrivilegeViewAgent, agentHTTP.ListConversations))
			r.Post("/api/v1/agent/conversations", s.protected(access.PrivilegeUseAgent, agentHTTP.CreateConversation))
			r.Get("/api/v1/agent/conversations/{conversation}", s.protected(access.PrivilegeViewAgent, agentHTTP.GetConversation))
			r.Patch("/api/v1/agent/conversations/{conversation}", s.protected(access.PrivilegeUseAgent, agentHTTP.UpdateConversation))
			r.Delete("/api/v1/agent/conversations/{conversation}", s.protected(access.PrivilegeUseAgent, agentHTTP.ArchiveConversation))
			r.Get("/api/v1/agent/conversations/{conversation}/messages", s.protected(access.PrivilegeViewAgent, agentHTTP.ListMessages))
			r.Get("/api/v1/agent/conversations/{conversation}/runs", s.protected(access.PrivilegeViewAgent, agentHTTP.ListRuns))
			r.Get("/api/v1/agent/conversations/{conversation}/runs/{run}", s.protected(access.PrivilegeViewAgent, agentHTTP.GetRun))
			r.Get("/api/v1/agent/conversations/{conversation}/runs/{run}/events", s.protected(access.PrivilegeViewAgent, agentHTTP.ListEvents))
			r.Post("/api/v1/agent/conversations/{conversation}/turns", s.protected(access.PrivilegeUseAgent, agentHTTP.CreateTurn))
			s.registerAPIGenRoutes(r)
		})
	}
	mux.Handle("/static/*", noCache(http.StripPrefix("/static/", http.FileServer(http.Dir("static")))))

	return mux
}

func (s *Server) protected(privilege access.Privilege, handler http.HandlerFunc) http.HandlerFunc {
	return s.protect(privilege, handler).ServeHTTP
}

func (s *Server) protect(privilege access.Privilege, next http.Handler) http.Handler {
	if s.auth == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), principalContextKey{}, localDeveloperPrincipal())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	return s.auth.Middleware(privilege, next)
}

func (s *Server) csrf(next http.Handler) http.Handler {
	if s.auth == nil {
		return next
	}
	return s.auth.CSRFMiddleware(next)
}

func (s *Server) authBegin(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		http.NotFound(w, r)
		return
	}
	s.auth.Begin(w, r)
}

func (s *Server) authCallback(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		http.NotFound(w, r)
		return
	}
	s.auth.Callback(w, r)
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		http.NotFound(w, r)
		return
	}
	s.auth.Logout(w, r)
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func favicon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="#0969da"/><path d="M8 22h16v3H8zm1-5h4v4H9zm5-7h4v11h-4zm5 4h4v7h-4z" fill="#fff"/></svg>`))
}
