package app

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	deploymentmodule "github.com/Yacobolo/leapview/internal/deployment/module"
)

var errPublicationDeploymentForbidden = deploymentmodule.ErrPublicationForbidden

func (a apiGenDispatcher) CreateDeployment(w http.ResponseWriter, r *http.Request, project string, headers apigenapi.GenCreateDeploymentHeaders) {
	a.deploymentModule.CreateDeployment(w, r, project, headers)
}

func (a apiGenDispatcher) GetDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string) {
	a.deploymentModule.GetDeployment(w, r, project, deploymentID)
}

func (a apiGenDispatcher) ListDeployments(w http.ResponseWriter, r *http.Request, project string, params apigenapi.GenListDeploymentsParams) {
	a.deploymentModule.ListDeployments(w, r, project, params)
}

func (a apiGenDispatcher) CancelDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string, headers apigenapi.GenCancelDeploymentHeaders) {
	a.deploymentModule.CancelDeployment(w, r, project, deploymentID, headers)
}

func (a apiGenDispatcher) RollbackDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string, headers apigenapi.GenRollbackDeploymentHeaders) {
	a.deploymentModule.RollbackDeployment(w, r, project, deploymentID, headers)
}

func (a apiGenDispatcher) ListDeploymentEvents(w http.ResponseWriter, r *http.Request, project, deploymentID string, params apigenapi.GenListDeploymentEventsParams, headers apigenapi.GenListDeploymentEventsHeaders) {
	a.deploymentModule.ListDeploymentEvents(w, r, project, deploymentID, params, headers)
}
