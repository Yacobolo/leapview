package module

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Yacobolo/leapview/internal/deployment"
	deploymentapi "github.com/Yacobolo/leapview/internal/deployment/api"
	"github.com/Yacobolo/leapview/internal/deployment/apiadapter"
	deploymenthttp "github.com/Yacobolo/leapview/internal/deployment/http"
	apitransport "github.com/Yacobolo/leapview/internal/platform/http/transport"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
	jobhttp "github.com/Yacobolo/leapview/internal/platform/jobs/http"
	"github.com/Yacobolo/leapview/internal/release"
)

var ErrPublicationForbidden = errors.New("publication deployment forbidden")

type PageParams = deploymentapi.PageParams

type ReleasePort interface {
	Get(context.Context, string, string) (release.Release, error)
	LinkDeployment(context.Context, string, string, string, string) error
	DeploymentRelease(context.Context, string, string) (string, string, error)
	ListDeploymentIDs(context.Context, string) ([]string, error)
	PriorDeploymentRelease(context.Context, string, string) (string, error)
}

type JobStore interface {
	Enqueue(context.Context, jobs.EnqueueInput) (jobs.Job, error)
	Cancel(context.Context, string) error
	AppendEvent(context.Context, string, string, string, []byte) (jobs.Event, error)
	ListEvents(context.Context, string, string, int64, int) ([]jobs.Event, error)
}

type APIConfig struct {
	Releases ReleasePort
	Jobs     JobStore
}

func (m *Module) CreateDeployment(w http.ResponseWriter, r *http.Request, project, idempotencyKey string) {
	var body deploymentapi.CreateRequest
	if err := apitransport.DecodeBody(w, r, &body); err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_JSON", err.Error(), nil)
		return
	}
	m.createDeployment(w, r, project, body.ReleaseID, idempotencyKey, "")
}

func (m *Module) createDeployment(w http.ResponseWriter, r *http.Request, project, releaseID, idempotencyKey, rollbackOf string) {
	principal, ok := m.principal(r)
	if !ok {
		apitransport.WriteProblem(w, r, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Bearer authentication is required", nil)
		return
	}
	if m.jobs.Coordinator == nil || m.api.Releases == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "DEPLOYMENT_SERVICE_UNAVAILABLE", "Deployment service is unavailable", nil)
		return
	}
	targetRelease, err := m.api.Releases.Get(r.Context(), project, releaseID)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	if targetRelease.Status != release.StatusReady {
		apitransport.WriteProblem(w, r, http.StatusConflict, "RELEASE_NOT_READY", "Only ready releases can be deployed", nil)
		return
	}
	targets := make([]apiadapter.TargetRequest, 0, len(targetRelease.Artifacts))
	for _, artifact := range targetRelease.Artifacts {
		if artifact.ServingStateID == "" {
			apitransport.WriteProblem(w, r, http.StatusConflict, "RELEASE_INCOMPLETE", "Release is missing a workspace artifact", nil)
			return
		}
		targets = append(targets, apiadapter.TargetRequest{Workspace: artifact.WorkspaceID, CandidateID: artifact.ServingStateID})
	}
	if m.jobs.Authorize != nil {
		if err := m.jobs.Authorize(r.Context(), principal.ID, m.handlerEnvironment(), targets); err != nil {
			if errors.Is(err, ErrPublicationForbidden) {
				apitransport.WriteProblem(w, r, http.StatusForbidden, "PUBLICATION_MANAGEMENT_REQUIRED", "MANAGE_PUBLICATIONS is required to activate a workspace containing a public dashboard publication", nil)
				return
			}
			apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "AUTHORIZATION_UNAVAILABLE", "Publication activation authorization could not be evaluated", nil)
			return
		}
	}
	created, err := m.jobs.Coordinator.Create(r.Context(), apiadapter.CreateRequest{
		Project: project, Environment: m.handlerEnvironment(), Targets: targets, Actor: principal.ID, IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	if err := m.api.Releases.LinkDeployment(r.Context(), project, created.ID, releaseID, rollbackOf); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if err := m.appendAPIEvent(r.Context(), created.ID, "deployment.queued", map[string]any{"deploymentId": created.ID, "projectId": project, "releaseId": releaseID, "status": "queued"}); err != nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_EVENT_STORE_UNAVAILABLE", "Deployment event history could not be persisted", nil)
		return
	}
	payload, _ := json.Marshal(ActivateJob{Project: project, Deployment: created.ID, Actor: principal.ID, IdempotencyKey: idempotencyKey + ":cutover"})
	if m.api.Jobs == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_QUEUE_UNAVAILABLE", "Deployment could not be queued", nil)
		return
	}
	if _, err := m.api.Jobs.Enqueue(r.Context(), jobs.EnqueueInput{
		ID: "deployment:" + created.ID + ":activate", Kind: ActivateJobKind,
		WorkloadClass: "control", WorkspaceID: "_node",
		ResourceKind: "deployment", ResourceID: created.ID, Payload: payload,
	}); err != nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_QUEUE_UNAVAILABLE", "Deployment could not be queued", nil)
		return
	}
	w.Header().Set("Location", deploymentLocation(project, created.ID))
	apitransport.WriteJSON(w, http.StatusAccepted, deploymentResponse(created, releaseID, principal.ID))
}

func (m *Module) GetDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string) {
	if m.jobs.Coordinator == nil || m.api.Releases == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "DEPLOYMENT_SERVICE_UNAVAILABLE", "Deployment service is unavailable", nil)
		return
	}
	releaseID, _, err := m.api.Releases.DeploymentRelease(r.Context(), project, deploymentID)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	row, err := m.jobs.Coordinator.Get(r.Context(), apiadapter.Scope{Project: project, DeploymentID: deploymentID})
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, deploymentResponse(row, releaseID, ""))
}

func (m *Module) ListDeployments(w http.ResponseWriter, r *http.Request, project string, params deploymentapi.PageParams) {
	if m.jobs.Coordinator == nil || m.api.Releases == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "DEPLOYMENT_SERVICE_UNAVAILABLE", "Deployment service is unavailable", nil)
		return
	}
	ids, err := m.api.Releases.ListDeploymentIDs(r.Context(), project)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	items := make([]deploymentapi.Response, 0, len(ids))
	for _, id := range ids {
		releaseID, _, err := m.api.Releases.DeploymentRelease(r.Context(), project, id)
		if err != nil {
			continue
		}
		row, err := m.jobs.Coordinator.Get(r.Context(), apiadapter.Scope{Project: project, DeploymentID: id})
		if err != nil {
			continue
		}
		items = append(items, deploymentResponse(row, releaseID, ""))
	}
	page, next, err := apitransport.KeysetPage(items, params.Limit, params.PageToken, func(item deploymentapi.Response) string { return item.CreatedAt + "\x00" + item.ID })
	if err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_CURSOR", err.Error(), nil)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, deploymentapi.ListResponse{Items: page, Page: deploymentapi.PageInfo{NextCursor: next}})
}

func (m *Module) CancelDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID string) {
	if m.jobs.Coordinator == nil || m.api.Releases == nil || m.api.Jobs == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "DEPLOYMENT_SERVICE_UNAVAILABLE", "Deployment service is unavailable", nil)
		return
	}
	releaseID, _, err := m.api.Releases.DeploymentRelease(r.Context(), project, deploymentID)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	if err := m.api.Jobs.Cancel(r.Context(), "deployment:"+deploymentID+":activate"); err != nil && !errors.Is(err, jobs.ErrConflict) {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_QUEUE_UNAVAILABLE", "Deployment cancellation could not be persisted", nil)
		return
	}
	row, err := m.jobs.Coordinator.Cancel(r.Context(), apiadapter.Scope{Project: project, DeploymentID: deploymentID})
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	_ = m.appendAPIEvent(r.Context(), deploymentID, "deployment.cancelled", map[string]any{"deploymentId": deploymentID, "status": "cancelled"})
	w.Header().Set("Location", deploymentLocation(project, deploymentID))
	apitransport.WriteJSON(w, http.StatusAccepted, deploymentResponse(row, releaseID, ""))
}

func (m *Module) RollbackDeployment(w http.ResponseWriter, r *http.Request, project, deploymentID, idempotencyKey string) {
	if m.api.Releases == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "DEPLOYMENT_SERVICE_UNAVAILABLE", "Deployment service is unavailable", nil)
		return
	}
	releaseID, err := m.api.Releases.PriorDeploymentRelease(r.Context(), project, deploymentID)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	m.createDeployment(w, r, project, releaseID, idempotencyKey, deploymentID)
}

func (m *Module) ListDeploymentEvents(w http.ResponseWriter, r *http.Request, project, deploymentID string, params deploymentapi.PageParams) {
	if m.api.Releases == nil || m.jobs.Coordinator == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "DEPLOYMENT_SERVICE_UNAVAILABLE", "Deployment service is unavailable", nil)
		return
	}
	if _, _, err := m.api.Releases.DeploymentRelease(r.Context(), project, deploymentID); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if _, err := m.jobs.Coordinator.Get(r.Context(), apiadapter.Scope{Project: project, DeploymentID: deploymentID}); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if m.api.Jobs == nil {
		apitransport.WriteProblem(w, r, http.StatusServiceUnavailable, "ASYNC_EVENT_STORE_UNAVAILABLE", "Deployment events are unavailable", nil)
		return
	}
	jobhttp.WriteEventPage(w, r, m.api.Jobs, "deployment", deploymentID, params.Limit, params.PageToken, "deployment:"+project+":"+deploymentID)
}

func (m *Module) principal(r *http.Request) (deploymenthttp.Principal, bool) {
	if m == nil || m.handler == nil {
		return deploymenthttp.Principal{}, false
	}
	return m.handler.Principal(r)
}

func (m *Module) handlerEnvironment() string {
	if m == nil || m.handler == nil {
		return ""
	}
	return m.handler.Environment()
}

func (m *Module) appendAPIEvent(ctx context.Context, deploymentID, eventType string, data any) error {
	if m == nil || m.api.Jobs == nil {
		return errors.New("deployment event store is unavailable")
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = m.api.Jobs.AppendEvent(ctx, "deployment", deploymentID, eventType, encoded)
	return err
}

func deploymentResponse(row apiadapter.Deployment, releaseID, actor string) deploymentapi.Response {
	status := deploymentapi.Status(row.Status)
	if row.Status == apiadapter.StatusPending {
		status = deploymentapi.StatusQueued
	}
	result := deploymentapi.Response{
		ID: row.ID, ProjectID: row.Project, ReleaseID: releaseID, Environment: row.Environment, Status: status,
		CreatedBy: actor, CreatedAt: row.CreatedAt, Targets: make([]deploymentapi.TargetResponse, 0, len(row.Targets)),
		Connections: make([]deploymentapi.ConnectionResponse, 0, len(row.Connections)),
	}
	for _, target := range row.Targets {
		stateID := target.CandidateID
		mapped := deploymentapi.TargetResponse{WorkspaceID: target.Workspace, ServingStateID: &stateID, Status: string(target.Status)}
		if target.PriorCandidateID != "" {
			mapped.PriorServingStateID = &target.PriorCandidateID
		}
		if target.Error != "" {
			mapped.Error = &target.Error
		}
		result.Targets = append(result.Targets, mapped)
	}
	for _, connection := range row.Connections {
		mapped := deploymentapi.ConnectionResponse{ConnectionID: connection.Connection, RevisionID: connection.RevisionID}
		if connection.PriorRevisionID != "" {
			mapped.PriorRevisionID = &connection.PriorRevisionID
		}
		result.Connections = append(result.Connections, mapped)
	}
	if row.ActivatedAt != "" {
		result.StartedAt = &row.ActivatedAt
		result.FinishedAt = &row.ActivatedAt
	}
	if row.Error != "" {
		result.Error = &row.Error
	}
	return result
}

func deploymentLocation(project, deploymentID string) string {
	return "/api/v1/projects/" + project + "/deployments/" + deploymentID
}

func writeAPIError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := http.StatusInternalServerError, "INTERNAL_ERROR"
	switch {
	case errors.Is(err, release.ErrNotFound), errors.Is(err, deployment.ErrNotFound):
		status, code = http.StatusNotFound, "DEPLOYMENT_NOT_FOUND"
	case errors.Is(err, release.ErrConflict), errors.Is(err, deployment.ErrConflict):
		status, code = http.StatusConflict, "DEPLOYMENT_CONFLICT"
	case errors.Is(err, apiadapter.ErrInvalid):
		status, code = http.StatusUnprocessableEntity, "INVALID_DEPLOYMENT"
	}
	detail := err.Error()
	if status == http.StatusInternalServerError {
		detail = "The deployment request could not be completed"
	}
	apitransport.WriteProblem(w, r, status, code, detail, nil)
}
