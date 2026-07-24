package module

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	apitransport "github.com/Yacobolo/leapview/internal/api/transport"
	"github.com/Yacobolo/leapview/internal/manageddata/control"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
	jobhttp "github.com/Yacobolo/leapview/internal/platform/jobs/http"
)

func (m *Module) beginFinalize(ctx context.Context, request control.UploadRequest) (control.UploadResult, error) {
	payload, err := json.Marshal(FinalizeUploadJob{Project: request.Project, Connection: request.Connection, UploadSession: request.UploadID})
	if err != nil {
		return control.UploadResult{}, err
	}
	event, err := json.Marshal(map[string]any{"uploadSessionId": request.UploadID, "status": "finalizing"})
	if err != nil {
		return control.UploadResult{}, err
	}
	request.Workflow = jobs.WorkflowIntent{
		Event: jobs.EventInput{
			Key: "upload_session.finalizing", ResourceKind: "upload", ResourceID: request.UploadID,
			EventType: "upload_session.finalizing", Data: event,
		},
		Job: jobs.EnqueueInput{
			ID: "upload:" + request.UploadID + ":finalize", Kind: FinalizeUploadJobKind,
			WorkloadClass: "control", WorkspaceID: "_node",
			ResourceKind: "upload", ResourceID: request.UploadID, Payload: payload,
		},
	}
	if m == nil || m.uploads == nil {
		return control.UploadResult{}, errors.New("managed-data finalization is unavailable")
	}
	return m.uploads.BeginFinalizeUpload(ctx, request)
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
