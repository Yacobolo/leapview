package filesystem

import (
	"context"
	"fmt"
	"os"

	"github.com/Yacobolo/libredash/internal/deployment"
	"github.com/Yacobolo/libredash/internal/workspace"
)

type ArtifactRepository interface {
	ArtifactByDeployment(ctx context.Context, deploymentID deployment.ID) (deployment.Artifact, error)
}

type AccessPolicyLoader struct {
	artifacts ArtifactRepository
}

func NewAccessPolicyLoader(artifacts ArtifactRepository) AccessPolicyLoader {
	return AccessPolicyLoader{artifacts: artifacts}
}

func (l AccessPolicyLoader) LoadAccessPolicy(ctx context.Context, current deployment.Deployment) (workspace.AccessPolicy, error) {
	if l.artifacts == nil {
		return workspace.AccessPolicy{}, fmt.Errorf("deployment artifact repository is required")
	}
	artifact, err := l.artifacts.ArtifactByDeployment(ctx, current.ID)
	if err != nil {
		return workspace.AccessPolicy{}, err
	}
	root, err := os.MkdirTemp("", "libredash-activate-*")
	if err != nil {
		return workspace.AccessPolicy{}, err
	}
	defer os.RemoveAll(root)
	if err := ExtractArtifact(artifact.Path, root); err != nil {
		return workspace.AccessPolicy{}, err
	}
	compiled, _, err := LoadCompiledWorkspaceArtifact(root)
	if err != nil {
		return workspace.AccessPolicy{}, err
	}
	if compiled.WorkspaceID != string(current.WorkspaceID) {
		return workspace.AccessPolicy{}, fmt.Errorf("compiled artifact workspace = %q, want %q", compiled.WorkspaceID, current.WorkspaceID)
	}
	if compiled.DeploymentID != string(current.ID) {
		return workspace.AccessPolicy{}, fmt.Errorf("compiled artifact deployment = %q, want %q", compiled.DeploymentID, current.ID)
	}
	if deployment.Environment(compiled.Environment) != deployment.NormalizeEnvironment(current.Environment) {
		return workspace.AccessPolicy{}, fmt.Errorf("compiled artifact environment = %q, want %q", compiled.Environment, deployment.NormalizeEnvironment(current.Environment))
	}
	if err := ValidateCompiledWorkspaceArtifact(compiled); err != nil {
		return workspace.AccessPolicy{}, err
	}
	return compiled.Definition.Access, nil
}
