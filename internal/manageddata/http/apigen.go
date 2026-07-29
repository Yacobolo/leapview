package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"

	manageddataapi "github.com/flidai/leapview/internal/manageddata/api"
	manageddatagen "github.com/flidai/leapview/internal/manageddata/api/gen"
	apitransport "github.com/flidai/leapview/internal/platform/http/transport"
)

type APIGenHandler interface {
	ListManagedConnections(stdhttp.ResponseWriter, *stdhttp.Request, string, manageddataapi.PageParams)
	GetManagedConnection(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	GetActiveManagedDataRevision(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	ListManagedDataRevisions(stdhttp.ResponseWriter, *stdhttp.Request, string, string, manageddataapi.PageParams)
	GetManagedDataRevision(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
	ListManagedDataUploadSessions(stdhttp.ResponseWriter, *stdhttp.Request, string, string, manageddataapi.PageParams)
	CreateManagedDataUploadSession(stdhttp.ResponseWriter, *stdhttp.Request, string, string, manageddataapi.IdempotencyHeaders)
	GetManagedDataUploadSession(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
	CancelManagedDataUploadSession(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, manageddataapi.IdempotencyHeaders)
	ListManagedDataUploadSessionEvents(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, manageddataapi.PageParams, manageddataapi.GenListManagedDataUploadSessionEventsHeaders)
	FinalizeManagedDataUploadSession(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, manageddataapi.IdempotencyHeaders)
	CreateManagedDataS3MultipartUpload(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, manageddataapi.IdempotencyHeaders)
	AbortManagedDataS3MultipartUpload(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, string, manageddataapi.IdempotencyHeaders)
	CompleteManagedDataS3MultipartUpload(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, string, manageddataapi.IdempotencyHeaders)
	SignManagedDataS3MultipartPart(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, string, int32)
}

type APIGenDispatcher struct{ handler APIGenHandler }

func NewAPIGenDispatcher(handler APIGenHandler) *APIGenDispatcher {
	return &APIGenDispatcher{handler: handler}
}

func (d *APIGenDispatcher) ListManagedConnections(w stdhttp.ResponseWriter, r *stdhttp.Request, project string, params manageddatagen.GenListManagedConnectionsParams) {
	d.handler.ListManagedConnections(w, r, project, manageddataapi.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}

func (d *APIGenDispatcher) GetManagedConnection(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection string) {
	d.handler.GetManagedConnection(w, r, project, connection)
}

func (d *APIGenDispatcher) GetActiveManagedDataRevision(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection string) {
	d.handler.GetActiveManagedDataRevision(w, r, project, connection)
}

func (d *APIGenDispatcher) ListManagedDataRevisions(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection string, params manageddatagen.GenListManagedDataRevisionsParams) {
	d.handler.ListManagedDataRevisions(w, r, project, connection, manageddataapi.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}

func (d *APIGenDispatcher) GetManagedDataRevision(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection, revision string) {
	d.handler.GetManagedDataRevision(w, r, project, connection, revision)
}

func (d *APIGenDispatcher) ListManagedDataUploadSessions(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection string, params manageddatagen.GenListManagedDataUploadSessionsParams) {
	d.handler.ListManagedDataUploadSessions(w, r, project, connection, manageddataapi.PageParams{Limit: params.Limit, PageToken: params.PageToken})
}

func (d *APIGenDispatcher) CreateManagedDataUploadSession(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection string, headers manageddatagen.GenCreateManagedDataUploadSessionHeaders) {
	d.handler.CreateManagedDataUploadSession(w, r, project, connection, manageddataapi.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (d *APIGenDispatcher) GetManagedDataUploadSession(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection, uploadSession string) {
	d.handler.GetManagedDataUploadSession(w, r, project, connection, uploadSession)
}

func (d *APIGenDispatcher) CancelManagedDataUploadSession(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection, uploadSession string, headers manageddatagen.GenCancelManagedDataUploadSessionHeaders) {
	d.handler.CancelManagedDataUploadSession(w, r, project, connection, uploadSession, manageddataapi.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (d *APIGenDispatcher) ListManagedDataUploadSessionEvents(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection, uploadSession string, params manageddatagen.GenListManagedDataUploadSessionEventsParams, headers manageddatagen.GenListManagedDataUploadSessionEventsHeaders) {
	d.handler.ListManagedDataUploadSessionEvents(
		w, r, project, connection, uploadSession,
		manageddataapi.PageParams{Limit: params.Limit, PageToken: params.PageToken},
		manageddataapi.GenListManagedDataUploadSessionEventsHeaders{Accept: headers.Accept, LastEventID: headers.LastEventID},
	)
}

func (d *APIGenDispatcher) FinalizeManagedDataUploadSession(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection, uploadSession string, headers manageddatagen.GenFinalizeManagedDataUploadSessionHeaders) {
	d.handler.FinalizeManagedDataUploadSession(w, r, project, connection, uploadSession, manageddataapi.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (d *APIGenDispatcher) CreateManagedDataS3MultipartUpload(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection, uploadSession string, headers manageddatagen.GenCreateManagedDataS3MultipartUploadHeaders) {
	d.handler.CreateManagedDataS3MultipartUpload(w, r, project, connection, uploadSession, manageddataapi.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (d *APIGenDispatcher) AbortManagedDataS3MultipartUpload(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection, uploadSession, multipartUpload string, headers manageddatagen.GenAbortManagedDataS3MultipartUploadHeaders) {
	d.handler.AbortManagedDataS3MultipartUpload(w, r, project, connection, uploadSession, multipartUpload, manageddataapi.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (d *APIGenDispatcher) CompleteManagedDataS3MultipartUpload(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection, uploadSession, multipartUpload string, headers manageddatagen.GenCompleteManagedDataS3MultipartUploadHeaders) {
	d.handler.CompleteManagedDataS3MultipartUpload(w, r, project, connection, uploadSession, multipartUpload, manageddataapi.IdempotencyHeaders{IdempotencyKey: headers.IdempotencyKey})
}

func (d *APIGenDispatcher) SignManagedDataS3MultipartPart(w stdhttp.ResponseWriter, r *stdhttp.Request, project, connection, uploadSession, multipartUpload string, partNumber int32, _ manageddatagen.GenSignManagedDataS3MultipartPartHeaders) {
	d.handler.SignManagedDataS3MultipartPart(w, r, project, connection, uploadSession, multipartUpload, partNumber)
}

type APIGenTransportErrorResponder struct{ Logger *slog.Logger }

func (responder APIGenTransportErrorResponder) RespondTransportError(ctx context.Context, w stdhttp.ResponseWriter, r *stdhttp.Request, failure manageddatagen.GenTransportError) {
	apitransport.WriteAPIGenFailure(ctx, w, r, responder.Logger, apitransport.APIGenFailure{
		OperationID: failure.OperationID, Kind: failure.Kind, StatusCode: failure.StatusCode,
		Code: failure.Code, PublicDetail: failure.PublicDetail, Cause: failure.Cause,
	})
}

func DispatchAPIGenOperation(operationID string, handler APIGenHandler, logger *slog.Logger, w stdhttp.ResponseWriter, r *stdhttp.Request) bool {
	return manageddatagen.DispatchAPIGenOperation(
		operationID, NewAPIGenDispatcher(handler), APIGenTransportErrorResponder{Logger: logger}, w, r,
	)
}
