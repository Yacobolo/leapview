package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/platform/jobs"
	"github.com/flidai/leapview/internal/servingstate"
	"github.com/stretchr/testify/require"
)

func TestUploadArtifactReplaysAlreadyRecordedContent(t *testing.T) {
	content := []byte("compiled workspace artifact")
	sum := sha256.Sum256(content)
	digest := fmt.Sprintf("%x", sum[:])
	repo := &serviceTestReleaseRepository{current: Release{
		ID: "release-1", ProjectID: "project-a", Status: StatusDraft,
		Artifacts: []Artifact{{
			ReleaseID: "release-1", WorkspaceID: "sales", ServingStateID: "state-1",
			ExpectedDigest: digest, SizeBytes: int64(len(content)), UploadedAt: "2026-07-27T19:00:00Z",
		}},
	}}
	artifacts := &serviceTestArtifactStore{}
	service := &Service{releases: repo, artifacts: artifacts}

	got, err := service.UploadArtifact(
		t.Context(),
		"project-a",
		"release-1",
		"sales",
		"sha-256=:"+base64Digest(content)+":",
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf("UploadArtifact() error = %v", err)
	}
	if got.SizeBytes != int64(len(content)) {
		t.Fatalf("UploadArtifact() = %#v, saved %q", got, artifacts.saved)
	}
	if artifacts.saveCalls != 0 {
		t.Fatalf("idempotent replay rewrote the retained upload %d times", artifacts.saveCalls)
	}
	if repo.recorded {
		t.Fatal("idempotent replay attempted to record the artifact twice")
	}
}

func TestUploadArtifactRejectsDifferentContentAfterArtifactWasRecorded(t *testing.T) {
	original := []byte("original artifact")
	replacement := []byte("different artifact")
	sum := sha256.Sum256(original)
	repo := &serviceTestReleaseRepository{current: Release{
		ID: "release-1", ProjectID: "project-a", Status: StatusDraft,
		Artifacts: []Artifact{{
			ReleaseID: "release-1", WorkspaceID: "sales", ServingStateID: "state-1",
			ExpectedDigest: fmt.Sprintf("%x", sum[:]), SizeBytes: int64(len(original)), UploadedAt: "2026-07-27T19:00:00Z",
		}},
	}}
	service := &Service{releases: repo, artifacts: &serviceTestArtifactStore{}}

	_, err := service.UploadArtifact(
		t.Context(),
		"project-a",
		"release-1",
		"sales",
		"sha-256=:"+base64Digest(replacement)+":",
		bytes.NewReader(replacement),
	)
	if !errors.Is(err, ErrDigest) {
		t.Fatalf("UploadArtifact() error = %v, want ErrDigest", err)
	}
}

func TestUploadArtifactRejectsMalformedExpectedDigestWithoutSaving(t *testing.T) {
	repo := &serviceTestReleaseRepository{current: Release{
		ID: "release-1", ProjectID: "project-a", Status: StatusDraft,
		Artifacts: []Artifact{{
			ReleaseID: "release-1", WorkspaceID: "sales", ServingStateID: "state-1",
			ExpectedDigest: "not-a-sha256-digest",
		}},
	}}
	artifacts := &serviceTestArtifactStore{}
	service := &Service{releases: repo, artifacts: artifacts}

	_, err := service.UploadArtifact(
		t.Context(),
		"project-a",
		"release-1",
		"sales",
		"sha-256=:invalid:",
		strings.NewReader("artifact"),
	)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("UploadArtifact() error = %v, want ErrInvalid", err)
	}
	if artifacts.saveCalls != 0 {
		t.Fatalf("malformed expected digest saved %d uploads", artifacts.saveCalls)
	}
}

func TestValidateFinalizationRequiresEveryArtifactToMatchReleaseConnectionPins(t *testing.T) {
	pinErr := errors.New("artifact pins disagree with release manifest")
	repo := &serviceTestReleaseRepository{current: Release{
		ID: "release-1", ProjectID: "project-a", ProjectDigest: "sha256:project", Status: StatusValidating,
		Manifest:  Manifest{Connections: []ConnectionPin{{ConnectionID: "orders", RevisionID: "sha256:orders"}}},
		Artifacts: []Artifact{{WorkspaceID: "sales", ServingStateID: "state-1", ExpectedDigest: "sha256:artifact"}},
	}}
	pins := &serviceTestPinValidator{err: pinErr}
	service := &Service{
		releases:     repo,
		finalization: repo,
		validator: serviceTestArtifactValidator{state: servingstate.State{
			ID: "state-1", ProjectID: "project-a", ProjectDigest: "sha256:project", Digest: "sha256:artifact",
		}},
		pins: pins,
	}

	got, err := service.ValidateFinalization(t.Context(), "project-a", "release-1")
	if !errors.Is(err, pinErr) || got.Status != StatusFailed {
		t.Fatalf("ValidateFinalization() = status %q, error %v", got.Status, err)
	}
	if repo.completed {
		t.Fatal("release became ready despite mismatched managed-data pins")
	}
	want := map[string]string{"orders": "sha256:orders"}
	if pins.stateID != "state-1" || pins.projectID != "project-a" || !reflect.DeepEqual(pins.expected, want) {
		t.Fatalf("pin validation = state %q project %q pins %#v, want state-1 project-a %#v", pins.stateID, pins.projectID, pins.expected, want)
	}
}

func TestValidateFinalizationReplaysReadyRelease(t *testing.T) {
	repo := &serviceTestReleaseRepository{current: Release{
		ID: "release-1", ProjectID: "project-a", Status: StatusReady,
	}}
	service := &Service{releases: repo, finalization: repo}

	got, err := service.ValidateFinalization(t.Context(), "project-a", "release-1")
	if err != nil || got.Status != StatusReady {
		t.Fatalf("ValidateFinalization() replay = status %q, error %v", got.Status, err)
	}
	if repo.completed {
		t.Fatal("ready release replay attempted finalization again")
	}
}

func TestPublishCandidatePromotesExactRetainedProvenanceWithoutRebuilding(t *testing.T) {
	provenance, err := NewProvenance(ProvenanceInput{
		Artifact: ProjectArtifactProvenance{
			SourceDigest:    "sha256:" + strings.Repeat("1", 64),
			ProjectDigest:   "sha256:" + strings.Repeat("2", 64),
			CompilerVersion: "leapview:test", SchemaVersion: 3,
			Workspaces: []WorkspaceArtifactProvenance{{
				WorkspaceID:    "sales",
				ArtifactDigest: "sha256:" + strings.Repeat("3", 64),
			}},
		},
		Candidate: CandidateProvenance{
			ID: "candidate_1", Revision: 4, OwnerID: "publisher",
		},
		Plan: TargetPlanProvenance{
			TargetID: "lvinst_dev", Environment: "dev",
			BaseGeneration: "deployment_7", RuntimeVersion: "runtime:test",
			PolicyDigest: "sha256:" + strings.Repeat("4", 64),
			Workspaces: []TargetWorkspacePlan{{
				WorkspaceID: "sales", ServingStateID: "state_candidate",
				ArtifactDigest: "sha256:" + strings.Repeat("5", 64),
				DataRevision:   "snapshot:17", DataMode: TargetDataReuseSnapshot,
				ManagedDataPins: []ManagedDataPin{{
					ConnectionID: "olist",
					RevisionID:   "sha256:" + strings.Repeat("6", 64),
				}},
			}},
		},
	})
	require.NoError(t, err)
	repository := &serviceTestReleaseRepository{}
	service := &Service{
		releases: repository, finalization: repository,
		validator: serviceTestArtifactValidator{state: servingstate.State{
			ID: "state_candidate", ProjectID: "commerce",
			ProjectDigest: provenance.Artifact.SourceDigest,
			Digest:        strings.TrimPrefix(provenance.Plan.Workspaces[0].ArtifactDigest, "sha256:"),
		}},
		pins: &serviceTestPinValidator{},
		candidateProvenance: serviceTestCandidateProvenanceRepository{
			provenance: provenance,
		},
	}

	published, err := service.PublishCandidate(t.Context(), PublishCandidateInput{
		ProjectID: "commerce", CandidateID: "candidate_1",
		CandidateRevision: 4, ProvenanceDigest: provenance.Digest,
		TargetID: "lvinst_dev", Environment: "dev",
		IdempotencyKey: "publish-1", CreatedBy: "publisher",
	})
	require.NoError(t, err)
	if published.Status != StatusReady || published.Provenance == nil ||
		published.Provenance.Digest != provenance.Digest {
		t.Fatalf("published release = %#v", published)
	}
	if repository.created.ProjectDigest != provenance.Artifact.SourceDigest ||
		repository.created.Workspaces[0].ServingStateID != "state_candidate" ||
		repository.created.Workspaces[0].ArtifactDigest != strings.Repeat("5", 64) {
		t.Fatalf("candidate promotion rebuilt or retargeted artifacts: %#v", repository.created)
	}
}

func TestPublishCandidateRejectsClientOrTargetDrift(t *testing.T) {
	provenance := candidateServiceTestProvenance(t)
	service := &Service{
		candidateProvenance: serviceTestCandidateProvenanceRepository{
			provenance: provenance,
		},
	}
	for name, mutate := range map[string]func(*PublishCandidateInput){
		"candidate revision": func(input *PublishCandidateInput) {
			input.CandidateRevision++
		},
		"provenance digest": func(input *PublishCandidateInput) {
			input.ProvenanceDigest = "sha256:" + strings.Repeat("f", 64)
		},
		"target": func(input *PublishCandidateInput) {
			input.TargetID = "lvinst_other"
		},
		"environment": func(input *PublishCandidateInput) {
			input.Environment = "prod"
		},
		"owner": func(input *PublishCandidateInput) {
			input.CreatedBy = "other"
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := PublishCandidateInput{
				ProjectID: "commerce", CandidateID: provenance.Candidate.ID,
				CandidateRevision: provenance.Candidate.Revision,
				ProvenanceDigest:  provenance.Digest,
				TargetID:          provenance.Plan.TargetID,
				Environment:       provenance.Plan.Environment,
				IdempotencyKey:    "publish-1",
				CreatedBy:         provenance.Candidate.OwnerID,
			}
			mutate(&input)
			if _, err := service.PublishCandidate(t.Context(), input); !errors.Is(err, ErrConflict) {
				t.Fatalf("PublishCandidate() error = %v, want ErrConflict", err)
			}
		})
	}
}

type serviceTestReleaseRepository struct {
	current   Release
	created   CreateInput
	completed bool
	recorded  bool
}

func (r *serviceTestReleaseRepository) Create(_ context.Context, input CreateInput) (Release, error) {
	r.created = input
	if r.current.ID != "" {
		return r.current, nil
	}
	r.current = Release{
		ID:        stableID("rel", input.ProjectID, input.IdempotencyKey),
		ProjectID: input.ProjectID, ProjectDigest: input.ProjectDigest,
		Status: StatusDraft, CreatedBy: input.CreatedBy,
		Manifest: Manifest{
			Workspaces:  append([]WorkspaceManifest(nil), input.Workspaces...),
			Connections: append([]ConnectionPin(nil), input.Connections...),
		},
		Provenance: input.Provenance,
	}
	for _, workspace := range input.Workspaces {
		r.current.Artifacts = append(r.current.Artifacts, Artifact{
			ReleaseID: r.current.ID, WorkspaceID: workspace.WorkspaceID,
			ExpectedDigest: workspace.ArtifactDigest,
		})
	}
	return r.current, nil
}
func (r *serviceTestReleaseRepository) Get(context.Context, string, string) (Release, error) {
	return r.current, nil
}
func (r *serviceTestReleaseRepository) List(context.Context, string) ([]Release, error) {
	return nil, nil
}
func (r *serviceTestReleaseRepository) AssignArtifactTarget(_ context.Context, _, _, workspaceID, servingStateID string) error {
	for index := range r.current.Artifacts {
		if r.current.Artifacts[index].WorkspaceID == workspaceID {
			r.current.Artifacts[index].ServingStateID = servingStateID
			r.current.Manifest.Workspaces[index].ServingStateID = servingStateID
		}
	}
	return nil
}
func (r *serviceTestReleaseRepository) RecordArtifact(_ context.Context, artifact Artifact) error {
	r.recorded = true
	for index := range r.current.Artifacts {
		if r.current.Artifacts[index].WorkspaceID == artifact.WorkspaceID {
			r.current.Artifacts[index].UploadedAt = "2026-07-30T00:00:00Z"
		}
	}
	return nil
}
func (r *serviceTestReleaseRepository) BeginFinalization(context.Context, string, string, jobs.WorkflowIntent) (Release, error) {
	if r.current.Status == StatusDraft {
		r.current.Status = StatusValidating
	}
	return r.current, nil
}
func (r *serviceTestReleaseRepository) CompleteFinalization(context.Context, string, string, map[string]string) (Release, error) {
	r.completed = true
	r.current.Status = StatusReady
	return r.current, nil
}
func (r *serviceTestReleaseRepository) FailFinalization(_ context.Context, _, _ string, cause error) (Release, error) {
	r.current.Status = StatusFailed
	r.current.Error = cause.Error()
	return r.current, nil
}

type serviceTestArtifactValidator struct{ state servingstate.State }

func (v serviceTestArtifactValidator) Validate(context.Context, servingstate.ID) (servingstate.State, error) {
	return v.state, nil
}

type serviceTestPinValidator struct {
	stateID, projectID string
	expected           map[string]string
	err                error
}

func (v *serviceTestPinValidator) ValidateServingStatePins(_ context.Context, stateID, projectID string, expected map[string]string) error {
	v.stateID, v.projectID = stateID, projectID
	v.expected = make(map[string]string, len(expected))
	for key, value := range expected {
		v.expected[key] = value
	}
	return v.err
}

func (v *serviceTestPinValidator) ResolveCandidatePins(
	context.Context,
	string,
	[]string,
	string,
) (map[string]string, error) {
	return nil, nil
}

type serviceTestArtifactStore struct {
	saved     string
	saveCalls int
}

func (s *serviceTestArtifactStore) SaveUpload(_ context.Context, _ servingstate.ID, source io.Reader) (int64, error) {
	s.saveCalls++
	content, err := io.ReadAll(source)
	s.saved = string(content)
	return int64(len(content)), err
}

func base64Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return base64.StdEncoding.EncodeToString(sum[:])
}

type serviceTestCandidateProvenanceRepository struct {
	provenance Provenance
}

func (repository serviceTestCandidateProvenanceRepository) RetainCandidateProvenance(
	context.Context,
	string,
	Provenance,
) (Provenance, error) {
	return repository.provenance, nil
}

func (repository serviceTestCandidateProvenanceRepository) CandidateProvenance(
	_ context.Context,
	_ string,
	candidateID string,
	candidateRevision int64,
) (Provenance, error) {
	if candidateID != repository.provenance.Candidate.ID ||
		candidateRevision != repository.provenance.Candidate.Revision {
		return Provenance{}, ErrNotFound
	}
	return repository.provenance, nil
}

func candidateServiceTestProvenance(t *testing.T) Provenance {
	t.Helper()
	provenance, err := NewProvenance(ProvenanceInput{
		Artifact: ProjectArtifactProvenance{
			SourceDigest:    "sha256:" + strings.Repeat("1", 64),
			ProjectDigest:   "sha256:" + strings.Repeat("2", 64),
			CompilerVersion: "leapview:test", SchemaVersion: 3,
			Workspaces: []WorkspaceArtifactProvenance{{
				WorkspaceID:    "sales",
				ArtifactDigest: "sha256:" + strings.Repeat("3", 64),
			}},
		},
		Candidate: CandidateProvenance{
			ID: "candidate_1", Revision: 4, OwnerID: "publisher",
		},
		Plan: TargetPlanProvenance{
			TargetID: "lvinst_dev", Environment: "dev",
			BaseGeneration: "deployment_7", RuntimeVersion: "runtime:test",
			PolicyDigest: "sha256:" + strings.Repeat("4", 64),
			Workspaces: []TargetWorkspacePlan{{
				WorkspaceID: "sales", ServingStateID: "state_candidate",
				ArtifactDigest: "sha256:" + strings.Repeat("5", 64),
				DataRevision:   "snapshot:17", DataMode: TargetDataReuseSnapshot,
			}},
		},
	})
	require.NoError(t, err)
	return provenance
}

// Compile-time guards keep the service fakes aligned with the real interfaces.
var _ Repository = (*serviceTestReleaseRepository)(nil)
var _ FinalizationUnitOfWork = (*serviceTestReleaseRepository)(nil)
var _ CandidateProvenanceRepository = serviceTestCandidateProvenanceRepository{}
var _ ArtifactValidator = serviceTestArtifactValidator{}
var _ ArtifactStore = (*serviceTestArtifactStore)(nil)
