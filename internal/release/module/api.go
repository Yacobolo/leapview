package module

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	apitransport "github.com/Yacobolo/leapview/internal/platform/http/transport"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
	jobhttp "github.com/Yacobolo/leapview/internal/platform/jobs/http"
	"github.com/Yacobolo/leapview/internal/release"
	releaseapi "github.com/Yacobolo/leapview/internal/release/api"
	releasefilesystem "github.com/Yacobolo/leapview/internal/release/filesystem"
)

type Principal struct {
	ID string
}

type PageParams = releaseapi.PageParams

type JobStore interface {
	Enqueue(context.Context, jobs.EnqueueInput) (jobs.Job, error)
	AppendEvent(context.Context, string, string, string, []byte) (jobs.Event, error)
	ListEvents(context.Context, string, string, int64, int) ([]jobs.Event, error)
}

type APIConfig struct {
	CurrentPrincipal func(*http.Request) (Principal, bool)
	Jobs             JobStore
}

func (m *Module) CreateRelease(w http.ResponseWriter, r *http.Request, project, idempotencyKey string) {
	principal, ok := m.currentPrincipal(r)
	if !ok {
		apitransport.WriteProblem(w, r, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Bearer authentication is required", nil)
		return
	}
	var body releaseapi.CreateRequest
	if err := apitransport.DecodeBody(w, r, &body); err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_JSON", err.Error(), nil)
		return
	}
	input := release.CreateInput{ProjectID: project, ProjectDigest: body.ProjectDigest, IdempotencyKey: idempotencyKey, CreatedBy: principal.ID}
	for _, item := range body.Workspaces {
		input.Workspaces = append(input.Workspaces, release.WorkspaceManifest{WorkspaceID: item.Workspace, ArtifactDigest: item.ArtifactDigest})
	}
	for _, item := range body.Connections {
		input.Connections = append(input.Connections, release.ConnectionPin{ConnectionID: item.Connection, RevisionID: item.RevisionID})
	}
	created, err := m.service.Create(r.Context(), input)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := m.appendEvent(r.Context(), created.ID, "release.created", response(created)); err != nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_EVENT_STORE_UNAVAILABLE", "Release event history could not be persisted", nil)
		return
	}
	w.Header().Set("Location", location(project, created.ID))
	apitransport.WriteJSON(w, http.StatusCreated, response(created))
}

func (m *Module) ListReleases(w http.ResponseWriter, r *http.Request, project string, params releaseapi.PageParams) {
	rows, err := m.service.List(r.Context(), project)
	if err != nil {
		writeError(w, r, err)
		return
	}
	items := make([]releaseapi.Response, 0, len(rows))
	for _, row := range rows {
		items = append(items, response(row))
	}
	page, next, err := apitransport.KeysetPage(items, params.Limit, params.PageToken, func(item releaseapi.Response) string { return item.CreatedAt + "\x00" + item.ID })
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_CURSOR", err.Error(), nil)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, releaseapi.ListResponse{Items: page, Page: releaseapi.PageInfo{NextCursor: next}})
}

func (m *Module) GetRelease(w http.ResponseWriter, r *http.Request, project, releaseID string) {
	row, err := m.service.Get(r.Context(), project, releaseID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("ETag", apitransport.StrongETag(row.RequestDigest+":"+string(row.Status)))
	apitransport.WriteJSON(w, http.StatusOK, response(row))
}

func (m *Module) UploadReleaseArtifact(w http.ResponseWriter, r *http.Request, project, releaseID, workspaceID, contentType, contentDigest string) {
	if contentType != "application/octet-stream" {
		apitransport.WriteProblem(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Release artifacts require application/octet-stream", nil)
		return
	}
	artifact, err := m.service.UploadArtifact(r.Context(), project, releaseID, workspaceID, contentDigest, http.MaxBytesReader(w, r.Body, releasefilesystem.MaxUploadBytes))
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Location", location(project, releaseID)+"/workspaces/"+workspaceID+"/artifact")
	apitransport.WriteJSON(w, http.StatusCreated, releaseapi.ArtifactResponse{ReleaseID: releaseID, WorkspaceID: workspaceID, Digest: artifact.ExpectedDigest, SizeBytes: artifact.SizeBytes})
}

func (m *Module) FinalizeRelease(w http.ResponseWriter, r *http.Request, project, releaseID string) {
	row, err := m.service.BeginFinalization(r.Context(), project, releaseID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := m.appendEvent(r.Context(), releaseID, "release.validating", response(row)); err != nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_EVENT_STORE_UNAVAILABLE", "Release finalization could not be queued", nil)
		return
	}
	payload, err := json.Marshal(FinalizeJob{Project: project, Release: releaseID})
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Release finalization could not be queued", nil)
		return
	}
	if m.api.Jobs == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_QUEUE_UNAVAILABLE", "Release finalization could not be queued", nil)
		return
	}
	if _, err := m.api.Jobs.Enqueue(r.Context(), jobs.EnqueueInput{
		ID: "release:" + releaseID + ":finalize", Kind: FinalizeJobKind,
		WorkloadClass: "control", WorkspaceID: "_node",
		ResourceKind: "release", ResourceID: releaseID, Payload: payload,
	}); err != nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_QUEUE_UNAVAILABLE", "Release finalization could not be queued", nil)
		return
	}
	w.Header().Set("Location", location(project, releaseID))
	apitransport.WriteJSON(w, http.StatusAccepted, response(row))
}

func (m *Module) ListReleaseEvents(w http.ResponseWriter, r *http.Request, project, releaseID string, params releaseapi.PageParams) {
	if _, err := m.service.Get(r.Context(), project, releaseID); err != nil {
		writeError(w, r, err)
		return
	}
	if m.api.Jobs == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_EVENT_STORE_UNAVAILABLE", "Release events are unavailable", nil)
		return
	}
	jobhttp.WriteEventPage(w, r, m.api.Jobs, "release", releaseID, params.Limit, params.PageToken, "release:"+project+":"+releaseID)
}

func (m *Module) currentPrincipal(r *http.Request) (Principal, bool) {
	if m == nil || m.api.CurrentPrincipal == nil {
		return Principal{}, false
	}
	return m.api.CurrentPrincipal(r)
}

func (m *Module) appendEvent(ctx context.Context, releaseID, eventType string, data any) error {
	if m == nil || m.api.Jobs == nil {
		return errors.New("release event store is unavailable")
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = m.api.Jobs.AppendEvent(ctx, "release", releaseID, eventType, encoded)
	return err
}

func response(row release.Release) releaseapi.Response {
	result := releaseapi.Response{
		ID: row.ID, ProjectID: row.ProjectID, ProjectDigest: row.ProjectDigest, Status: releaseapi.Status(row.Status),
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, Workspaces: make([]releaseapi.WorkspaceManifest, 0, len(row.Manifest.Workspaces)),
		Connections: make([]releaseapi.ConnectionPin, 0, len(row.Manifest.Connections)),
	}
	for _, item := range row.Manifest.Workspaces {
		mapped := releaseapi.WorkspaceManifest{Workspace: item.WorkspaceID, ArtifactDigest: item.ArtifactDigest}
		if item.ServingStateID != "" {
			mapped.ServingStateID = &item.ServingStateID
		}
		result.Workspaces = append(result.Workspaces, mapped)
	}
	for _, item := range row.Manifest.Connections {
		result.Connections = append(result.Connections, releaseapi.ConnectionPin{Connection: item.ConnectionID, RevisionID: item.RevisionID})
	}
	if row.FinalizedAt != "" {
		result.FinalizedAt = &row.FinalizedAt
	}
	if row.Error != "" {
		result.Error = &row.Error
	}
	return result
}

func location(project, releaseID string) string {
	return "/api/v1/projects/" + project + "/releases/" + releaseID
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := http.StatusInternalServerError, "INTERNAL_ERROR"
	switch {
	case errors.Is(err, release.ErrInvalid):
		status, code = http.StatusUnprocessableEntity, "INVALID_RELEASE"
	case errors.Is(err, release.ErrNotFound):
		status, code = http.StatusNotFound, "RELEASE_NOT_FOUND"
	case errors.Is(err, release.ErrIncomplete), errors.Is(err, release.ErrConflict), errors.Is(err, release.ErrImmutable):
		status, code = http.StatusConflict, "RELEASE_CONFLICT"
	case errors.Is(err, release.ErrDigest):
		status, code = http.StatusUnprocessableEntity, "CONTENT_DIGEST_MISMATCH"
	}
	detail := err.Error()
	if status == http.StatusInternalServerError {
		detail = "The release request could not be completed"
	}
	apitransport.WriteProblem(w, r, status, code, detail, nil)
}
