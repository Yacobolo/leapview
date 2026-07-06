package filesystem

import (
	"context"
	"fmt"
	"os"

	servingstate "github.com/Yacobolo/libredash/internal/servingstate"
	"github.com/Yacobolo/libredash/internal/workspace"
)

type ArtifactRepository interface {
	ArtifactByServingState(ctx context.Context, servingStateID servingstate.ID) (servingstate.Artifact, error)
}

type AccessPolicyLoader struct {
	artifacts ArtifactRepository
}

func NewAccessPolicyLoader(artifacts ArtifactRepository) AccessPolicyLoader {
	return AccessPolicyLoader{artifacts: artifacts}
}

func (l AccessPolicyLoader) LoadAccessPolicy(ctx context.Context, current servingstate.State) (workspace.AccessPolicy, error) {
	if l.artifacts == nil {
		return workspace.AccessPolicy{}, fmt.Errorf("serving state artifact repository is required")
	}
	artifact, err := l.artifacts.ArtifactByServingState(ctx, current.ID)
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
	if compiled.ServingStateID != string(current.ID) {
		return workspace.AccessPolicy{}, fmt.Errorf("compiled artifact serving state = %q, want %q", compiled.ServingStateID, current.ID)
	}
	if servingstate.Environment(compiled.Environment) != servingstate.NormalizeEnvironment(current.Environment) {
		return workspace.AccessPolicy{}, fmt.Errorf("compiled artifact environment = %q, want %q", compiled.Environment, servingstate.NormalizeEnvironment(current.Environment))
	}
	if err := ValidateCompiledWorkspaceArtifact(compiled); err != nil {
		return workspace.AccessPolicy{}, err
	}
	return compiled.Definition.Access, nil
}
