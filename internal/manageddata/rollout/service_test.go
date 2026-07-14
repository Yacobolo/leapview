package rollout

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Yacobolo/libredash/internal/manageddata"
	"github.com/Yacobolo/libredash/internal/runtimehost"
	servingstate "github.com/Yacobolo/libredash/internal/servingstate"
)

func TestActivatePreparesCandidatePersistsSnapshotThenCommits(t *testing.T) {
	repo := &fakeRepository{
		collection: manageddata.Collection{ID: "collection_1", ProjectID: "project", ConnectionName: "warehouse", Status: manageddata.CollectionStatusActive},
		revision:   manageddata.Revision{ID: "revision_2", CollectionID: "collection_1", Status: manageddata.RevisionStatusReady},
		rollout: manageddata.Rollout{ID: "rollout_1", CollectionID: "collection_1", Environment: "prod", RevisionID: "revision_2", Status: manageddata.RolloutStatusPending, Targets: []manageddata.RolloutTarget{
			{RolloutID: "rollout_1", WorkspaceID: "sales", ServingStateID: "state_2", PriorServingStateID: "state_1", Status: manageddata.TargetStatusPending},
		}},
		pointer: manageddata.EnvironmentPointer{CollectionID: "collection_1", Environment: "prod", RevisionID: "revision_1", Generation: 4},
	}
	resolver := &fakeResolver{resolution: runtimehost.ManagedDataResolution{RevisionID: "sha256:candidate", Roots: map[string]string{"warehouse": "/cache/revision"}}}
	runtime := &fakeRuntime{prepared: &fakePrepared{snapshots: []runtimehost.PreparedSnapshot{{WorkspaceID: "sales", ServingStateID: "state_2", DuckLakeSnapshotID: 42}}}}
	states := &fakeServingStates{}
	service, err := New(repo, states, runtime, resolver)
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Activate(context.Background(), Scope{Project: "project", Connection: "warehouse", RolloutID: "rollout_1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != manageddata.RolloutStatusActive {
		t.Fatalf("status = %q", got.Status)
	}
	wantCandidate := []runtimehost.ServingStateCandidate{{ServingStateID: "state_2", ManagedData: resolver.resolution}}
	if !reflect.DeepEqual(runtime.candidates, wantCandidate) {
		t.Fatalf("candidates = %#v, want %#v", runtime.candidates, wantCandidate)
	}
	if !reflect.DeepEqual(states.recorded, map[servingstate.ID]int64{"state_2": 42}) {
		t.Fatalf("recorded snapshots = %#v", states.recorded)
	}
	if repo.activatedExpectation != (manageddata.PointerExpectation{RevisionID: "revision_1", Generation: 4}) {
		t.Fatalf("activation expectation = %#v", repo.activatedExpectation)
	}
	if !runtime.committed {
		t.Fatal("prepared runtimes were not committed")
	}
}

func TestActivatePreparationFailureLeavesMetadataUntouched(t *testing.T) {
	repo := validFakeRepository()
	prepareErr := errors.New("duckdb preparation failed")
	runtime := &fakeRuntime{prepareErr: prepareErr}
	service, err := New(repo, &fakeServingStates{}, runtime, &fakeResolver{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Activate(context.Background(), Scope{Project: "project", Connection: "warehouse", RolloutID: "rollout_1"})
	if !errors.Is(err, prepareErr) {
		t.Fatalf("error = %v, want %v", err, prepareErr)
	}
	if repo.activateCalls != 0 || runtime.committed {
		t.Fatalf("activate calls=%d committed=%v", repo.activateCalls, runtime.committed)
	}
	if repo.failedRollout != "rollout_1" {
		t.Fatalf("failed rollout = %q", repo.failedRollout)
	}
}

func TestRollbackUsesPriorServingStatesAndBoundRevision(t *testing.T) {
	repo := validFakeRepository()
	repo.rollout.Status = manageddata.RolloutStatusActive
	repo.bindings = map[string][]manageddata.ServingStateBinding{
		"state_1": {{ServingStateID: "state_1", CollectionID: "collection_1", RevisionID: "revision_1", Environment: "prod"}},
	}
	repo.revisions = map[string]manageddata.Revision{
		"revision_1": {ID: "revision_1", CollectionID: "collection_1", Status: manageddata.RevisionStatusReady},
		"revision_2": repo.revision,
	}
	runtime := &fakeRuntime{prepared: &fakePrepared{}}
	service, err := New(repo, &fakeServingStates{}, runtime, &fakeResolver{})
	if err != nil {
		t.Fatal(err)
	}

	rolledBack, err := service.Rollback(context.Background(), Scope{Project: "project", Connection: "warehouse", RolloutID: "rollout_1"}, "operator rollback")
	if err != nil {
		t.Fatal(err)
	}
	if repo.created.RevisionID != "revision_1" || len(repo.created.Targets) != 1 || repo.created.Targets[0].ServingStateID != "state_1" {
		t.Fatalf("rollback create input = %#v", repo.created)
	}
	if rolledBack.Status != manageddata.RolloutStatusActive {
		t.Fatalf("rollback status = %q", rolledBack.Status)
	}
}

func validFakeRepository() *fakeRepository {
	return &fakeRepository{
		collection: manageddata.Collection{ID: "collection_1", ProjectID: "project", ConnectionName: "warehouse", Status: manageddata.CollectionStatusActive},
		revision:   manageddata.Revision{ID: "revision_2", CollectionID: "collection_1", Status: manageddata.RevisionStatusReady},
		rollout: manageddata.Rollout{ID: "rollout_1", CollectionID: "collection_1", Environment: "prod", RevisionID: "revision_2", Status: manageddata.RolloutStatusPending, Targets: []manageddata.RolloutTarget{
			{RolloutID: "rollout_1", WorkspaceID: "sales", ServingStateID: "state_2", PriorServingStateID: "state_1", Status: manageddata.TargetStatusPending},
		}},
		pointer: manageddata.EnvironmentPointer{CollectionID: "collection_1", Environment: "prod", RevisionID: "revision_1", Generation: 1},
	}
}

type fakeRepository struct {
	collection           manageddata.Collection
	revision             manageddata.Revision
	revisions            map[string]manageddata.Revision
	rollout              manageddata.Rollout
	pointer              manageddata.EnvironmentPointer
	bindings             map[string][]manageddata.ServingStateBinding
	created              manageddata.CreateRolloutInput
	activateCalls        int
	activatedExpectation manageddata.PointerExpectation
	failedRollout        string
}

func (r *fakeRepository) CollectionByProjectConnection(context.Context, string, string) (manageddata.Collection, error) {
	return r.collection, nil
}
func (r *fakeRepository) RevisionByID(_ context.Context, id string) (manageddata.Revision, error) {
	if r.revisions != nil {
		if revision, ok := r.revisions[id]; ok {
			return revision, nil
		}
	}
	return r.revision, nil
}
func (r *fakeRepository) CreateRollout(_ context.Context, input manageddata.CreateRolloutInput) (manageddata.Rollout, error) {
	r.created = input
	r.rollout = manageddata.Rollout{ID: "rollout_rollback", CollectionID: input.CollectionID, Environment: input.Environment, RevisionID: input.RevisionID, Status: manageddata.RolloutStatusPending}
	for _, target := range input.Targets {
		r.rollout.Targets = append(r.rollout.Targets, manageddata.RolloutTarget{RolloutID: r.rollout.ID, WorkspaceID: target.WorkspaceID, ServingStateID: target.ServingStateID, Status: manageddata.TargetStatusPending})
	}
	return r.rollout, nil
}
func (r *fakeRepository) RolloutByID(context.Context, string) (manageddata.Rollout, error) {
	return r.rollout, nil
}
func (r *fakeRepository) ActivateRollout(_ context.Context, _ string, expected manageddata.PointerExpectation) (manageddata.Rollout, error) {
	r.activateCalls++
	r.activatedExpectation = expected
	r.rollout.Status = manageddata.RolloutStatusActive
	return r.rollout, nil
}
func (r *fakeRepository) FailRollout(_ context.Context, id string, _ error) error {
	r.failedRollout = id
	return nil
}
func (r *fakeRepository) EnvironmentPointer(context.Context, string, manageddata.Environment) (manageddata.EnvironmentPointer, error) {
	return r.pointer, nil
}
func (r *fakeRepository) ListServingStateBindings(_ context.Context, id string) ([]manageddata.ServingStateBinding, error) {
	return append([]manageddata.ServingStateBinding(nil), r.bindings[id]...), nil
}

type fakeResolver struct {
	resolution runtimehost.ManagedDataResolution
	err        error
}

func (r *fakeResolver) ResolveRolloutCandidate(context.Context, servingstate.ID, string, string) (runtimehost.ManagedDataResolution, error) {
	return r.resolution, r.err
}

type fakePrepared struct {
	snapshots []runtimehost.PreparedSnapshot
	closed    bool
}

func (p *fakePrepared) Snapshots() []runtimehost.PreparedSnapshot {
	return append([]runtimehost.PreparedSnapshot(nil), p.snapshots...)
}
func (p *fakePrepared) Close() error { p.closed = true; return nil }

type fakeRuntime struct {
	prepared   *fakePrepared
	prepareErr error
	candidates []runtimehost.ServingStateCandidate
	committed  bool
}

func (r *fakeRuntime) Prepare(_ context.Context, candidates []runtimehost.ServingStateCandidate) (Prepared, error) {
	r.candidates = append([]runtimehost.ServingStateCandidate(nil), candidates...)
	return r.prepared, r.prepareErr
}
func (r *fakeRuntime) Commit(prepared Prepared, activate func() error) error {
	if err := activate(); err != nil {
		return err
	}
	r.committed = true
	return nil
}

type fakeServingStates struct{ recorded map[servingstate.ID]int64 }

func (s *fakeServingStates) RecordDuckLakeSnapshot(_ context.Context, id servingstate.ID, snapshot int64) error {
	if s.recorded == nil {
		s.recorded = map[servingstate.ID]int64{}
	}
	s.recorded[id] = snapshot
	return nil
}
