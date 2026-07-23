package module

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	apitransport "github.com/Yacobolo/leapview/internal/api/transport"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
	jobhttp "github.com/Yacobolo/leapview/internal/platform/jobs/http"
	"github.com/Yacobolo/leapview/internal/release"
	releasefilesystem "github.com/Yacobolo/leapview/internal/release/filesystem"
)

type Principal struct {
	ID string
}

type JobStore interface {
	Enqueue(context.Context, jobs.EnqueueInput) (jobs.Job, error)
	AppendEvent(context.Context, string, string, string, []byte) (jobs.Event, error)
	ListEvents(context.Context, string, string, int64, int) ([]jobs.Event, error)
}

type APIConfig struct {
	CurrentPrincipal func(*http.Request) (Principal, bool)
	Jobs             JobStore
}

func (m *Module) CreateRelease(w http.ResponseWriter, r *http.Request, project string, headers apigenapi.GenCreateReleaseHeaders) {
	principal, ok := m.currentPrincipal(r)
	if !ok {
		apitransport.WriteProblem(w, r, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Bearer authentication is required", nil)
		return
	}
	var body apigenapi.ReleaseCreateRequest
	if err := apitransport.DecodeBody(w, r, &body); err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_JSON", err.Error(), nil)
		return
	}
	input := release.CreateInput{ProjectID: project, ProjectDigest: body.ProjectDigest, IdempotencyKey: headers.IdempotencyKey, CreatedBy: principal.ID}
	for _, item := range body.Workspaces {
		input.Workspaces = append(input.Workspaces, release.WorkspaceManifest{WorkspaceID: item.Workspace, ArtifactDigest: item.ArtifactDigest})
	}
	for _, item := range body.Connections {
		input.Connections = append(input.Connections, release.ConnectionPin{ConnectionID: item.Connection, RevisionID: item.RevisionId})
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

func (m *Module) ListReleases(w http.ResponseWriter, r *http.Request, project string, params apigenapi.GenListReleasesParams) {
	rows, err := m.service.List(r.Context(), project)
	if err != nil {
		writeError(w, r, err)
		return
	}
	items := make([]apigenapi.ReleaseResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, response(row))
	}
	page, next, err := apitransport.KeysetPage(items, params.Limit, params.PageToken, func(item apigenapi.ReleaseResponse) string { return item.CreatedAt + "\x00" + item.Id })
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_CURSOR", err.Error(), nil)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, apigenapi.ReleaseListResponse{Items: page, Page: apigenapi.PageInfo{NextCursor: next}})
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

func (m *Module) UploadReleaseArtifact(w http.ResponseWriter, r *http.Request, project, releaseID, workspaceID string, headers apigenapi.GenUploadReleaseArtifactHeaders) {
	if headers.ContentType != "application/octet-stream" {
		apitransport.WriteProblem(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Release artifacts require application/octet-stream", nil)
		return
	}
	artifact, err := m.service.UploadArtifact(r.Context(), project, releaseID, workspaceID, headers.ContentDigest, http.MaxBytesReader(w, r.Body, releasefilesystem.MaxUploadBytes))
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Location", location(project, releaseID)+"/workspaces/"+workspaceID+"/artifact")
	apitransport.WriteJSON(w, http.StatusCreated, apigenapi.ReleaseArtifactResponse{ReleaseId: releaseID, WorkspaceId: workspaceID, Digest: artifact.ExpectedDigest, SizeBytes: artifact.SizeBytes})
}

func (m *Module) FinalizeRelease(w http.ResponseWriter, r *http.Request, project, releaseID string, _ apigenapi.GenFinalizeReleaseHeaders) {
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

func (m *Module) ListReleaseEvents(w http.ResponseWriter, r *http.Request, project, releaseID string, params apigenapi.GenListReleaseEventsParams, _ apigenapi.GenListReleaseEventsHeaders) {
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

func response(row release.Release) apigenapi.ReleaseResponse {
	result := apigenapi.ReleaseResponse{
		Id: row.ID, ProjectId: row.ProjectID, ProjectDigest: row.ProjectDigest, Status: apigenapi.ReleaseStatus(row.Status),
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, Workspaces: make([]apigenapi.ReleaseWorkspaceManifest, 0, len(row.Manifest.Workspaces)),
		Connections: make([]apigenapi.ReleaseConnectionPin, 0, len(row.Manifest.Connections)),
	}
	for _, item := range row.Manifest.Workspaces {
		mapped := apigenapi.ReleaseWorkspaceManifest{Workspace: item.WorkspaceID, ArtifactDigest: item.ArtifactDigest}
		if item.ServingStateID != "" {
			mapped.ServingStateId = &item.ServingStateID
		}
		result.Workspaces = append(result.Workspaces, mapped)
	}
	for _, item := range row.Manifest.Connections {
		result.Connections = append(result.Connections, apigenapi.ReleaseConnectionPin{Connection: item.ConnectionID, RevisionId: item.RevisionID})
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
