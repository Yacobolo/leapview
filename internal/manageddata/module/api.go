package module

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
	"github.com/Yacobolo/leapview/internal/manageddata/control"
	apitransport "github.com/Yacobolo/leapview/internal/platform/http/transport"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
	jobhttp "github.com/Yacobolo/leapview/internal/platform/jobs/http"
)

func (m *Module) enqueueFinalize(ctx context.Context, request control.UploadRequest) error {
	if err := m.appendEvent(ctx, request.UploadID, "upload_session.finalizing", map[string]any{"uploadSessionId": request.UploadID, "status": "finalizing"}); err != nil {
		return err
	}
	payload, err := json.Marshal(FinalizeUploadJob{Project: request.Project, Connection: request.Connection, UploadSession: request.UploadID})
	if err != nil {
		return err
	}
	if m == nil || m.jobs == nil {
		return errors.New("managed-data job queue is unavailable")
	}
	_, err = m.jobs.Enqueue(ctx, jobs.EnqueueInput{
		ID: "upload:" + request.UploadID + ":finalize", Kind: FinalizeUploadJobKind,
		WorkloadClass: "control", WorkspaceID: "_node",
		ResourceKind: "upload", ResourceID: request.UploadID, Payload: payload,
	})
	return err
}

func (m *Module) recordUploadCreated(ctx context.Context, result control.UploadResult) error {
	return m.appendEvent(ctx, result.ID, "upload_session.created", map[string]any{
		"uploadSessionId": result.ID, "projectId": result.Collection.Project,
		"connectionId": result.Collection.Connection, "status": result.Status,
	})
}

func (m *Module) appendEvent(ctx context.Context, uploadID, eventType string, data any) error {
	if m == nil || m.jobs == nil {
		return errors.New("managed-data event store is unavailable")
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = m.jobs.AppendEvent(ctx, "upload", uploadID, eventType, encoded)
	return err
}

func (m *Module) ListUploadSessionEvents(w http.ResponseWriter, r *http.Request, projectID, connectionID, sessionID string, params apigenapi.GenListManagedDataUploadSessionEventsParams, _ apigenapi.GenListManagedDataUploadSessionEventsHeaders) {
	if m == nil || m.uploads == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "UPLOAD_SERVICE_UNAVAILABLE", "Managed-data uploads are unavailable", nil)
		return
	}
	if _, err := m.uploads.RecoverUpload(r.Context(), control.UploadRequest{Project: projectID, Connection: connectionID, UploadID: sessionID}); err != nil {
		apitransport.WriteProblem(w, r, http.StatusNotFound, "UPLOAD_SESSION_NOT_FOUND", "Upload session not found", nil)
		return
	}
	if m.jobs == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_EVENT_STORE_UNAVAILABLE", "Upload events are unavailable", nil)
		return
	}
	jobhttp.WriteEventPage(w, r, m.jobs, "upload", sessionID, params.Limit, params.PageToken, "upload:"+projectID+":"+connectionID+":"+sessionID)
}
