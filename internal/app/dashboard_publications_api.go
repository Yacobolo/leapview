package app

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
)

func (a apiGenAdapter) ListDashboardPublications(w http.ResponseWriter, r *http.Request, workspaceID string) {
	a.server.dashboardModule.ListDashboardPublications(w, r, workspaceID)
}

func (a apiGenAdapter) GetDashboardPublication(w http.ResponseWriter, r *http.Request, workspaceID, name string) {
	a.server.dashboardModule.GetDashboardPublication(w, r, workspaceID, name)
}

func (a apiGenAdapter) SuspendDashboardPublication(w http.ResponseWriter, r *http.Request, workspaceID, name string, headers apigenapi.GenSuspendDashboardPublicationHeaders) {
	a.server.dashboardModule.SuspendDashboardPublication(w, r, workspaceID, name, headers)
}

func (a apiGenAdapter) ResumeDashboardPublication(w http.ResponseWriter, r *http.Request, workspaceID, name string, headers apigenapi.GenResumeDashboardPublicationHeaders) {
	a.server.dashboardModule.ResumeDashboardPublication(w, r, workspaceID, name, headers)
}

func (a apiGenAdapter) RotateDashboardPublication(w http.ResponseWriter, r *http.Request, workspaceID, name string, headers apigenapi.GenRotateDashboardPublicationHeaders) {
	a.server.dashboardModule.RotateDashboardPublication(w, r, workspaceID, name, headers)
}
