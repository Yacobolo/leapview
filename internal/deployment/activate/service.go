package activate

import (
	"context"
	"errors"
	"fmt"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/deployment"
	"github.com/Yacobolo/libredash/internal/workspace"
)

var ErrInvalidStatus = errors.New("deployment cannot be activated")

type Repository interface {
	ByID(ctx context.Context, id deployment.ID) (deployment.Deployment, error)
	RecordDuckLakeSnapshot(ctx context.Context, deploymentID deployment.ID, snapshotID int64) error
	Activate(ctx context.Context, workspaceID deployment.WorkspaceID, environment deployment.Environment, deploymentID deployment.ID) (deployment.Deployment, error)
	ActivateWithWorkspacePolicy(ctx context.Context, workspaceID deployment.WorkspaceID, environment deployment.Environment, deploymentID deployment.ID, policy workspace.AccessPolicy) (deployment.Deployment, error)
}

type AccessPolicyLoader interface {
	LoadAccessPolicy(ctx context.Context, deployment deployment.Deployment) (workspace.AccessPolicy, error)
}

type RuntimeHost interface {
	PrepareDeployment(ctx context.Context, deploymentID string) (deployment.PreparedRuntime, error)
	CommitPrepared(prepared deployment.PreparedRuntime) error
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

func (s Service) Activate(ctx context.Context, deploymentID deployment.ID) (deployment.Deployment, error) {
	current, err := s.repo.ByID(ctx, deploymentID)
	if err != nil {
		return deployment.Deployment{}, err
	}
	if !current.CanActivate() {
		return deployment.Deployment{}, fmt.Errorf("%w: deployment %s has status %q, want validated", ErrInvalidStatus, deploymentID, current.Status)
	}

	var policy *workspace.AccessPolicy
	if s.access != nil && s.policies != nil {
		loaded, err := s.policies.LoadAccessPolicy(ctx, current)
		if err != nil {
			return deployment.Deployment{}, err
		}
		policy = &loaded
	}
	var prepared deployment.PreparedRuntime
	if s.runtime != nil {
		prepared, err = s.runtime.PrepareDeployment(ctx, string(deploymentID))
		if err != nil {
			return deployment.Deployment{}, err
		}
	}
	if snapshot, ok := prepared.(preparedDuckLakeSnapshot); ok && snapshot.DuckLakeSnapshotID() > 0 {
		if err := s.repo.RecordDuckLakeSnapshot(ctx, current.ID, snapshot.DuckLakeSnapshotID()); err != nil {
			if prepared != nil {
				_ = prepared.Close()
			}
			return deployment.Deployment{}, err
		}
	}

	var activated deployment.Deployment
	if policy != nil {
		activated, err = s.repo.ActivateWithWorkspacePolicy(ctx, current.WorkspaceID, current.Environment, current.ID, *policy)
	} else {
		activated, err = s.repo.Activate(ctx, current.WorkspaceID, current.Environment, current.ID)
	}
	if err != nil {
		if prepared != nil {
			_ = prepared.Close()
		}
		return deployment.Deployment{}, err
	}
	if prepared != nil {
		if err := s.runtime.CommitPrepared(prepared); err != nil {
			_ = prepared.Close()
			return deployment.Deployment{}, err
		}
	}
	return activated, nil
}
