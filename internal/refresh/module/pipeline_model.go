package module

import (
	"context"

	projectbundle "github.com/Yacobolo/leapview/internal/project/bundle"
	refreshrun "github.com/Yacobolo/leapview/internal/refresh/run"
	"github.com/Yacobolo/leapview/internal/servingstate"
)

type ActiveArtifactReader interface {
	ActiveArtifact(context.Context, servingstate.WorkspaceID, servingstate.Environment) (servingstate.State, servingstate.Artifact, error)
}

func PipelineModelResolver(states ActiveArtifactReader, artifacts refreshrun.ArtifactLoader, environment servingstate.Environment) func(context.Context, string, string) (string, bool, error) {
	if artifacts == nil {
		artifacts = projectbundle.RefreshArtifactLoader{}
	}
	return func(ctx context.Context, workspaceID, pipelineID string) (string, bool, error) {
		if states == nil || artifacts == nil {
			return "", false, nil
		}
		_, artifact, err := states.ActiveArtifact(ctx, servingstate.WorkspaceID(workspaceID), servingstate.NormalizeEnvironment(environment))
		if err != nil {
			return "", false, err
		}
		loaded, err := artifacts.Load(ctx, artifact)
		if err != nil {
			return "", false, err
		}
		if loaded.Definition == nil {
			return "", false, nil
		}
		pipeline, ok := loaded.Definition.Pipelines[pipelineID]
		if !ok {
			return "", false, nil
		}
		return pipeline.SemanticModel, true, nil
	}
}
