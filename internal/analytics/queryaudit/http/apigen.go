package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"

	analyticsgen "github.com/flidai/leapview/internal/analytics/api/gen"
	apitransport "github.com/flidai/leapview/internal/platform/http/transport"
)

type QueryEventsHandler interface {
	ListQueryEvents(stdhttp.ResponseWriter, *stdhttp.Request)
}

// APIGenDispatcher adapts the Analytics query-audit HTTP handler to its
// generated transport contract.
type APIGenDispatcher struct {
	handler QueryEventsHandler
}

func NewAPIGenDispatcher(handler QueryEventsHandler) *APIGenDispatcher {
	return &APIGenDispatcher{handler: handler}
}

func (d *APIGenDispatcher) ListQueryEvents(w stdhttp.ResponseWriter, r *stdhttp.Request, _ string, _ analyticsgen.GenListQueryEventsParams) {
	d.handler.ListQueryEvents(w, r)
}

type APIGenTransportErrorResponder struct {
	Logger *slog.Logger
}

func (responder APIGenTransportErrorResponder) RespondTransportError(ctx context.Context, w stdhttp.ResponseWriter, r *stdhttp.Request, failure analyticsgen.GenTransportError) {
	apitransport.WriteAPIGenFailure(ctx, w, r, responder.Logger, apitransport.APIGenFailure{
		OperationID: failure.OperationID, Kind: failure.Kind, StatusCode: failure.StatusCode,
		Code: failure.Code, PublicDetail: failure.PublicDetail, Cause: failure.Cause,
	})
}

func DispatchAPIGenOperation(operationID string, handler QueryEventsHandler, logger *slog.Logger, w stdhttp.ResponseWriter, r *stdhttp.Request) bool {
	return analyticsgen.DispatchAPIGenOperation(
		operationID,
		NewAPIGenDispatcher(handler),
		APIGenTransportErrorResponder{Logger: logger},
		w,
		r,
	)
}
