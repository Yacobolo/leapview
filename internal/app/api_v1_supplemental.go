package app

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
	manageddatamodule "github.com/Yacobolo/leapview/internal/manageddata/module"
	releasemodule "github.com/Yacobolo/leapview/internal/release/module"
)

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
