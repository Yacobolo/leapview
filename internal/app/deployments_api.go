package app

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	deploymentmodule "github.com/Yacobolo/leapview/internal/deployment/module"
)

var errPublicationDeploymentForbidden = deploymentmodule.ErrPublicationForbidden

func (a apiGenAdapter) CreateDeployment(w http.ResponseWriter, r *http.Request, project string, headers apigenapi.GenCreateDeploymentHeaders) {
	a.server.deploymentModule.CreateDeployment(w, r, project, headers)
}

func (a apiGenAdapter) GetDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string) {
	a.server.deploymentModule.GetDeployment(w, r, project, deploymentID)
}

func (a apiGenAdapter) ListDeployments(w http.ResponseWriter, r *http.Request, project string, params apigenapi.GenListDeploymentsParams) {
	a.server.deploymentModule.ListDeployments(w, r, project, params)
}

func (a apiGenAdapter) CancelDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string, headers apigenapi.GenCancelDeploymentHeaders) {
	a.server.deploymentModule.CancelDeployment(w, r, project, deploymentID, headers)
}

func (a apiGenAdapter) RollbackDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string, headers apigenapi.GenRollbackDeploymentHeaders) {
	a.server.deploymentModule.RollbackDeployment(w, r, project, deploymentID, headers)
}

func (a apiGenAdapter) ListDeploymentEvents(w http.ResponseWriter, r *http.Request, project, deploymentID string, params apigenapi.GenListDeploymentEventsParams, headers apigenapi.GenListDeploymentEventsHeaders) {
	a.server.deploymentModule.ListDeploymentEvents(w, r, project, deploymentID, params, headers)
}
