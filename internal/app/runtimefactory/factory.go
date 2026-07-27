package runtimefactory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	dashboardruntimefactory "github.com/Yacobolo/leapview/internal/dashboard/runtimefactory"
	projectartifact "github.com/Yacobolo/leapview/internal/project/artifact"
	projectbundle "github.com/Yacobolo/leapview/internal/project/bundle"
	"github.com/Yacobolo/leapview/internal/runtimehost"
	servingstate "github.com/Yacobolo/leapview/internal/servingstate"
)

type FactoryConfig struct {
	DuckDBDir        string
	RuntimeDir       string
	DashboardRuntime dashboardruntimefactory.Builder
}

type servingStateRuntimeFactory struct {
	duckDBDir        string
	runtimeDir       string
	dashboardRuntime dashboardruntimefactory.Builder
}

func NewFactory(config FactoryConfig) runtimehost.RuntimeFactory {
	return servingStateRuntimeFactory{
		duckDBDir: config.DuckDBDir, runtimeDir: config.RuntimeDir, dashboardRuntime: config.DashboardRuntime,
	}
}

func (f servingStateRuntimeFactory) Prepare(ctx context.Context, input runtimehost.RuntimeInput) (runtimehost.Runtime, error) {
	duckDBDir := runtimeFirstNonEmpty(input.DuckDBDir, f.duckDBDir)
	runtimeDir := runtimeFirstNonEmpty(input.RuntimeDir, f.runtimeDir)
	targetDir := filepath.Join(runtimeDir, string(input.State.ID)+"-"+shortDigest(input.Artifact.Digest))
	if err := os.RemoveAll(targetDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, err
	}
	if err := projectbundle.ExtractArtifact(input.Artifact.Path, targetDir); err != nil {
		return nil, err
	}
	duckDir := filepath.Join(duckDBDir, string(servingstate.NormalizeEnvironment(input.State.Environment)))
	compiled, _, err := projectbundle.LoadCompiledWorkspaceArtifact(targetDir)
	if err != nil {
		return nil, err
	}
	if compiled.WorkspaceID != string(input.State.WorkspaceID) {
		return nil, fmt.Errorf("compiled artifact workspace = %q, want %q", compiled.WorkspaceID, input.State.WorkspaceID)
	}
	if err := bindManagedDataRoots(compiled.Manifest, input.ManagedData.Roots); err != nil {
		return nil, err
	}
	if f.dashboardRuntime == nil {
		return nil, fmt.Errorf("dashboard runtime builder is required")
	}
	runtimeInput := dashboardruntimefactory.Input{
		Directory: duckDir, SnapshotID: input.State.DuckLakeSnapshotID,
		ServingStateID: string(input.State.ID), WorkspaceID: string(input.State.WorkspaceID),
		Environment: string(servingstate.NormalizeEnvironment(input.State.Environment)), SemanticModelDigest: input.State.Digest,
		ArtifactDigest: input.Artifact.Digest, SourceDataDigest: input.ManagedData.RevisionID,
		Definition: projectartifact.DashboardProjection(compiled.Manifest),
	}
	service, err := f.dashboardRuntime(ctx, runtimeInput)
	if err != nil {
		return nil, err
	}
	if input.State.DuckLakeSnapshotID == 0 {
		snapshotID := service.DuckLakeSnapshotID()
		if snapshotID > 0 {
			if err := service.Close(); err != nil {
				return nil, err
			}
			runtimeInput.SnapshotID = snapshotID
			service, err = f.dashboardRuntime(ctx, runtimeInput)
			if err != nil {
				return nil, err
			}
		}
	}
	return dashboardRuntimeWithGraph{
		Service: service, workspaceID: string(input.State.WorkspaceID),
		servingStateID: string(input.State.ID), graph: compiled.Graph,
	}, nil
}

func runtimeFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func shortDigest(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
