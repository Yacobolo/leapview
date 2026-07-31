package release

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/flidai/leapview/internal/platform/jobs"
	"github.com/flidai/leapview/internal/servingstate"
	"github.com/flidai/leapview/internal/workspace"
	ocidigest "github.com/opencontainers/go-digest"
)

type Repository interface {
	Create(context.Context, CreateInput) (Release, error)
	Get(context.Context, string, string) (Release, error)
	List(context.Context, string) ([]Release, error)
	AssignArtifactTarget(context.Context, string, string, string, string) error
	RecordArtifact(context.Context, Artifact) error
}

// FinalizationUnitOfWork owns release finalization state transitions. The
// SQLite implementation verifies expected artifact digests and commits the
// ready projection atomically.
type FinalizationUnitOfWork interface {
	BeginFinalization(context.Context, string, string, jobs.WorkflowIntent) (Release, error)
	CompleteFinalization(context.Context, string, string, map[string]string) (Release, error)
	FailFinalization(context.Context, string, string, error) (Release, error)
}

type ServingStateRepository interface {
	Create(context.Context, servingstate.CreateInput) (servingstate.State, error)
}

type WorkspaceRepository interface {
	Ensure(context.Context, workspace.EnsureInput) error
}

type ArtifactStore interface {
	SaveUpload(context.Context, servingstate.ID, io.Reader) (int64, error)
}

type ArtifactValidator interface {
	Validate(context.Context, servingstate.ID) (servingstate.State, error)
}

type PinValidator interface {
	ValidateServingStatePins(context.Context, string, string, map[string]string) error
}

type CandidatePinResolver interface {
	ResolveCandidatePins(context.Context, string, []string, string) (map[string]string, error)
}

type ManagedDataPins interface {
	PinValidator
	CandidatePinResolver
}

type CandidateProvenanceRepository interface {
	RetainCandidateProvenance(
		context.Context,
		string,
		Provenance,
	) (Provenance, error)
	CandidateProvenance(
		context.Context,
		string,
		string,
		int64,
	) (Provenance, error)
}

type Service struct {
	releases            Repository
	finalization        FinalizationUnitOfWork
	states              ServingStateRepository
	workspaces          WorkspaceRepository
	artifacts           ArtifactStore
	validator           ArtifactValidator
	pins                PinValidator
	candidateProvenance CandidateProvenanceRepository
	environment         servingstate.Environment
}

type ServiceOptions struct {
	Releases            Repository
	Finalization        FinalizationUnitOfWork
	States              ServingStateRepository
	Workspaces          WorkspaceRepository
	Artifacts           ArtifactStore
	Validator           ArtifactValidator
	Pins                PinValidator
	CandidateProvenance CandidateProvenanceRepository
	Environment         servingstate.Environment
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Releases == nil || options.Finalization == nil || options.States == nil || options.Workspaces == nil || options.Artifacts == nil || options.Validator == nil {
		return nil, fmt.Errorf("release repository, finalization unit of work, artifact store, and validator are required")
	}
	return &Service{
		releases: options.Releases, finalization: options.Finalization,
		states: options.States, workspaces: options.Workspaces,
		artifacts: options.Artifacts, validator: options.Validator,
		pins:                options.Pins,
		candidateProvenance: options.CandidateProvenance,
		environment:         servingstate.NormalizeEnvironment(options.Environment),
	}, nil
}

func (s *Service) RetainCandidateProvenance(
	ctx context.Context,
	projectID string,
	provenance Provenance,
) (Provenance, error) {
	if s == nil || s.candidateProvenance == nil {
		return Provenance{}, ErrCandidateArtifactUnavailable
	}
	if strings.TrimSpace(projectID) == "" {
		return Provenance{}, ErrInvalid
	}
	if err := provenance.Validate(); err != nil {
		return Provenance{}, err
	}
	return s.candidateProvenance.RetainCandidateProvenance(
		ctx,
		strings.TrimSpace(projectID),
		provenance,
	)
}

func (s *Service) CandidateProvenance(
	ctx context.Context,
	projectID,
	candidateID string,
	candidateRevision int64,
) (Provenance, error) {
	if s == nil || s.candidateProvenance == nil {
		return Provenance{}, ErrCandidateArtifactUnavailable
	}
	return s.candidateProvenance.CandidateProvenance(
		ctx,
		strings.TrimSpace(projectID),
		strings.TrimSpace(candidateID),
		candidateRevision,
	)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Release, error) {
	input.ID = stableID("rel", input.ProjectID, input.IdempotencyKey)
	manifest := Manifest{Workspaces: input.Workspaces, Connections: input.Connections}
	if input.Provenance != nil {
		if err := input.Provenance.Validate(); err != nil {
			return Release{}, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		if input.Provenance.Artifact.SourceDigest != strings.TrimSpace(input.ProjectDigest) {
			return Release{}, fmt.Errorf("%w: provenance source digest does not match release project digest", ErrInvalid)
		}
		planned := make(
			map[string]TargetWorkspacePlan,
			len(input.Provenance.Plan.Workspaces),
		)
		for _, workspace := range input.Provenance.Plan.Workspaces {
			planned[workspace.WorkspaceID] = workspace
		}
		if len(planned) != len(input.Workspaces) {
			return Release{}, fmt.Errorf("%w: provenance workspace set does not match release", ErrInvalid)
		}
		for _, workspace := range input.Workspaces {
			plan, ok := planned[workspace.WorkspaceID]
			if !ok ||
				plan.ArtifactDigest != provenanceWorkspaceArtifactDigest(workspace.ArtifactDigest) ||
				plan.ServingStateID != workspace.ServingStateID {
				return Release{}, fmt.Errorf("%w: provenance target plan does not match release", ErrInvalid)
			}
		}
	}
	encoded, err := json.Marshal(struct {
		Manifest   Manifest    `json:"manifest"`
		Provenance *Provenance `json:"provenance,omitempty"`
	}{Manifest: manifest, Provenance: input.Provenance})
	if err != nil {
		return Release{}, err
	}
	input.RequestDigest = ocidigest.FromBytes(encoded).String()
	created, err := s.releases.Create(ctx, input)
	if err != nil {
		return Release{}, err
	}
	for _, artifact := range created.Artifacts {
		if artifact.ServingStateID != "" {
			continue
		}
		var retainedStateID string
		for _, workspace := range input.Workspaces {
			if workspace.WorkspaceID == artifact.WorkspaceID {
				retainedStateID = workspace.ServingStateID
				break
			}
		}
		if retainedStateID != "" {
			if err := s.releases.AssignArtifactTarget(
				ctx,
				created.ProjectID,
				created.ID,
				artifact.WorkspaceID,
				retainedStateID,
			); err != nil {
				return Release{}, err
			}
			continue
		}
		if err := s.workspaces.Ensure(ctx, workspace.EnsureInput{ID: workspace.WorkspaceID(artifact.WorkspaceID), Title: artifact.WorkspaceID}); err != nil {
			return Release{}, err
		}
		state, err := s.states.Create(ctx, servingstate.CreateInput{WorkspaceID: servingstate.WorkspaceID(artifact.WorkspaceID), ProjectID: created.ProjectID, Environment: s.environment, CreatedBy: created.CreatedBy})
		if err != nil {
			return Release{}, err
		}
		if err := s.releases.AssignArtifactTarget(ctx, created.ProjectID, created.ID, artifact.WorkspaceID, string(state.ID)); err != nil {
			return Release{}, err
		}
	}
	return s.releases.Get(ctx, created.ProjectID, created.ID)
}

func provenanceWorkspaceArtifactDigest(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "sha256:") {
		return value
	}
	return "sha256:" + value
}

func (s *Service) Get(ctx context.Context, projectID, releaseID string) (Release, error) {
	return s.releases.Get(ctx, projectID, releaseID)
}

func (s *Service) List(ctx context.Context, projectID string) ([]Release, error) {
	return s.releases.List(ctx, projectID)
}

func (s *Service) UploadArtifact(ctx context.Context, projectID, releaseID, workspaceID, contentDigest string, source io.Reader) (Artifact, error) {
	current, err := s.releases.Get(ctx, projectID, releaseID)
	if err != nil {
		return Artifact{}, err
	}
	if current.Status != StatusDraft {
		return Artifact{}, ErrImmutable
	}
	var target Artifact
	found := false
	for _, artifact := range current.Artifacts {
		if artifact.WorkspaceID == workspaceID {
			target, found = artifact, true
			break
		}
	}
	if !found || target.ServingStateID == "" {
		return Artifact{}, ErrNotFound
	}
	expectedDigest := ocidigest.NewDigestFromEncoded(
		ocidigest.SHA256,
		strings.TrimSpace(target.ExpectedDigest),
	)
	if err := expectedDigest.Validate(); err != nil {
		return Artifact{}, ErrInvalid
	}
	expectedDigestBytes, err := hex.DecodeString(expectedDigest.Encoded())
	if err != nil {
		return Artifact{}, ErrInvalid
	}
	expectedContentDigest := "sha-256=:" + base64.StdEncoding.EncodeToString(expectedDigestBytes) + ":"
	if strings.TrimSpace(contentDigest) != expectedContentDigest {
		return Artifact{}, ErrDigest
	}
	if target.UploadedAt != "" {
		return target, nil
	}
	verifier := expectedDigest.Verifier()
	size, err := s.artifacts.SaveUpload(
		ctx,
		servingstate.ID(target.ServingStateID),
		io.TeeReader(source, verifier),
	)
	if err != nil {
		return Artifact{}, err
	}
	if !verifier.Verified() {
		return Artifact{}, ErrDigest
	}
	target.SizeBytes = size
	if err := s.releases.RecordArtifact(ctx, target); err != nil {
		return Artifact{}, err
	}
	return target, nil
}

func (s *Service) Finalize(ctx context.Context, projectID, releaseID string) (Release, error) {
	if _, err := s.BeginFinalization(ctx, projectID, releaseID, jobs.WorkflowIntent{}); err != nil {
		return Release{}, err
	}
	return s.ValidateFinalization(ctx, projectID, releaseID)
}

func (s *Service) BeginFinalization(ctx context.Context, projectID, releaseID string, workflow jobs.WorkflowIntent) (Release, error) {
	return s.finalization.BeginFinalization(ctx, projectID, releaseID, workflow)
}

func (s *Service) ValidateFinalization(ctx context.Context, projectID, releaseID string) (Release, error) {
	current, err := s.releases.Get(ctx, projectID, releaseID)
	if err != nil {
		return Release{}, err
	}
	if current.Status == StatusReady {
		return current, nil
	}
	if current.Status == StatusFailed {
		return current, fmt.Errorf("%w: %s", ErrConflict, current.Error)
	}
	if current.Status != StatusValidating {
		return Release{}, ErrConflict
	}
	digests := make(map[string]string, len(current.Artifacts))
	expectedPins := make(map[string]string, len(current.Manifest.Connections))
	for _, pin := range current.Manifest.Connections {
		if pin.ConnectionID == "" || pin.RevisionID == "" {
			return s.failFinalization(ctx, current, ErrInvalid)
		}
		if _, duplicate := expectedPins[pin.ConnectionID]; duplicate {
			return s.failFinalization(ctx, current, ErrInvalid)
		}
		expectedPins[pin.ConnectionID] = pin.RevisionID
	}
	if len(expectedPins) > 0 && s.pins == nil {
		return s.failFinalization(ctx, current, fmt.Errorf("%w: managed-data pin validation is unavailable", ErrConflict))
	}
	for _, artifact := range current.Artifacts {
		state, validateErr := s.validator.Validate(ctx, servingstate.ID(artifact.ServingStateID))
		if validateErr != nil {
			return s.failFinalization(ctx, current, validateErr)
		}
		if state.ProjectID != current.ProjectID || state.ProjectDigest != current.ProjectDigest || state.Digest != artifact.ExpectedDigest {
			mismatch := fmt.Errorf("%w: artifact %q does not match the release manifest", ErrConflict, artifact.WorkspaceID)
			return s.failFinalization(ctx, current, mismatch)
		}
		if s.pins != nil {
			if pinErr := s.pins.ValidateServingStatePins(ctx, artifact.ServingStateID, current.ProjectID, expectedPins); pinErr != nil {
				return s.failFinalization(ctx, current, pinErr)
			}
		}
		digests[artifact.WorkspaceID] = state.Digest
	}
	return s.finalization.CompleteFinalization(ctx, projectID, releaseID, digests)
}

func (s *Service) failFinalization(ctx context.Context, current Release, cause error) (Release, error) {
	failed, failErr := s.finalization.FailFinalization(ctx, current.ProjectID, current.ID, cause)
	if failErr != nil {
		return Release{}, errorsJoin(cause, failErr)
	}
	return failed, cause
}

func stableID(prefix string, values ...string) string {
	hash := sha256.New()
	for _, value := range values {
		_, _ = hash.Write([]byte(strings.TrimSpace(value)))
		_, _ = hash.Write([]byte{0})
	}
	return prefix + "_" + hex.EncodeToString(hash.Sum(nil))[:24]
}

func errorsJoin(primary, secondary error) error {
	return fmt.Errorf("%v; persist failure: %w", primary, secondary)
}
