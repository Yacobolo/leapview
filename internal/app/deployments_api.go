package app

import (
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
	deploymentmodule "github.com/Yacobolo/leapview/internal/deployment/module"
)

var errPublicationDeploymentForbidden = deploymentmodule.ErrPublicationForbidden

func (a apiGenDispatcher) CreateDeployment(w http.ResponseWriter, r *http.Request, project string, headers apigenapi.GenCreateDeploymentHeaders) {
	a.deploymentModule.CreateDeployment(w, r, project, headers.IdempotencyKey)
}

func (a apiGenDispatcher) GetDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string) {
	a.deploymentModule.GetDeployment(w, r, project, deploymentID)
}

func (a apiGenDispatcher) ListDeployments(w http.ResponseWriter, r *http.Request, project string, params apigenapi.GenListDeploymentsParams) {
	a.deploymentModule.ListDeployments(w, r, project, deploymentmodule.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}

func (a apiGenDispatcher) CancelDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string, headers apigenapi.GenCancelDeploymentHeaders) {
	a.deploymentModule.CancelDeployment(w, r, project, deploymentID)
}

func (a apiGenDispatcher) RollbackDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string, headers apigenapi.GenRollbackDeploymentHeaders) {
	a.deploymentModule.RollbackDeployment(w, r, project, deploymentID, headers.IdempotencyKey)
}

func (a apiGenDispatcher) ListDeploymentEvents(w http.ResponseWriter, r *http.Request, project, deploymentID string, params apigenapi.GenListDeploymentEventsParams, headers apigenapi.GenListDeploymentEventsHeaders) {
	a.deploymentModule.ListDeploymentEvents(w, r, project, deploymentID, deploymentmodule.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}
