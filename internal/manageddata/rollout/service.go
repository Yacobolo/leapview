// Package rollout coordinates atomic project-global managed-data cutovers.
package rollout

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Yacobolo/libredash/internal/manageddata"
	"github.com/Yacobolo/libredash/internal/runtimehost"
	servingstate "github.com/Yacobolo/libredash/internal/servingstate"
)

type Repository interface {
	CollectionByProjectConnection(context.Context, string, string) (manageddata.Collection, error)
	RevisionByID(context.Context, string) (manageddata.Revision, error)
	CreateRollout(context.Context, manageddata.CreateRolloutInput) (manageddata.Rollout, error)
	RolloutByID(context.Context, string) (manageddata.Rollout, error)
	ActivateRollout(context.Context, string, manageddata.PointerExpectation) (manageddata.Rollout, error)
	FailRollout(context.Context, string, error) error
	EnvironmentPointer(context.Context, string, manageddata.Environment) (manageddata.EnvironmentPointer, error)
	ListServingStateBindings(context.Context, string) ([]manageddata.ServingStateBinding, error)
}

type ServingStateRepository interface {
	RecordDuckLakeSnapshot(context.Context, servingstate.ID, int64) error
}

type CandidateResolver interface {
	ResolveRolloutCandidate(context.Context, servingstate.ID, string, string) (runtimehost.ManagedDataResolution, error)
}

type Prepared interface {
	Snapshots() []runtimehost.PreparedSnapshot
	Close() error
}

type Runtime interface {
	Prepare(context.Context, []runtimehost.ServingStateCandidate) (Prepared, error)
	Commit(Prepared, func() error) error
}

type registryRuntime struct{ registry *runtimehost.Registry }
type registryPrepared struct{ set *runtimehost.PreparedSet }

func NewRegistryRuntime(registry *runtimehost.Registry) (Runtime, error) {
	if registry == nil {
		return nil, fmt.Errorf("runtime registry is required")
	}
	return registryRuntime{registry: registry}, nil
}

func (r registryRuntime) Prepare(ctx context.Context, candidates []runtimehost.ServingStateCandidate) (Prepared, error) {
	set, err := r.registry.PrepareServingStateCandidates(ctx, candidates)
	if err != nil {
		return nil, err
	}
	return registryPrepared{set: set}, nil
}

func (r registryRuntime) Commit(prepared Prepared, activate func() error) error {
	value, ok := prepared.(registryPrepared)
	if !ok || value.set == nil {
		return fmt.Errorf("prepared runtimes belong to a different coordinator")
	}
	return r.registry.CommitPreparedSet(value.set, activate)
}

func (p registryPrepared) Snapshots() []runtimehost.PreparedSnapshot { return p.set.Snapshots() }
func (p registryPrepared) Close() error                              { return p.set.Close() }

type Service struct {
	repo     Repository
	states   ServingStateRepository
	runtime  Runtime
	resolver CandidateResolver
}

type Scope struct {
	Project    string
	Connection string
	RolloutID  string
}

type CreateRequest struct {
	ID          string
	Project     string
	Connection  string
	Environment manageddata.Environment
	RevisionID  string
	Targets     []manageddata.RolloutTargetInput
	Actor       string
}

func New(repo Repository, states ServingStateRepository, runtime Runtime, resolver CandidateResolver) (*Service, error) {
	if repo == nil || states == nil || runtime == nil || resolver == nil {
		return nil, fmt.Errorf("rollout repository, serving-state repository, runtime, and resolver are required")
	}
	return &Service{repo: repo, states: states, runtime: runtime, resolver: resolver}, nil
}

func (s *Service) Create(ctx context.Context, request CreateRequest) (manageddata.Rollout, error) {
	collection, err := s.collection(ctx, request.Project, request.Connection)
	if err != nil {
		return manageddata.Rollout{}, err
	}
	revision, err := s.repo.RevisionByID(ctx, strings.TrimSpace(request.RevisionID))
	if err != nil {
		return manageddata.Rollout{}, err
	}
	if revision.CollectionID != collection.ID || revision.Status != manageddata.RevisionStatusReady {
		return manageddata.Rollout{}, fmt.Errorf("revision is not a ready revision for the managed connection")
	}
	environment, err := manageddata.NormalizeEnvironment(string(request.Environment))
	if err != nil {
		return manageddata.Rollout{}, err
	}
	if len(request.Targets) == 0 {
		return manageddata.Rollout{}, fmt.Errorf("rollout requires at least one workspace target")
	}
	return s.repo.CreateRollout(ctx, manageddata.CreateRolloutInput{
		ID: strings.TrimSpace(request.ID), CollectionID: collection.ID, Environment: environment,
		RevisionID: revision.ID, Targets: append([]manageddata.RolloutTargetInput(nil), request.Targets...), CreatedBy: strings.TrimSpace(request.Actor),
	})
}

func (s *Service) Get(ctx context.Context, scope Scope) (manageddata.Rollout, error) {
	collection, err := s.collection(ctx, scope.Project, scope.Connection)
	if err != nil {
		return manageddata.Rollout{}, err
	}
	row, err := s.repo.RolloutByID(ctx, strings.TrimSpace(scope.RolloutID))
	if err != nil {
		return manageddata.Rollout{}, err
	}
	if row.CollectionID != collection.ID {
		return manageddata.Rollout{}, manageddata.ErrNotFound
	}
	return row, nil
}

func (s *Service) Activate(ctx context.Context, scope Scope) (manageddata.Rollout, error) {
	row, err := s.Get(ctx, scope)
	if err != nil {
		return manageddata.Rollout{}, err
	}
	if row.Status != manageddata.RolloutStatusPending {
		if row.Status == manageddata.RolloutStatusActive {
			return row, nil
		}
		return manageddata.Rollout{}, fmt.Errorf("%w: rollout is %s", manageddata.ErrConflict, row.Status)
	}
	candidates := make([]runtimehost.ServingStateCandidate, 0, len(row.Targets))
	targets := append([]manageddata.RolloutTarget(nil), row.Targets...)
	sort.Slice(targets, func(i, j int) bool { return targets[i].WorkspaceID < targets[j].WorkspaceID })
	for _, target := range targets {
		resolution, resolveErr := s.resolver.ResolveRolloutCandidate(ctx, servingstate.ID(target.ServingStateID), row.CollectionID, row.RevisionID)
		if resolveErr != nil {
			_ = s.repo.FailRollout(ctx, row.ID, resolveErr)
			return manageddata.Rollout{}, resolveErr
		}
		candidates = append(candidates, runtimehost.ServingStateCandidate{ServingStateID: target.ServingStateID, ManagedData: resolution})
	}
	prepared, err := s.runtime.Prepare(ctx, candidates)
	if err != nil {
		_ = s.repo.FailRollout(ctx, row.ID, err)
		return manageddata.Rollout{}, err
	}
	defer prepared.Close()
	for _, snapshot := range prepared.Snapshots() {
		if snapshot.DuckLakeSnapshotID <= 0 {
			continue
		}
		if err := s.states.RecordDuckLakeSnapshot(ctx, snapshot.ServingStateID, snapshot.DuckLakeSnapshotID); err != nil {
			_ = s.repo.FailRollout(ctx, row.ID, err)
			return manageddata.Rollout{}, err
		}
	}
	expected := manageddata.PointerExpectation{}
	pointer, err := s.repo.EnvironmentPointer(ctx, row.CollectionID, row.Environment)
	if err == nil {
		expected = manageddata.PointerExpectation{RevisionID: pointer.RevisionID, Generation: pointer.Generation}
	} else if !errors.Is(err, manageddata.ErrNotFound) {
		return manageddata.Rollout{}, err
	}
	var activated manageddata.Rollout
	err = s.runtime.Commit(prepared, func() error {
		var activateErr error
		activated, activateErr = s.repo.ActivateRollout(ctx, row.ID, expected)
		return activateErr
	})
	if err != nil {
		return manageddata.Rollout{}, err
	}
	return activated, nil
}

func (s *Service) Rollback(ctx context.Context, scope Scope, _ string) (manageddata.Rollout, error) {
	original, err := s.Get(ctx, scope)
	if err != nil {
		return manageddata.Rollout{}, err
	}
	if original.Status != manageddata.RolloutStatusActive && original.Status != manageddata.RolloutStatusSuperseded {
		return manageddata.Rollout{}, fmt.Errorf("%w: only an active or superseded rollout can be rolled back", manageddata.ErrConflict)
	}
	priorRevision := ""
	targets := make([]manageddata.RolloutTargetInput, 0, len(original.Targets))
	for _, target := range original.Targets {
		if target.PriorServingStateID == "" {
			return manageddata.Rollout{}, fmt.Errorf("%w: rollout has no prior serving state for workspace %s", manageddata.ErrConflict, target.WorkspaceID)
		}
		bindings, listErr := s.repo.ListServingStateBindings(ctx, target.PriorServingStateID)
		if listErr != nil {
			return manageddata.Rollout{}, listErr
		}
		boundRevision := ""
		for _, binding := range bindings {
			if binding.CollectionID == original.CollectionID {
				boundRevision = binding.RevisionID
				break
			}
		}
		if boundRevision == "" || priorRevision != "" && priorRevision != boundRevision {
			return manageddata.Rollout{}, fmt.Errorf("%w: prior serving states do not share one managed-data revision", manageddata.ErrConflict)
		}
		priorRevision = boundRevision
		targets = append(targets, manageddata.RolloutTargetInput{WorkspaceID: target.WorkspaceID, ServingStateID: target.PriorServingStateID})
	}
	collection, err := s.repo.CollectionByProjectConnection(ctx, strings.TrimSpace(scope.Project), strings.TrimSpace(scope.Connection))
	if err != nil {
		return manageddata.Rollout{}, err
	}
	created, err := s.repo.CreateRollout(ctx, manageddata.CreateRolloutInput{
		CollectionID: collection.ID, Environment: original.Environment, RevisionID: priorRevision, Targets: targets,
	})
	if err != nil {
		return manageddata.Rollout{}, err
	}
	return s.Activate(ctx, Scope{Project: scope.Project, Connection: scope.Connection, RolloutID: created.ID})
}

func (s *Service) collection(ctx context.Context, project, connection string) (manageddata.Collection, error) {
	project = strings.TrimSpace(project)
	connection = strings.TrimSpace(connection)
	if project == "" || connection == "" {
		return manageddata.Collection{}, fmt.Errorf("project and connection are required")
	}
	collection, err := s.repo.CollectionByProjectConnection(ctx, project, connection)
	if err != nil {
		return manageddata.Collection{}, err
	}
	if collection.ProjectID != project || collection.ConnectionName != connection || collection.Status != manageddata.CollectionStatusActive {
		return manageddata.Collection{}, manageddata.ErrNotFound
	}
	return collection, nil
}
