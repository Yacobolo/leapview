package activate

import (
	"context"
	"errors"
	"fmt"

	"github.com/Yacobolo/libredash/internal/access"
	servingstate "github.com/Yacobolo/libredash/internal/servingstate"
	"github.com/Yacobolo/libredash/internal/workspace"
)

var ErrInvalidStatus = errors.New("serving state cannot be activated")

type Repository interface {
	ByID(ctx context.Context, id servingstate.ID) (servingstate.State, error)
	RecordDuckLakeSnapshot(ctx context.Context, servingStateID servingstate.ID, snapshotID int64) error
	Activate(ctx context.Context, workspaceID servingstate.WorkspaceID, environment servingstate.Environment, servingStateID servingstate.ID) (servingstate.State, error)
	ActivateWithWorkspacePolicy(ctx context.Context, workspaceID servingstate.WorkspaceID, environment servingstate.Environment, servingStateID servingstate.ID, policy workspace.AccessPolicy) (servingstate.State, error)
}

type AccessPolicyLoader interface {
	LoadAccessPolicy(ctx context.Context, state servingstate.State) (workspace.AccessPolicy, error)
}

type RuntimeHost interface {
	PrepareServingState(ctx context.Context, servingStateID string) (servingstate.PreparedRuntime, error)
	CommitPrepared(prepared servingstate.PreparedRuntime) error
}

type preparedDuckLakeSnapshot interface {
	DuckLakeSnapshotID() int64
}

type Service struct {
	repo     Repository
	runtime  RuntimeHost
	policies AccessPolicyLoader
	access   access.WorkspacePolicyReconciler
}

func NewService(repo Repository, runtime RuntimeHost) Service {
	return Service{repo: repo, runtime: runtime}
}

func NewServiceWithAccess(repo Repository, runtime RuntimeHost, policies AccessPolicyLoader, accessReconciler access.WorkspacePolicyReconciler) Service {
	return Service{repo: repo, runtime: runtime, policies: policies, access: accessReconciler}
}

func (s Service) Activate(ctx context.Context, servingStateID servingstate.ID) (servingstate.State, error) {
	current, err := s.repo.ByID(ctx, servingStateID)
	if err != nil {
		return servingstate.State{}, err
	}
	if !current.CanActivate() {
		return servingstate.State{}, fmt.Errorf("%w: serving state %s has status %q, want validated", ErrInvalidStatus, servingStateID, current.Status)
	}

	var policy *workspace.AccessPolicy
	if s.access != nil && s.policies != nil {
		loaded, err := s.policies.LoadAccessPolicy(ctx, current)
		if err != nil {
			return servingstate.State{}, err
		}
		policy = &loaded
	}
	var prepared servingstate.PreparedRuntime
	if s.runtime != nil {
		prepared, err = s.runtime.PrepareServingState(ctx, string(servingStateID))
		if err != nil {
			return servingstate.State{}, err
		}
	}
	if snapshot, ok := prepared.(preparedDuckLakeSnapshot); ok && snapshot.DuckLakeSnapshotID() > 0 {
		if err := s.repo.RecordDuckLakeSnapshot(ctx, current.ID, snapshot.DuckLakeSnapshotID()); err != nil {
			if prepared != nil {
				_ = prepared.Close()
			}
			return servingstate.State{}, err
		}
	}

	var activated servingstate.State
	if policy != nil {
		activated, err = s.repo.ActivateWithWorkspacePolicy(ctx, current.WorkspaceID, current.Environment, current.ID, *policy)
	} else {
		activated, err = s.repo.Activate(ctx, current.WorkspaceID, current.Environment, current.ID)
	}
	if err != nil {
		if prepared != nil {
			_ = prepared.Close()
		}
		return servingstate.State{}, err
	}
	if prepared != nil {
		if err := s.runtime.CommitPrepared(prepared); err != nil {
			_ = prepared.Close()
			return servingstate.State{}, err
		}
	}
	return activated, nil
}
