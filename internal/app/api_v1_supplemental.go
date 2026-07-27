package app

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
	manageddatamodule "github.com/Yacobolo/leapview/internal/manageddata/module"
	releasemodule "github.com/Yacobolo/leapview/internal/release/module"
)

func (a apiGenDispatcher) ListProjects(w http.ResponseWriter, r *http.Request, params apigenapi.GenListProjectsParams) {
	a.releaseModule.ListProjects(w, r, releasemodule.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}

func (a apiGenDispatcher) GetProject(w http.ResponseWriter, r *http.Request, projectID string) {
	a.releaseModule.GetProject(w, r, projectID)
}

func (a apiGenDispatcher) ListProjectWorkspaces(w http.ResponseWriter, r *http.Request, projectID string, params apigenapi.GenListProjectWorkspacesParams) {
	a.releaseModule.ListProjectWorkspaces(w, r, projectID, releasemodule.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}

func (a apiGenDispatcher) ListManagedConnections(w http.ResponseWriter, r *http.Request, projectID string, params apigenapi.GenListManagedConnectionsParams) {
	a.releaseModule.ListManagedConnections(w, r, projectID, releasemodule.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}

func (a apiGenDispatcher) GetManagedConnection(w http.ResponseWriter, r *http.Request, projectID, connectionID string) {
	a.releaseModule.GetManagedConnection(w, r, projectID, connectionID)
}

func (a apiGenDispatcher) ListManagedDataUploadSessionEvents(w http.ResponseWriter, r *http.Request, projectID, connectionID, sessionID string, params apigenapi.GenListManagedDataUploadSessionEventsParams, headers apigenapi.GenListManagedDataUploadSessionEventsHeaders) {
	a.managedDataModule.ListUploadSessionEvents(
		w, r, projectID, connectionID, sessionID,
		manageddatamodule.PageParams{Limit: params.Limit, PageToken: params.PageToken},
		manageddatamodule.EventHeaders{Accept: headers.Accept, LastEventID: headers.LastEventID},
	)
}

func (a apiGenDispatcher) GetWorkspace(w http.ResponseWriter, r *http.Request, workspaceID string) {
	a.workspaceModule.GetWorkspace(w, r, workspaceID)
}

func (a apiGenDispatcher) CancelRefreshRun(w http.ResponseWriter, r *http.Request, workspaceID, runID string, headers apigenapi.GenCancelRefreshRunHeaders) {
	a.refreshModule.CancelRefreshRun(w, r, workspaceID, runID)
}

func (a apiGenDispatcher) ListRefreshRunEvents(w http.ResponseWriter, r *http.Request, workspaceID, runID string, params apigenapi.GenListRefreshRunEventsParams, headers apigenapi.GenListRefreshRunEventsHeaders) {
	a.refreshModule.ListRefreshRunEvents(w, r, workspaceID, runID, params.Limit, params.PageToken)
}
