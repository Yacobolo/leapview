package module

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/flidai/leapview/internal/deployment"
	deploymentapi "github.com/flidai/leapview/internal/deployment/api"
	"github.com/flidai/leapview/internal/platform/digest"
	apitransport "github.com/flidai/leapview/internal/platform/http/transport"
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
	if err := m.candidateSources.Commit(r.Context(), scope, request); err != nil {
		writeCandidateAPIError(w, r, err)
		return
	}
	var candidate deployment.Candidate
	var err error
	if request.ExpectedCandidateID == "" {
		var started deployment.CandidateStartResult
		started, err = m.candidates.Start(r.Context(), deployment.StartCandidateRequest{
			ProjectID: project, OwnerID: principalID, ArtifactDigest: request.ArtifactDigest,
		})
		candidate = started.Candidate
	} else {
		candidate, err = m.candidates.ReplaceArtifact(r.Context(), deployment.CandidateScope{
			ProjectID: project, CandidateID: request.ExpectedCandidateID, OwnerID: principalID,
		}, request.ExpectedArtifactDigest, request.ArtifactDigest)
	}
	if err != nil {
		writeCandidateAPIError(w, r, err)
		return
	}
	apitransport.WriteJSON(w, http.StatusOK, m.candidateResponse(candidate, false))
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
