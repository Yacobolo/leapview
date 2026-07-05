package app

import (
	"net/http"

	workspacehttp "github.com/Yacobolo/libredash/internal/workspace/http"
)

func (s *Server) workspaceHTTPHandler() workspacehttp.Handler {
	return workspacehttp.Handler{
		WorkspaceID:            s.workspaceID,
		Environment:            func(r *http.Request) string { return string(s.requestDeploymentEnvironment(r)) },
		WorkspaceRepository:    s.workspaceRepository,
		AccessRepository:       s.accessRepository,
		WorkspaceList:          s.workspaceList,
		WorkspaceAssetsEdges:   s.workspaceAssetsAndEdges,
		RoleBindingsAndRoles:   s.roleBindingsAndRoles,
		CatalogForWorkspace:    s.catalogForWorkspace,
		WorkspaceResponse:      s.workspaceResponse,
		CSRFToken:              func(r *http.Request) string { return csrfToken(r, s.auth) },
		CurrentRoleLabel:       s.currentRoleLabel,
		WorkspaceCatalogPage:   s.renderWorkspacesPage,
		WorkspaceAssetsPage:    s.renderWorkspaceAssetsPage,
		WorkspaceAssetPage:     s.renderWorkspaceAssetRedirect,
		WorkspaceAssetDetail:   s.renderWorkspaceAssetSection,
		ConnectionsPage:        s.renderConnectionsPage,
		ConnectionSourcePage:   s.renderConnectionSourceAssetRedirect,
		ConnectionSourceDetail: s.renderConnectionSourceAssetSection,
		ConnectionAssetPage:    s.renderConnectionAssetRedirect,
		ConnectionAssetDetail:  s.renderConnectionAssetSection,
		AssetUpdates:           s.assetUpdatesStream,
		AssetRefresh:           s.assetRefreshPost,
		AssetMaterialize:       s.assetRefreshMaterializationsPost,
	}
}
