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
	"testing"

	"github.com/flidai/leapview/internal/platform/jobs"
	"github.com/flidai/leapview/internal/servingstate"
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

type serviceTestReleaseRepository struct {
	current   Release
	completed bool
	recorded  bool
}

func (r *serviceTestReleaseRepository) Create(context.Context, CreateInput) (Release, error) {
	return Release{}, nil
}
func (r *serviceTestReleaseRepository) Get(context.Context, string, string) (Release, error) {
	return r.current, nil
}
func (r *serviceTestReleaseRepository) List(context.Context, string) ([]Release, error) {
	return nil, nil
}
func (r *serviceTestReleaseRepository) AssignArtifactTarget(context.Context, string, string, string, string) error {
	return nil
}
func (r *serviceTestReleaseRepository) RecordArtifact(context.Context, Artifact) error {
	r.recorded = true
	return nil
}
func (r *serviceTestReleaseRepository) BeginFinalization(context.Context, string, string, jobs.WorkflowIntent) (Release, error) {
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

// Compile-time guards keep the service fakes aligned with the real interfaces.
var _ Repository = (*serviceTestReleaseRepository)(nil)
var _ FinalizationUnitOfWork = (*serviceTestReleaseRepository)(nil)
var _ ArtifactValidator = serviceTestArtifactValidator{}
var _ ArtifactStore = (*serviceTestArtifactStore)(nil)
