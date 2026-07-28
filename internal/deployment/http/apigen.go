package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"

	deploymentgen "github.com/Yacobolo/leapview/internal/deployment/api/gen"
	apitransport "github.com/Yacobolo/leapview/internal/platform/http/transport"
)

type APIGenHandler interface {
	ListDeployments(stdhttp.ResponseWriter, *stdhttp.Request, string, *int32, *string)
	CreateDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	GetDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	CancelDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	ListDeploymentEvents(stdhttp.ResponseWriter, *stdhttp.Request, string, string, *int32, *string)
	RollbackDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
}

type APIGenDispatcher struct{ handler APIGenHandler }

func NewAPIGenDispatcher(handler APIGenHandler) *APIGenDispatcher {
	return &APIGenDispatcher{handler: handler}
}

func (d *APIGenDispatcher) ListDeployments(w stdhttp.ResponseWriter, r *stdhttp.Request, project string, params deploymentgen.GenListDeploymentsParams) {
	d.handler.ListDeployments(w, r, project, params.Limit, params.PageToken)
}

func (d *APIGenDispatcher) CreateDeployment(w stdhttp.ResponseWriter, r *stdhttp.Request, project string, headers deploymentgen.GenCreateDeploymentHeaders) {
	d.handler.CreateDeployment(w, r, project, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) GetDeployment(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment string) {
	d.handler.GetDeployment(w, r, project, deployment)
}

func (d *APIGenDispatcher) CancelDeployment(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment string, _ deploymentgen.GenCancelDeploymentHeaders) {
	d.handler.CancelDeployment(w, r, project, deployment)
}

func (d *APIGenDispatcher) ListDeploymentEvents(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment string, params deploymentgen.GenListDeploymentEventsParams, _ deploymentgen.GenListDeploymentEventsHeaders) {
	d.handler.ListDeploymentEvents(w, r, project, deployment, params.Limit, params.PageToken)
}

func (d *APIGenDispatcher) RollbackDeployment(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment string, headers deploymentgen.GenRollbackDeploymentHeaders) {
	d.handler.RollbackDeployment(w, r, project, deployment, headers.IdempotencyKey)
}

type APIGenTransportErrorResponder struct{ Logger *slog.Logger }

func (responder APIGenTransportErrorResponder) RespondTransportError(ctx context.Context, w stdhttp.ResponseWriter, r *stdhttp.Request, failure deploymentgen.GenTransportError) {
	apitransport.WriteAPIGenFailure(ctx, w, r, responder.Logger, apitransport.APIGenFailure{
		OperationID: failure.OperationID, Kind: failure.Kind, StatusCode: failure.StatusCode,
		Code: failure.Code, PublicDetail: failure.PublicDetail, Cause: failure.Cause,
	})
}

func DispatchAPIGenOperation(operationID string, handler APIGenHandler, logger *slog.Logger, w stdhttp.ResponseWriter, r *stdhttp.Request) bool {
	return deploymentgen.DispatchAPIGenOperation(
		operationID, NewAPIGenDispatcher(handler), APIGenTransportErrorResponder{Logger: logger}, w, r,
	)
}
