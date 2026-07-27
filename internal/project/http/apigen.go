package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"

	apitransport "github.com/Yacobolo/leapview/internal/platform/http/transport"
	projectgen "github.com/Yacobolo/leapview/internal/project/api/gen"
)

// Handler is the runtime port consumed by Project's generated HTTP adapter.
// A provider may implement it without turning the compile-time Project
// capability into a synthetic runtime module.
type Handler interface {
	ListProjects(stdhttp.ResponseWriter, *stdhttp.Request, *int32, *string)
	GetProject(stdhttp.ResponseWriter, *stdhttp.Request, string)
	ListProjectWorkspaces(stdhttp.ResponseWriter, *stdhttp.Request, string, *int32, *string)
}

type APIGenDispatcher struct {
	handler Handler
}

func NewAPIGenDispatcher(handler Handler) *APIGenDispatcher {
	return &APIGenDispatcher{handler: handler}
}

func (d *APIGenDispatcher) ListProjects(w stdhttp.ResponseWriter, r *stdhttp.Request, params projectgen.GenListProjectsParams) {
	d.handler.ListProjects(w, r, params.Limit, params.PageToken)
}

func (d *APIGenDispatcher) GetProject(w stdhttp.ResponseWriter, r *stdhttp.Request, project string) {
	d.handler.GetProject(w, r, project)
}

func (d *APIGenDispatcher) ListProjectWorkspaces(w stdhttp.ResponseWriter, r *stdhttp.Request, project string, params projectgen.GenListProjectWorkspacesParams) {
	d.handler.ListProjectWorkspaces(w, r, project, params.Limit, params.PageToken)
}

type APIGenTransportErrorResponder struct {
	Logger *slog.Logger
}

func (responder APIGenTransportErrorResponder) RespondTransportError(ctx context.Context, w stdhttp.ResponseWriter, r *stdhttp.Request, failure projectgen.GenTransportError) {
	apitransport.WriteAPIGenFailure(ctx, w, r, responder.Logger, apitransport.APIGenFailure{
		OperationID: failure.OperationID, Kind: failure.Kind, StatusCode: failure.StatusCode,
		Code: failure.Code, PublicDetail: failure.PublicDetail, Cause: failure.Cause,
	})
}

func DispatchAPIGenOperation(operationID string, handler Handler, logger *slog.Logger, w stdhttp.ResponseWriter, r *stdhttp.Request) bool {
	return projectgen.DispatchAPIGenOperation(
		operationID,
		NewAPIGenDispatcher(handler),
		APIGenTransportErrorResponder{Logger: logger},
		w,
		r,
	)
}
