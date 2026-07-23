package app

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
)

func (a apiGenAdapter) ListProjects(w http.ResponseWriter, r *http.Request, params apigenapi.GenListProjectsParams) {
	a.server.releaseModule.ListProjects(w, r, params)
}

func (a apiGenAdapter) GetProject(w http.ResponseWriter, r *http.Request, projectID string) {
	a.server.releaseModule.GetProject(w, r, projectID)
}

func (a apiGenAdapter) ListProjectWorkspaces(w http.ResponseWriter, r *http.Request, projectID string, params apigenapi.GenListProjectWorkspacesParams) {
	a.server.releaseModule.ListProjectWorkspaces(w, r, projectID, params)
}

func (a apiGenAdapter) ListManagedConnections(w http.ResponseWriter, r *http.Request, projectID string, params apigenapi.GenListManagedConnectionsParams) {
	a.server.releaseModule.ListManagedConnections(w, r, projectID, params)
}

func (a apiGenAdapter) GetManagedConnection(w http.ResponseWriter, r *http.Request, projectID, connectionID string) {
	a.server.releaseModule.GetManagedConnection(w, r, projectID, connectionID)
}

func (a apiGenAdapter) ListManagedDataUploadSessionEvents(w http.ResponseWriter, r *http.Request, projectID, connectionID, sessionID string, params apigenapi.GenListManagedDataUploadSessionEventsParams, headers apigenapi.GenListManagedDataUploadSessionEventsHeaders) {
	a.server.managedDataModule.ListUploadSessionEvents(w, r, projectID, connectionID, sessionID, params, headers)
}

func (a apiGenAdapter) GetWorkspace(w http.ResponseWriter, r *http.Request, workspaceID string) {
	a.server.workspaceModule.GetWorkspace(w, r, workspaceID)
}

func (a apiGenAdapter) CancelRefreshRun(w http.ResponseWriter, r *http.Request, workspaceID, runID string, headers apigenapi.GenCancelRefreshRunHeaders) {
	a.server.refreshModule.CancelRefreshRun(w, r, workspaceID, runID, headers)
}

func (a apiGenAdapter) ListRefreshRunEvents(w http.ResponseWriter, r *http.Request, workspaceID, runID string, params apigenapi.GenListRefreshRunEventsParams, headers apigenapi.GenListRefreshRunEventsHeaders) {
	a.server.refreshModule.ListRefreshRunEvents(w, r, workspaceID, runID, params, headers)
}
