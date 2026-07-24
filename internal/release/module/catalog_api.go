package module

import (
	"net/http"

	apitransport "github.com/Yacobolo/leapview/internal/platform/http/transport"
	releaseapi "github.com/Yacobolo/leapview/internal/release/api"
)

func (m *Module) ListProjects(w http.ResponseWriter, r *http.Request, params releaseapi.PageParams) {
	if m == nil || m.catalog == nil {
		apitransport.WriteJSON(w, http.StatusOK, releaseapi.ProjectListResponse{Items: []releaseapi.ProjectResponse{}, Page: releaseapi.PageInfo{}})
		return
	}
	rows, err := m.catalog.ListProjects(r.Context())
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusInternalServerError, "PROJECT_LIST_FAILED", "Projects could not be loaded", nil)
		return
	}
	items := make([]releaseapi.ProjectResponse, 0, len(rows))
	for _, row := range rows {
		item := releaseapi.ProjectResponse{ID: row.ID, Title: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
		if row.LatestReleaseID != "" {
			item.LatestReleaseID = &row.LatestReleaseID
		}
		if row.ActiveDeploymentID != "" {
			item.ActiveDeploymentID = &row.ActiveDeploymentID
		}
		items = append(items, item)
	}
	page, next, err := apitransport.KeysetPage(items, params.Limit, params.PageToken, func(item releaseapi.ProjectResponse) string { return item.ID })
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_CURSOR", err.Error(), nil)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, releaseapi.ProjectListResponse{Items: page, Page: releaseapi.PageInfo{NextCursor: next}})
}

func (m *Module) GetProject(w http.ResponseWriter, r *http.Request, projectID string) {
	if m == nil || m.catalog == nil {
		apitransport.WriteProblem(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		return
	}
	row, err := m.catalog.GetProject(r.Context(), projectID)
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		return
	}
	item := releaseapi.ProjectResponse{ID: projectID, Title: projectID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if row.LatestReleaseID != "" {
		item.LatestReleaseID = &row.LatestReleaseID
	}
	if row.ActiveDeploymentID != "" {
		item.ActiveDeploymentID = &row.ActiveDeploymentID
	}
	apitransport.WriteJSON(w, http.StatusOK, item)
}

func (m *Module) ListProjectWorkspaces(w http.ResponseWriter, r *http.Request, projectID string, params releaseapi.PageParams) {
	if m == nil || m.catalog == nil {
		apitransport.WriteProblem(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		return
	}
	rows, err := m.catalog.ListProjectWorkspaces(r.Context(), projectID, m.environment)
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusInternalServerError, "PROJECT_WORKSPACES_FAILED", "Project workspaces could not be loaded", nil)
		return
	}
	items := make([]releaseapi.ProjectWorkspaceResponse, 0, len(rows))
	for _, row := range rows {
		item := releaseapi.ProjectWorkspaceResponse{ID: row.ID, Title: row.Title}
		if row.Description != "" {
			item.Description = &row.Description
		}
		if row.ActiveServingStateID != "" {
			item.ActiveServingStateID = &row.ActiveServingStateID
		}
		items = append(items, item)
	}
	page, next, err := apitransport.KeysetPage(items, params.Limit, params.PageToken, func(item releaseapi.ProjectWorkspaceResponse) string { return item.ID })
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_CURSOR", err.Error(), nil)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, releaseapi.ProjectWorkspaceListResponse{Items: page, Page: releaseapi.PageInfo{NextCursor: next}})
}

func (m *Module) ListManagedConnections(w http.ResponseWriter, r *http.Request, projectID string, params releaseapi.PageParams) {
	if m == nil || m.catalog == nil {
		apitransport.WriteJSON(w, http.StatusOK, releaseapi.ManagedConnectionListResponse{Items: []releaseapi.ManagedConnectionResponse{}, Page: releaseapi.PageInfo{}})
		return
	}
	rows, err := m.catalog.ListConnections(r.Context(), projectID, m.environment)
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusInternalServerError, "CONNECTION_LIST_FAILED", "Connections could not be loaded", nil)
		return
	}
	items := make([]releaseapi.ManagedConnectionResponse, 0, len(rows))
	for _, row := range rows {
		item := releaseapi.ManagedConnectionResponse{ID: row.ID, ProjectID: projectID, Title: row.Title}
		if row.Description != "" {
			item.Description = &row.Description
		}
		if row.ActiveRevisionID != "" {
			item.ActiveRevisionID = &row.ActiveRevisionID
		}
		items = append(items, item)
	}
	page, next, err := apitransport.KeysetPage(items, params.Limit, params.PageToken, func(item releaseapi.ManagedConnectionResponse) string { return item.ID })
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_CURSOR", err.Error(), nil)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, releaseapi.ManagedConnectionListResponse{Items: page, Page: releaseapi.PageInfo{NextCursor: next}})
}

func (m *Module) GetManagedConnection(w http.ResponseWriter, r *http.Request, projectID, connectionID string) {
	if m == nil || m.catalog == nil {
		apitransport.WriteProblem(w, r, http.StatusNotFound, "CONNECTION_NOT_FOUND", "Connection not found", nil)
		return
	}
	row, err := m.catalog.GetConnection(r.Context(), projectID, connectionID, m.environment)
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusNotFound, "CONNECTION_NOT_FOUND", "Connection not found", nil)
		return
	}
	item := releaseapi.ManagedConnectionResponse{ID: connectionID, ProjectID: projectID, Title: row.Title}
	if row.Description != "" {
		item.Description = &row.Description
	}
	if row.ActiveRevisionID != "" {
		item.ActiveRevisionID = &row.ActiveRevisionID
	}
	apitransport.WriteJSON(w, http.StatusOK, item)
}

func (m *Module) ProjectCursorSnapshot(r *http.Request, projectID string) string {
	if m == nil || m.catalog == nil {
		return ""
	}
	row, err := m.catalog.GetProject(r.Context(), projectID)
	if err != nil {
		return ""
	}
	if row.ActiveDeploymentID != "" {
		return "deployment:" + row.ActiveDeploymentID
	}
	if row.LatestReleaseID != "" {
		return "release:" + row.LatestReleaseID
	}
	return ""
}
