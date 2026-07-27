package app

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
)

func (a apiGenDispatcher) ListDashboardPublications(w http.ResponseWriter, r *http.Request, workspaceID string) {
	a.dashboardModule.ListDashboardPublications(w, r, workspaceID)
}

func (a apiGenDispatcher) GetDashboardPublication(w http.ResponseWriter, r *http.Request, workspaceID, name string) {
	a.dashboardModule.GetDashboardPublication(w, r, workspaceID, name)
}

func (a apiGenDispatcher) SuspendDashboardPublication(w http.ResponseWriter, r *http.Request, workspaceID, name string, headers apigenapi.GenSuspendDashboardPublicationHeaders) {
	a.dashboardModule.SuspendDashboardPublication(w, r, workspaceID, name)
}

func (a apiGenDispatcher) ResumeDashboardPublication(w http.ResponseWriter, r *http.Request, workspaceID, name string, headers apigenapi.GenResumeDashboardPublicationHeaders) {
	a.dashboardModule.ResumeDashboardPublication(w, r, workspaceID, name)
}

func (a apiGenDispatcher) RotateDashboardPublication(w http.ResponseWriter, r *http.Request, workspaceID, name string, headers apigenapi.GenRotateDashboardPublicationHeaders) {
	a.dashboardModule.RotateDashboardPublication(w, r, workspaceID, name)
}
