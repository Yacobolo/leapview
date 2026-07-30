package module

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/platform"
	"github.com/flidai/leapview/internal/project"
	projectcompiler "github.com/flidai/leapview/internal/project/compiler"
	projectdevloop "github.com/flidai/leapview/internal/project/devloop"
	"github.com/flidai/leapview/internal/release"
	"github.com/flidai/leapview/internal/servingstate"
	"github.com/flidai/leapview/internal/workspace"
)

func TestCandidateArtifactsRefreshThenReuseTargetSnapshot(t *testing.T) {
	projectPath := targetBoundCandidateProject(t)
	snapshot, err := (projectdevloop.FilesystemBuilder{
		ProjectPath: projectPath,
	}).Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	states := newCandidateArtifactStateRepository()
	workspaces := &candidateArtifactWorkspaceRepository{}
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "leapview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	module, err := Build(t.Context(), Config{
		Database:          store.SQLDB(),
		States:            states,
		Workspaces:        workspaces,
		ArtifactDirectory: t.TempDir(),
		Environment:       servingstate.DefaultEnvironment,
	})
	if err != nil {
		t.Fatal(err)
	}
	source := releaseCandidateSource(t, snapshot, projectPath)
	if err := os.RemoveAll(filepath.Dir(projectPath)); err != nil {
		t.Fatal(err)
	}
	first, err := module.PrepareCandidateArtifacts(t.Context(), release.CandidateArtifactRequest{
		CandidateID: "candidate_1", ProjectID: snapshot.ProjectID,
		OwnerID: "principal_1", Environment: "dev", ArtifactDigest: snapshot.Digest,
		Source: source,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.AuthorizationFingerprint == "" || len(first.Workspaces) == 0 {
		t.Fatalf("first artifact set = %#v", first)
	}
	if first.Artifact.SourceDigest != snapshot.Digest ||
		first.Artifact.ProjectDigest != source.ProjectDigest ||
		first.Artifact.CompilerVersion == "" ||
		first.Artifact.SchemaVersion < 1 ||
		len(first.Artifact.Workspaces) != len(first.Workspaces) {
		t.Fatalf("immutable project artifact provenance = %#v", first.Artifact)
	}
	for _, prepared := range first.Workspaces {
		if prepared.DataMode != "refresh_sources" || len(prepared.Connections) == 0 {
			t.Fatalf("initial workspace must refresh through target bindings: %#v", prepared)
		}
		state := states.states[servingstate.ID(prepared.ServingStateID)]
		state.DuckLakeSnapshotID = 42
		state.Status = servingstate.StatusActive
		states.states[state.ID] = state
		states.active[activeCandidateArtifactKey{
			workspace: state.WorkspaceID, environment: state.Environment,
		}] = state.ID
	}

	second, err := module.PrepareCandidateArtifacts(t.Context(), release.CandidateArtifactRequest{
		CandidateID: "candidate_2", ProjectID: snapshot.ProjectID,
		OwnerID: "principal_1", Environment: "dev", ArtifactDigest: snapshot.Digest,
		Source: source,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.AuthorizationFingerprint == "" || len(second.Workspaces) != len(first.Workspaces) {
		t.Fatalf("second artifact set = %#v", second)
	}
	if !reflect.DeepEqual(second.Artifact, first.Artifact) {
		t.Fatalf(
			"target snapshot reuse changed project artifact provenance: %#v / %#v",
			first.Artifact,
			second.Artifact,
		)
	}
	for _, prepared := range second.Workspaces {
		if prepared.DataMode != "reuse_snapshot" ||
			prepared.DataRevision != "snapshot:42" ||
			len(prepared.Connections) != 0 {
			t.Fatalf("unchanged workspace did not reuse target snapshot: %#v", prepared)
		}
		if state := states.states[servingstate.ID(prepared.ServingStateID)]; state.DuckLakeSnapshotID != 42 {
			t.Fatalf("candidate serving state snapshot = %d, want 42", state.DuckLakeSnapshotID)
		}
	}
	if len(workspaces.ensured) != len(first.Workspaces)+len(second.Workspaces) {
		t.Fatalf("ensured workspaces = %v", workspaces.ensured)
	}
}

func TestCandidateRestrictionsSelectOnlyOwnerAndUniversalPolicies(t *testing.T) {
	policy := `{
		"groups":{"authors":{"id":"authors","name":"Authors","members":[{"principalId":"author_1"}]}},
		"dataPolicies":{
			"all":{"id":"all","object":{"type":"workspace"},"policyType":"row_filter","expressionJson":"{\"field\":\"orders.region\",\"operator\":\"equals\",\"values\":[\"EU\"]}"},
			"owner":{"id":"owner","object":{"type":"semantic_model","id":"sales"},"subject":{"kind":"principal","principalId":"author_1"},"policyType":"column_mask","expressionJson":"{\"field\":\"orders.email\",\"strategy\":\"redact\"}"},
			"group":{"id":"group","object":{"type":"table","id":"orders"},"subject":{"kind":"group","group":"authors"},"policyType":"row_filter","expressionJson":"{\"field\":\"orders.team\",\"operator\":\"equals\",\"values\":[\"A\"]}"},
			"foreign":{"id":"foreign","object":{"type":"workspace"},"subject":{"kind":"principal","principalId":"author_2"},"policyType":"row_filter","expressionJson":"{}"}
		}
	}`
	restrictions, err := candidateRestrictions(policy, "sales", "author_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(restrictions) != 3 {
		t.Fatalf("candidate restrictions = %#v", restrictions)
	}
	if restrictions[0].ObjectID != "workspace:sales" ||
		restrictions[1].ObjectID != "table:sales:orders" ||
		restrictions[2].ObjectID != "semantic_model:sales:sales" {
		t.Fatalf("candidate restriction scopes = %#v", restrictions)
	}
}

func targetBoundCandidateProject(t *testing.T) string {
	t.Helper()
	sourceProject := filepath.Join("..", "..", "..", "dashboards", "leapview.yaml")
	sourceProject, err := filepath.Abs(sourceProject)
	if err != nil {
		t.Fatal(err)
	}
	sourceRoot := filepath.Dir(sourceProject)
	destinationRoot := t.TempDir()
	files, err := projectcompiler.SourceFiles(sourceProject)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range files {
		relative, err := filepath.Rel(sourceRoot, source)
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.ToSlash(relative) == "connections/olist.yaml" {
			content = []byte(strings.Replace(
				string(content),
				"kind: managed",
				"kind: postgres",
				1,
			))
		}
		if strings.HasPrefix(filepath.ToSlash(relative), "sources/") {
			content = []byte(strings.Replace(
				string(content),
				"  path:",
				"  object:",
				1,
			))
		}
		target := filepath.Join(destinationRoot, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(destinationRoot, "leapview.yaml")
}

func releaseCandidateSource(
	t *testing.T,
	snapshot projectdevloop.Snapshot,
	projectPath string,
) project.CandidateSourceSnapshot {
	t.Helper()
	compiled, err := projectcompiler.CompileProjectArtifact(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(t.TempDir(), "project.artifact.json")
	if err := os.WriteFile(artifactPath, compiled.Canonical(), 0o600); err != nil {
		t.Fatal(err)
	}
	return project.CandidateSourceSnapshot{
		ProjectID: snapshot.ProjectID, ArtifactDigest: snapshot.Digest,
		ProjectDigest: compiled.Digest(), ProjectArtifactPath: artifactPath,
	}
}

type activeCandidateArtifactKey struct {
	workspace   servingstate.WorkspaceID
	environment servingstate.Environment
}

type candidateArtifactStateRepository struct {
	next      int
	states    map[servingstate.ID]servingstate.State
	artifacts map[servingstate.ID]servingstate.Artifact
	active    map[activeCandidateArtifactKey]servingstate.ID
}

func newCandidateArtifactStateRepository() *candidateArtifactStateRepository {
	return &candidateArtifactStateRepository{
		states:    make(map[servingstate.ID]servingstate.State),
		artifacts: make(map[servingstate.ID]servingstate.Artifact),
		active:    make(map[activeCandidateArtifactKey]servingstate.ID),
	}
}

func (repository *candidateArtifactStateRepository) Create(
	_ context.Context,
	input servingstate.CreateInput,
) (servingstate.State, error) {
	repository.next++
	state := servingstate.State{
		ID:          servingstate.ID("candidate_state_" + strconv.Itoa(repository.next)),
		WorkspaceID: input.WorkspaceID, ProjectID: input.ProjectID,
		Environment: input.Environment, Source: input.Source,
		Status: servingstate.StatusPending, CreatedBy: input.CreatedBy,
	}
	repository.states[state.ID] = state
	return state, nil
}

func (repository *candidateArtifactStateRepository) ByID(
	_ context.Context,
	id servingstate.ID,
) (servingstate.State, error) {
	state, ok := repository.states[id]
	if !ok {
		return servingstate.State{}, servingstate.ErrNotFound
	}
	return state, nil
}

func (repository *candidateArtifactStateRepository) MarkFailed(
	_ context.Context,
	id servingstate.ID,
	cause error,
) error {
	state := repository.states[id]
	state.Status = servingstate.StatusFailed
	state.Error = cause.Error()
	repository.states[id] = state
	return nil
}

func (repository *candidateArtifactStateRepository) SaveValidated(
	_ context.Context,
	id servingstate.ID,
	validation servingstate.Validation,
	artifact servingstate.Artifact,
) (servingstate.State, error) {
	state := repository.states[id]
	state.Status = servingstate.StatusValidated
	state.ProjectDigest = validation.ProjectDigest
	state.ProjectWorkspaces = append([]string(nil), validation.ProjectWorkspaces...)
	state.AccessPolicyJSON = "{}"
	state.Digest = validation.Digest
	repository.states[id] = state
	repository.artifacts[id] = artifact
	return state, nil
}

func (repository *candidateArtifactStateRepository) ActiveArtifact(
	_ context.Context,
	workspaceID servingstate.WorkspaceID,
	environment servingstate.Environment,
) (servingstate.State, servingstate.Artifact, error) {
	id, ok := repository.active[activeCandidateArtifactKey{
		workspace: workspaceID, environment: environment,
	}]
	if !ok {
		return servingstate.State{}, servingstate.Artifact{}, servingstate.ErrNotFound
	}
	return repository.states[id], repository.artifacts[id], nil
}

func (repository *candidateArtifactStateRepository) RecordDuckLakeSnapshot(
	_ context.Context,
	id servingstate.ID,
	snapshotID int64,
) error {
	state := repository.states[id]
	state.DuckLakeSnapshotID = snapshotID
	repository.states[id] = state
	return nil
}

type candidateArtifactWorkspaceRepository struct {
	ensured []workspace.WorkspaceID
}

func (repository *candidateArtifactWorkspaceRepository) Ensure(
	_ context.Context,
	input workspace.EnsureInput,
) error {
	repository.ensured = append(repository.ensured, input.ID)
	return nil
}
