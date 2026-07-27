package module

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	apitransport "github.com/Yacobolo/leapview/internal/api/transport"
	"github.com/Yacobolo/leapview/internal/workspace"
)

func (m *Module) GetWorkspace(w http.ResponseWriter, r *http.Request, workspaceID string) {
	if m == nil || m.handler.ReadModel.WorkspaceRepository == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "WORKSPACE_SERVICE_UNAVAILABLE", "Workspace service is unavailable", nil)
		return
	}
	repo, err := m.handler.ReadModel.WorkspaceRepository()
	if err != nil || repo == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "WORKSPACE_SERVICE_UNAVAILABLE", "Workspace service is unavailable", nil)
		return
	}
	row, err := repo.ByID(r.Context(), workspace.WorkspaceID(workspaceID))
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "Workspace not found", nil)
		return
	}
	item := apigenapi.WorkspaceResponse{
		Id: string(row.ID), Title: row.Title, Description: row.Description,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.ActiveServingStateID != "" {
		value := string(row.ActiveServingStateID)
		item.ActiveServingStateId = &value
	}
	apitransport.WriteJSON(w, http.StatusOK, item)
}
