package activate

import (
	"context"
	"errors"
	"fmt"

	"github.com/Yacobolo/libredash/internal/deployment"
)

var ErrInvalidStatus = errors.New("deployment cannot be activated")

type Repository interface {
	ByID(ctx context.Context, id deployment.ID) (deployment.Deployment, error)
	Activate(ctx context.Context, workspaceID deployment.WorkspaceID, deploymentID deployment.ID) (deployment.Deployment, error)
}

type RuntimeHost interface {
	PrepareDeployment(ctx context.Context, deploymentID string) (deployment.PreparedRuntime, error)
	CommitPrepared(prepared deployment.PreparedRuntime) error
}

type Service struct {
	repo    Repository
	runtime RuntimeHost
}

func NewService(repo Repository, runtime RuntimeHost) Service {
	return Service{repo: repo, runtime: runtime}
}

func (s Service) Activate(ctx context.Context, deploymentID deployment.ID) (deployment.Deployment, error) {
	current, err := s.repo.ByID(ctx, deploymentID)
	if err != nil {
		return deployment.Deployment{}, err
	}
	if !current.CanActivate() {
		return deployment.Deployment{}, fmt.Errorf("%w: deployment %s has status %q, want validated", ErrInvalidStatus, deploymentID, current.Status)
	}

	var prepared deployment.PreparedRuntime
	if s.runtime != nil {
		prepared, err = s.runtime.PrepareDeployment(ctx, string(deploymentID))
		if err != nil {
			return deployment.Deployment{}, err
		}
	}

	activated, err := s.repo.Activate(ctx, current.WorkspaceID, current.ID)
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
