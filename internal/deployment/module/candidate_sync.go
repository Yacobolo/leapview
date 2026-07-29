package module

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/flidai/leapview/internal/deployment"
	deploymentapi "github.com/flidai/leapview/internal/deployment/api"
	"github.com/flidai/leapview/internal/platform/digest"
	apitransport "github.com/flidai/leapview/internal/platform/http/transport"
	"github.com/flidai/leapview/internal/project"
	"github.com/flidai/leapview/internal/release"
)

const maxCandidateSourceBlobBytes = 16 << 20

func (m *Module) PlanProjectCandidateSynchronization(w http.ResponseWriter, r *http.Request, project string) {
	request, ok := m.decodeCandidateSynchronizationRequest(w, r)
	if !ok {
		return
	}
	principalID, ok := m.candidateSynchronizationPrincipal(w, r)
	if !ok {
		return
	}
	if !m.validateExpectedCandidate(w, r, project, principalID, request) {
		return
	}
	missing, err := m.candidateSources.Plan(r.Context(), deployment.CandidateSourceScope{
		ProjectID: project, OwnerID: principalID,
	}, request)
	if err != nil {
		writeCandidateAPIError(w, r, err)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, deploymentapi.CandidateSynchronizationPlanResponse{
		ArtifactDigest: request.ArtifactDigest, MissingDigests: missing,
	})
}

func (m *Module) UploadProjectCandidateSourceBlob(
	w http.ResponseWriter,
	r *http.Request,
	project, identity, contentType, contentDigest string,
) {
	principalID, ok := m.candidateSynchronizationPrincipal(w, r)
	if !ok {
		return
	}
	identity = strings.TrimSpace(identity)
	if contentType != "application/octet-stream" ||
		digest.ValidateSHA256Identity(identity) != nil ||
		strings.TrimSpace(contentDigest) != candidateSourceContentDigest(identity) {
		apitransport.WriteProblem(
			w, r, http.StatusUnprocessableEntity, "INVALID_CANDIDATE_SOURCE_BLOB",
			"Candidate source blob headers do not match the canonical content identity", nil,
		)
		return
	}
	counter := &candidateSourceCountingReader{source: http.MaxBytesReader(
		w, r.Body, maxCandidateSourceBlobBytes,
	)}
	if err := m.candidateSources.Upload(r.Context(), deployment.CandidateSourceScope{
		ProjectID: project, OwnerID: principalID,
	}, identity, counter); err != nil {
		writeCandidateAPIError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/projects/"+url.PathEscape(strings.TrimSpace(project))+
		"/candidate-sync/blobs/"+url.PathEscape(identity))
	apitransport.WriteJSON(w, http.StatusCreated, deploymentapi.CandidateSourceBlobResponse{
		Digest: identity, SizeBytes: counter.read,
	})
}

func (m *Module) CommitProjectCandidateSynchronization(
	w http.ResponseWriter,
	r *http.Request,
	project, _ string,
) {
	request, ok := m.decodeCandidateSynchronizationRequest(w, r)
	if !ok {
		return
	}
	principalID, ok := m.candidateSynchronizationPrincipal(w, r)
	if !ok {
		return
	}
	if !m.validateExpectedCandidate(w, r, project, principalID, request) {
		return
	}
	scope := deployment.CandidateSourceScope{ProjectID: project, OwnerID: principalID}
	source, err := m.candidateSources.Commit(r.Context(), scope, request)
	if err != nil {
		writeCandidateAPIError(w, r, err)
		return
	}
	var candidate deployment.Candidate
	if request.ExpectedCandidateID == "" {
		var started deployment.CandidateStartResult
		started, err = m.candidates.Start(r.Context(), deployment.StartCandidateRequest{
			ProjectID: project, OwnerID: principalID, ArtifactDigest: request.ArtifactDigest,
		})
		candidate = started.Candidate
		if err == nil && candidate.Status == deployment.CandidateFailed {
			candidate, err = m.candidates.Retry(r.Context(), candidateScope(candidate))
		}
	} else {
		candidate, err = m.candidates.Get(r.Context(), deployment.CandidateScope{
			ProjectID: project, CandidateID: request.ExpectedCandidateID, OwnerID: principalID,
		})
	}
	if err != nil {
		writeCandidateAPIError(w, r, err)
		return
	}
	if candidate.Status == deployment.CandidateReady &&
		candidate.ArtifactDigest == request.ArtifactDigest {
		apitransport.WriteJSON(w, http.StatusOK, m.candidateResponse(candidate, false))
		return
	}
	tentative, err := tentativeCandidate(candidate, request)
	if err != nil {
		writeCandidateAPIError(w, r, err)
		return
	}
	if err := m.prepareCandidate(r.Context(), tentative, source); err != nil {
		if request.ExpectedCandidateID == "" {
			_, _ = m.candidates.MarkFailed(
				r.Context(),
				candidateScope(candidate),
				candidate.ArtifactDigest,
				"CANDIDATE_PREPARATION_FAILED",
			)
		}
		writeCandidateAPIError(w, r, candidatePreparationError(err))
		return
	}
	if request.ExpectedCandidateID != "" {
		candidate, err = m.candidates.ReplaceArtifact(
			r.Context(),
			candidateScope(candidate),
			request.ExpectedArtifactDigest,
			request.ArtifactDigest,
		)
		if err != nil {
			writeCandidateAPIError(w, r, err)
			return
		}
	}
	candidate, err = m.candidates.MarkReady(
		r.Context(),
		candidateScope(candidate),
		request.ArtifactDigest,
	)
	if err != nil {
		writeCandidateAPIError(w, r, err)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, m.candidateResponse(candidate, false))
}

func (m *Module) prepareCandidate(
	ctx context.Context,
	candidate deployment.Candidate,
	source project.CandidateSourceSnapshot,
) error {
	if m == nil || m.candidateArtifacts == nil || m.candidateRuntimes == nil {
		return deployment.ErrCandidateUnavailable
	}
	artifacts, err := m.candidateArtifacts.PrepareCandidateArtifacts(
		ctx,
		release.CandidateArtifactRequest{
			CandidateID: candidate.ID, ProjectID: candidate.ProjectID,
			OwnerID: candidate.OwnerID, Environment: candidate.Environment,
			ArtifactDigest: candidate.ArtifactDigest, Source: source,
		},
	)
	if err != nil {
		return err
	}
	workspaces := make([]deployment.CandidateWorkspaceRuntime, len(artifacts.Workspaces))
	for index, workspace := range artifacts.Workspaces {
		requirements := make(
			[]deployment.CandidateConnectionRequirement,
			len(workspace.Connections),
		)
		for requirementIndex, requirement := range workspace.Connections {
			requirements[requirementIndex] = deployment.CandidateConnectionRequirement{
				LogicalConnectionID: requirement.LogicalConnectionID,
				ConnectorKind:       requirement.ConnectorKind,
			}
		}
		workspaces[index] = deployment.CandidateWorkspaceRuntime{
			WorkspaceID: workspace.WorkspaceID, ServingStateID: workspace.ServingStateID,
			ArtifactDigest: workspace.ArtifactDigest, DataRevision: workspace.DataRevision,
			DataMode: deployment.CandidateDataMode(workspace.DataMode), Connections: requirements,
			Restrictions: candidateRuntimeRestrictions(workspace.Restrictions),
		}
	}
	return m.candidateRuntimes.Prepare(ctx, deployment.CandidateRuntimeRequest{
		Candidate: candidate, AuthorizationFingerprint: artifacts.AuthorizationFingerprint,
		Workspaces: workspaces,
	})
}

func candidateRuntimeRestrictions(values []release.CandidateRestriction) []deployment.CandidateRestriction {
	result := make([]deployment.CandidateRestriction, len(values))
	for index, value := range values {
		result[index] = deployment.CandidateRestriction{
			ID: value.ID, WorkspaceID: value.WorkspaceID, ObjectID: value.ObjectID,
			PolicyType: value.PolicyType, ExpressionJSON: value.ExpressionJSON,
		}
	}
	return result
}

func tentativeCandidate(
	candidate deployment.Candidate,
	request deployment.CandidateSynchronizationRequest,
) (deployment.Candidate, error) {
	if request.ExpectedCandidateID == "" {
		if candidate.Status != deployment.CandidatePreparing {
			return deployment.Candidate{}, deployment.ErrCandidateConflict
		}
		return candidate, nil
	}
	if candidate.Status != deployment.CandidateReady ||
		candidate.ArtifactDigest != strings.TrimSpace(request.ExpectedArtifactDigest) {
		return deployment.Candidate{}, deployment.ErrCandidateConflict
	}
	candidate.ArtifactDigest = strings.TrimSpace(request.ArtifactDigest)
	candidate.Status = deployment.CandidatePreparing
	candidate.FailureReason = ""
	candidate.ReadyAt = time.Time{}
	return candidate, nil
}

func candidateScope(candidate deployment.Candidate) deployment.CandidateScope {
	return deployment.CandidateScope{
		ProjectID: candidate.ProjectID, CandidateID: candidate.ID,
		OwnerID: candidate.OwnerID, TargetID: candidate.TargetID,
	}
}

func candidatePreparationError(err error) error {
	switch {
	case errors.Is(err, release.ErrCandidateArtifactInvalid):
		return fmt.Errorf("%w: %v", deployment.ErrCandidateInvalid, err)
	case errors.Is(err, release.ErrCandidateArtifactUnavailable):
		return fmt.Errorf("%w: %v", deployment.ErrCandidateUnavailable, err)
	default:
		return err
	}
}

func (m *Module) decodeCandidateSynchronizationRequest(
	w http.ResponseWriter,
	r *http.Request,
) (deployment.CandidateSynchronizationRequest, bool) {
	var body deploymentapi.CandidateSynchronizationRequest
	if err := apitransport.DecodeBody(w, r, &body); err != nil {
		apitransport.WriteProblem(w, r, http.StatusBadRequest, "INVALID_JSON", err.Error(), nil)
		return deployment.CandidateSynchronizationRequest{}, false
	}
	request := deployment.CandidateSynchronizationRequest{
		ProjectFile: body.ProjectFile, ArtifactDigest: body.ArtifactDigest,
		Artifacts: make([]deployment.CandidateSourceArtifact, len(body.Artifacts)),
	}
	if body.ExpectedCandidateID != nil {
		request.ExpectedCandidateID = *body.ExpectedCandidateID
	}
	if body.ExpectedArtifactDigest != nil {
		request.ExpectedArtifactDigest = *body.ExpectedArtifactDigest
	}
	for index, artifact := range body.Artifacts {
		request.Artifacts[index] = deployment.CandidateSourceArtifact{
			Path: artifact.Path, Digest: artifact.Digest,
		}
	}
	return request, true
}

func (m *Module) candidateSynchronizationPrincipal(
	w http.ResponseWriter,
	r *http.Request,
) (string, bool) {
	principalID, ok := m.candidatePrincipalID(w, r)
	if !ok {
		return "", false
	}
	if m.candidateSources == nil {
		writeCandidateUnavailable(w, r)
		return "", false
	}
	return principalID, true
}

func (m *Module) validateExpectedCandidate(
	w http.ResponseWriter,
	r *http.Request,
	project, principalID string,
	request deployment.CandidateSynchronizationRequest,
) bool {
	hasID := strings.TrimSpace(request.ExpectedCandidateID) != ""
	hasDigest := strings.TrimSpace(request.ExpectedArtifactDigest) != ""
	if hasID != hasDigest {
		writeCandidateAPIError(w, r, fmt.Errorf(
			"%w: expected candidate identity and digest must be supplied together",
			deployment.ErrCandidateInvalid,
		))
		return false
	}
	if !hasID {
		return true
	}
	candidate, err := m.candidates.Get(r.Context(), deployment.CandidateScope{
		ProjectID: project, CandidateID: request.ExpectedCandidateID, OwnerID: principalID,
	})
	if err != nil {
		writeCandidateAPIError(w, r, err)
		return false
	}
	if candidate.ArtifactDigest != strings.TrimSpace(request.ExpectedArtifactDigest) {
		writeCandidateAPIError(w, r, deployment.ErrCandidateConflict)
		return false
	}
	return true
}

func candidateSourceContentDigest(identity string) string {
	decoded, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(identity), "sha256:"))
	if err != nil || len(decoded) != 32 {
		return ""
	}
	return "sha-256=:" + base64.StdEncoding.EncodeToString(decoded) + ":"
}

type candidateSourceCountingReader struct {
	source io.Reader
	read   int64
}

func (reader *candidateSourceCountingReader) Read(buffer []byte) (int, error) {
	count, err := reader.source.Read(buffer)
	reader.read += int64(count)
	return count, err
}
