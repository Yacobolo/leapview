package bundle

import (
	"context"
	"os"

	projectartifact "github.com/Yacobolo/leapview/internal/project/artifact"
	refreshrun "github.com/Yacobolo/leapview/internal/refresh/run"
	"github.com/Yacobolo/leapview/internal/servingstate"
)

// RefreshArtifactLoader decodes the refresh projection from a persisted
// project bundle without exposing the bundle layout to the refresh capability.
type RefreshArtifactLoader struct{}

func (RefreshArtifactLoader) Load(_ context.Context, artifact servingstate.Artifact) (refreshrun.LoadedArtifact, error) {
	root, err := os.MkdirTemp("", "leapview-refresh-artifact-*")
	if err != nil {
		return refreshrun.LoadedArtifact{}, err
	}
	defer os.RemoveAll(root)
	if err := ExtractArtifact(artifact.Path, root); err != nil {
		return refreshrun.LoadedArtifact{}, err
	}
	compiled, _, err := LoadCompiledWorkspaceArtifact(root)
	if err != nil {
		return refreshrun.LoadedArtifact{}, err
	}
	return refreshrun.LoadedArtifact{
		Definition:           projectartifact.RefreshProjection(compiled.Manifest),
		Graph:                compiled.Graph,
		ManagedDataRevisions: compiled.ManagedDataRevisions,
	}, nil
}
