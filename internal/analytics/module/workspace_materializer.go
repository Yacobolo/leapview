package module

import (
	"context"
	"fmt"

	analyticsduckdb "github.com/Yacobolo/leapview/internal/analytics/duckdb"
	analyticsducklake "github.com/Yacobolo/leapview/internal/analytics/ducklake"
	analyticsmaterialization "github.com/Yacobolo/leapview/internal/analytics/materialization"
	servingstate "github.com/Yacobolo/leapview/internal/servingstate"
)

type duckDBWorkspaceMaterializer struct {
	environment *analyticsducklake.Environment
}

func (e duckDBWorkspaceMaterializer) MaterializeWorkspace(ctx context.Context, request analyticsmaterialization.WorkspaceRequest) (int64, error) {
	runtime, err := analyticsduckdb.OpenWorkspaceMaterializeRuntime(ctx, analyticsduckdb.WorkspaceRuntimeConfig{
		Models: request.Models, Database: e.environment,
		CredentialResolver: analyticsduckdb.EnvironmentCredentialResolver{},
		ServingStateID:     request.ServingStateID, WorkspaceID: request.WorkspaceID,
		Environment: string(servingstate.NormalizeEnvironment(request.Environment)),
		TargetType:  request.TargetType, TargetID: request.TargetID,
		SemanticDigest: request.SemanticDigest, ArtifactDigest: request.ArtifactDigest,
		SkipInitialRefresh: true,
	})
	if err != nil {
		return 0, err
	}
	defer runtime.Close()
	if err := runtime.RefreshWorkspaceTables(ctx, request.Tables); err != nil {
		return 0, err
	}
	snapshotID := runtime.DuckLakeSnapshotID()
	if snapshotID <= 0 {
		return 0, fmt.Errorf("refresh did not produce a DuckLake snapshot")
	}
	return snapshotID, nil
}
