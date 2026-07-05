package app

import (
	"net/http"

	"github.com/Yacobolo/libredash/internal/ui"
	workspacehttp "github.com/Yacobolo/libredash/internal/workspace/http"
)

func (s *Server) workspaceHTTPHandler() workspacehttp.Handler {
	return workspacehttp.Handler{
		WorkspaceID:          s.workspaceID,
		Environment:          func(r *http.Request) string { return string(s.requestDeploymentEnvironment(r)) },
		WorkspaceRepository:  s.workspaceRepository,
		AccessRepository:     s.accessRepository,
		WorkspaceList:        s.workspaceList,
		WorkspaceAssetsEdges: s.workspaceAssetsAndEdges,
		PlatformAssetsEdges:  s.platformConnectionAssetsAndEdges,
		MetricsForWorkspace:  s.workspaceHTTPMetrics,
		CatalogForWorkspaces: s.catalogForWorkspacesPage,
		RoleBindingsAndRoles: s.roleBindingsAndRoles,
		CatalogForWorkspace:  s.catalogForWorkspace,
		WorkspaceResponse:    s.workspaceResponse,
		CanManageAccess:      s.canManageWorkspaceAccess,
		WorkspaceAccess:      s.workspaceAccessResponse,
		RefreshState:         s.workspaceRefreshSupport(),
		RefreshRunner:        s.workspaceRefreshSupport(),
		Broker:               s.broker,
		CSRFToken:            func(r *http.Request) string { return csrfToken(r, s.auth) },
		CurrentRoleLabel:     s.currentRoleLabel,
		ChromeOptions:        func(r *http.Request) []ui.ChromeOption { return []ui.ChromeOption{s.chatChromeOption(r)} },
	}
}

func (s *Server) workspaceHTTPMetrics(workspaceID string) (workspacehttp.Metrics, bool) {
	metrics, ok := s.metricsForWorkspace(workspaceID)
	if !ok {
		return nil, false
	}
	return metrics, true
}
