package http

import (
	"context"
	"log/slog"
	stdhttp "net/http"

	deploymentgen "github.com/flidai/leapview/internal/deployment/api/gen"
	apitransport "github.com/flidai/leapview/internal/platform/http/transport"
)

type APIGenHandler interface {
	UploadProjectCandidateSourceBlob(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, string)
	CommitProjectCandidateSynchronization(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	PlanProjectCandidateSynchronization(stdhttp.ResponseWriter, *stdhttp.Request, string)
	StartProjectCandidate(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	GetProjectCandidate(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	ReplaceProjectCandidateArtifact(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
	RetryProjectCandidate(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
	CancelProjectCandidate(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
	PublishProjectCandidate(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
	ReviewProjectCandidate(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	ListDeployments(stdhttp.ResponseWriter, *stdhttp.Request, string, *int32, *string)
	CreateDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	GetDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	CancelDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	RetryDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
	ListDeploymentEvents(stdhttp.ResponseWriter, *stdhttp.Request, string, string, *int32, *string)
	RollbackDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
	RequestDeploymentApproval(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
	ApproveDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, string)
	DenyDeploymentApproval(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, string)
	RevokeDeploymentApproval(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string, string)
	ActivateDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string)
}

func (d *APIGenDispatcher) UploadProjectCandidateSourceBlob(w stdhttp.ResponseWriter, r *stdhttp.Request, project, digest string, headers deploymentgen.GenUploadProjectCandidateSourceBlobHeaders) {
	d.handler.UploadProjectCandidateSourceBlob(w, r, project, digest, headers.ContentType, headers.ContentDigest)
}

func (d *APIGenDispatcher) CommitProjectCandidateSynchronization(w stdhttp.ResponseWriter, r *stdhttp.Request, project string, headers deploymentgen.GenCommitProjectCandidateSynchronizationHeaders) {
	d.handler.CommitProjectCandidateSynchronization(w, r, project, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) PlanProjectCandidateSynchronization(w stdhttp.ResponseWriter, r *stdhttp.Request, project string, _ deploymentgen.GenPlanProjectCandidateSynchronizationHeaders) {
	d.handler.PlanProjectCandidateSynchronization(w, r, project)
}

type APIGenDispatcher struct{ handler APIGenHandler }

func NewAPIGenDispatcher(handler APIGenHandler) *APIGenDispatcher {
	return &APIGenDispatcher{handler: handler}
}

func (d *APIGenDispatcher) StartProjectCandidate(w stdhttp.ResponseWriter, r *stdhttp.Request, project string, headers deploymentgen.GenStartProjectCandidateHeaders) {
	d.handler.StartProjectCandidate(w, r, project, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) GetProjectCandidate(w stdhttp.ResponseWriter, r *stdhttp.Request, project, candidate string) {
	d.handler.GetProjectCandidate(w, r, project, candidate)
}

func (d *APIGenDispatcher) ReplaceProjectCandidateArtifact(w stdhttp.ResponseWriter, r *stdhttp.Request, project, candidate string, headers deploymentgen.GenReplaceProjectCandidateArtifactHeaders) {
	d.handler.ReplaceProjectCandidateArtifact(w, r, project, candidate, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) RetryProjectCandidate(w stdhttp.ResponseWriter, r *stdhttp.Request, project, candidate string, headers deploymentgen.GenRetryProjectCandidateHeaders) {
	d.handler.RetryProjectCandidate(w, r, project, candidate, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) CancelProjectCandidate(w stdhttp.ResponseWriter, r *stdhttp.Request, project, candidate string, headers deploymentgen.GenCancelProjectCandidateHeaders) {
	d.handler.CancelProjectCandidate(w, r, project, candidate, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) PublishProjectCandidate(w stdhttp.ResponseWriter, r *stdhttp.Request, project, candidate string, headers deploymentgen.GenPublishProjectCandidateHeaders) {
	d.handler.PublishProjectCandidate(w, r, project, candidate, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) ReviewProjectCandidate(w stdhttp.ResponseWriter, r *stdhttp.Request, project, candidate string) {
	d.handler.ReviewProjectCandidate(w, r, project, candidate)
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

func (d *APIGenDispatcher) RetryDeployment(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment string, headers deploymentgen.GenRetryDeploymentHeaders) {
	d.handler.RetryDeployment(w, r, project, deployment, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) ListDeploymentEvents(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment string, params deploymentgen.GenListDeploymentEventsParams, _ deploymentgen.GenListDeploymentEventsHeaders) {
	d.handler.ListDeploymentEvents(w, r, project, deployment, params.Limit, params.PageToken)
}

func (d *APIGenDispatcher) RollbackDeployment(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment string, headers deploymentgen.GenRollbackDeploymentHeaders) {
	d.handler.RollbackDeployment(w, r, project, deployment, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) RequestDeploymentApproval(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment string, headers deploymentgen.GenRequestDeploymentApprovalHeaders) {
	d.handler.RequestDeploymentApproval(w, r, project, deployment, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) ApproveDeployment(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment, approval string, headers deploymentgen.GenApproveDeploymentHeaders) {
	d.handler.ApproveDeployment(w, r, project, deployment, approval, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) DenyDeploymentApproval(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment, approval string, headers deploymentgen.GenDenyDeploymentApprovalHeaders) {
	d.handler.DenyDeploymentApproval(w, r, project, deployment, approval, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) RevokeDeploymentApproval(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment, approval string, headers deploymentgen.GenRevokeDeploymentApprovalHeaders) {
	d.handler.RevokeDeploymentApproval(w, r, project, deployment, approval, headers.IdempotencyKey)
}

func (d *APIGenDispatcher) ActivateDeployment(w stdhttp.ResponseWriter, r *stdhttp.Request, project, deployment string, headers deploymentgen.GenActivateDeploymentHeaders) {
	d.handler.ActivateDeployment(w, r, project, deployment, headers.IdempotencyKey)
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
