package module

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	apitransport "github.com/Yacobolo/leapview/internal/api/transport"
)

func (m *Module) ListProjects(w http.ResponseWriter, r *http.Request, params apigenapi.GenListProjectsParams) {
	if m == nil || m.catalog == nil {
		apitransport.WriteJSON(w, http.StatusOK, apigenapi.ProjectListResponse{Items: []apigenapi.ProjectResponse{}, Page: apigenapi.PageInfo{}})
		return
	}
	rows, err := m.catalog.ListProjects(r.Context())
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusInternalServerError, "PROJECT_LIST_FAILED", "Projects could not be loaded", nil)
		return
	}
	items := make([]apigenapi.ProjectResponse, 0, len(rows))
	for _, row := range rows {
		item := apigenapi.ProjectResponse{Id: row.ID, Title: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
		if row.LatestReleaseID != "" {
			item.LatestReleaseId = &row.LatestReleaseID
		}
		if row.ActiveDeploymentID != "" {
			item.ActiveDeploymentId = &row.ActiveDeploymentID
		}
		items = append(items, item)
	}
	page, next, err := apitransport.KeysetPage(items, params.Limit, params.PageToken, func(item apigenapi.ProjectResponse) string { return item.Id })
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_CURSOR", err.Error(), nil)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, apigenapi.ProjectListResponse{Items: page, Page: apigenapi.PageInfo{NextCursor: next}})
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
	item := apigenapi.ProjectResponse{Id: projectID, Title: projectID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if row.LatestReleaseID != "" {
		item.LatestReleaseId = &row.LatestReleaseID
	}
	if row.ActiveDeploymentID != "" {
		item.ActiveDeploymentId = &row.ActiveDeploymentID
	}
	apitransport.WriteJSON(w, http.StatusOK, item)
}

func (m *Module) ListProjectWorkspaces(w http.ResponseWriter, r *http.Request, projectID string, params apigenapi.GenListProjectWorkspacesParams) {
	if m == nil || m.catalog == nil {
		apitransport.WriteProblem(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		return
	}
	rows, err := m.catalog.ListProjectWorkspaces(r.Context(), projectID, m.environment)
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusInternalServerError, "PROJECT_WORKSPACES_FAILED", "Project workspaces could not be loaded", nil)
		return
	}
	items := make([]apigenapi.ProjectWorkspaceResponse, 0, len(rows))
	for _, row := range rows {
		item := apigenapi.ProjectWorkspaceResponse{Id: row.ID, Title: row.Title}
		if row.Description != "" {
			item.Description = &row.Description
		}
		if row.ActiveServingStateID != "" {
			item.ActiveServingStateId = &row.ActiveServingStateID
		}
		items = append(items, item)
	}
	page, next, err := apitransport.KeysetPage(items, params.Limit, params.PageToken, func(item apigenapi.ProjectWorkspaceResponse) string { return item.Id })
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_CURSOR", err.Error(), nil)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, apigenapi.ProjectWorkspaceListResponse{Items: page, Page: apigenapi.PageInfo{NextCursor: next}})
}

func (m *Module) ListManagedConnections(w http.ResponseWriter, r *http.Request, projectID string, params apigenapi.GenListManagedConnectionsParams) {
	if m == nil || m.catalog == nil {
		apitransport.WriteJSON(w, http.StatusOK, apigenapi.ManagedConnectionListResponse{Items: []apigenapi.ManagedConnectionResponse{}, Page: apigenapi.PageInfo{}})
		return
	}
	rows, err := m.catalog.ListConnections(r.Context(), projectID, m.environment)
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusInternalServerError, "CONNECTION_LIST_FAILED", "Connections could not be loaded", nil)
		return
	}
	items := make([]apigenapi.ManagedConnectionResponse, 0, len(rows))
	for _, row := range rows {
		item := apigenapi.ManagedConnectionResponse{Id: row.ID, ProjectId: projectID, Title: row.Title}
		if row.Description != "" {
			item.Description = &row.Description
		}
		if row.ActiveRevisionID != "" {
			item.ActiveRevisionId = &row.ActiveRevisionID
		}
		items = append(items, item)
	}
	page, next, err := apitransport.KeysetPage(items, params.Limit, params.PageToken, func(item apigenapi.ManagedConnectionResponse) string { return item.Id })
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_CURSOR", err.Error(), nil)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, apigenapi.ManagedConnectionListResponse{Items: page, Page: apigenapi.PageInfo{NextCursor: next}})
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
	item := apigenapi.ManagedConnectionResponse{Id: connectionID, ProjectId: projectID, Title: row.Title}
	if row.Description != "" {
		item.Description = &row.Description
	}
	if row.ActiveRevisionID != "" {
		item.ActiveRevisionId = &row.ActiveRevisionID
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
