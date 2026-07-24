package module

import (
	"context"
	"fmt"

	analyticsduckdb "github.com/Yacobolo/leapview/internal/analytics/duckdb"
	"github.com/Yacobolo/leapview/internal/analytics/resultcache"
	analyticsruntime "github.com/Yacobolo/leapview/internal/analytics/runtime"
)

type workspaceRuntimeFactory struct {
	module *Module
}

func (m *Module) WorkspaceRuntimeFactory() analyticsruntime.WorkspaceFactory {
	return workspaceRuntimeFactory{module: m}
}

func (f workspaceRuntimeFactory) OpenWorkspace(ctx context.Context, request analyticsruntime.WorkspaceRequest) (analyticsruntime.Workspace, error) {
	if f.module == nil || f.module.environment == nil || f.module.cache == nil {
		return nil, fmt.Errorf("analytical runtime is unavailable")
	}
	cacheScope, err := f.module.cache.OpenScope(resultcache.ScopeID{
		WorkspaceID: request.WorkspaceID,
		RuntimeID:   request.ServingStateID,
	})
	if err != nil {
		return nil, err
	}
	runtime, err := analyticsduckdb.OpenWorkspaceMaterializeRuntime(ctx, analyticsduckdb.WorkspaceRuntimeConfig{
		Models: request.Models, Database: f.module.environment,
		CredentialResolver: analyticsduckdb.EnvironmentCredentialResolver{},
		QueryCache:         cacheScope, ResultLimits: request.ResultLimits,
		SnapshotID: request.SnapshotID, ServingStateID: request.ServingStateID,
		WorkspaceID: request.WorkspaceID, Environment: request.Environment,
		SemanticDigest: request.SemanticDigest, ArtifactDigest: request.ArtifactDigest,
		SourceDataDigest: request.SourceDataDigest,
	})
	if err != nil {
		_ = cacheScope.Close()
		return nil, err
	}
	return runtime, nil
}
