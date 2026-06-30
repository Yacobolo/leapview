package activate

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/deployment"
	deploymentfs "github.com/Yacobolo/libredash/internal/deployment/filesystem"
)

var ErrInvalidStatus = errors.New("deployment cannot be activated")

type Repository interface {
	ByID(ctx context.Context, id deployment.ID) (deployment.Deployment, error)
	Activate(ctx context.Context, workspaceID deployment.WorkspaceID, deploymentID deployment.ID) (deployment.Deployment, error)
}

type ArtifactRepository interface {
	ArtifactByDeployment(ctx context.Context, deploymentID deployment.ID) (deployment.Artifact, error)
}

type RuntimeHost interface {
	PrepareDeployment(ctx context.Context, deploymentID string) (deployment.PreparedRuntime, error)
	CommitPrepared(prepared deployment.PreparedRuntime) error
}

type Service struct {
	repo      Repository
	runtime   RuntimeHost
	artifacts ArtifactRepository
	access    access.WorkspacePolicyReconciler
}

func NewService(repo Repository, runtime RuntimeHost) Service {
	return Service{repo: repo, runtime: runtime}
}

func NewServiceWithAccess(repo Repository, runtime RuntimeHost, artifacts ArtifactRepository, accessReconciler access.WorkspacePolicyReconciler) Service {
	return Service{repo: repo, runtime: runtime, artifacts: artifacts, access: accessReconciler}
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
	if s.access != nil && s.artifacts != nil {
		if err := s.reconcileAccess(ctx, current); err != nil {
			if prepared != nil {
				_ = prepared.Close()
			}
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

func (s Service) reconcileAccess(ctx context.Context, current deployment.Deployment) error {
	artifact, err := s.artifacts.ArtifactByDeployment(ctx, current.ID)
	if err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "libredash-activate-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	if err := deploymentfs.ExtractArtifact(artifact.Path, root); err != nil {
		return err
	}
	compiled, _, err := deploymentfs.LoadCompiledWorkspaceArtifact(root)
	if err != nil {
		return err
	}
	if compiled.WorkspaceID != string(current.WorkspaceID) {
		return fmt.Errorf("compiled artifact workspace = %q, want %q", compiled.WorkspaceID, current.WorkspaceID)
	}
	if compiled.DeploymentID != string(current.ID) {
		return fmt.Errorf("compiled artifact deployment = %q, want %q", compiled.DeploymentID, current.ID)
	}
	if compiled.Validation.Status != "passed" {
		return fmt.Errorf("compiled artifact validation status = %q, want passed", compiled.Validation.Status)
	}
	return s.access.ReconcileWorkspacePolicy(ctx, string(current.WorkspaceID), compiled.Definition.Access)
}
