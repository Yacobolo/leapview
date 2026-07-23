package module

import (
	"context"
	"encoding/json"

	"github.com/Yacobolo/leapview/internal/manageddata/control"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
)

const FinalizeUploadJobKind = "upload.finalize"

type FinalizeUploadJob struct {
	Project       string
	Connection    string
	UploadSession string
}

func (m *Module) JobHandlers(events jobs.EventAppender) []jobs.Handler {
	return []jobs.Handler{jobs.HandlerFunc{JobKind: FinalizeUploadJobKind, Run: func(ctx context.Context, job jobs.Job) error {
		var payload FinalizeUploadJob
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return err
		}
		result, err := m.finalizer.CompleteFinalizeUpload(ctx, control.UploadRequest{Project: payload.Project, Connection: payload.Connection, UploadID: payload.UploadSession})
		event := "upload_session.completed"
		if err != nil {
			event = "upload_session.failed"
		}
		data, _ := json.Marshal(map[string]any{"uploadSessionId": payload.UploadSession, "status": result.Upload.Status})
		_, _ = events.AppendEvent(context.WithoutCancel(ctx), "upload", payload.UploadSession, event, data)
		return err
	}}}
}
