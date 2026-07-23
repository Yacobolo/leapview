package dashboardadapter

import (
	"context"

	analyticsduckdb "github.com/Yacobolo/leapview/internal/analytics/duckdb"
	analyticsducklake "github.com/Yacobolo/leapview/internal/analytics/ducklake"
	"github.com/Yacobolo/leapview/internal/analytics/resultcache"
	dashboardruntime "github.com/Yacobolo/leapview/internal/dashboard/runtime"
	dashboardruntimefactory "github.com/Yacobolo/leapview/internal/dashboard/runtimefactory"
	"github.com/Yacobolo/leapview/internal/dataquery"
)

type RuntimeFactoryConfig struct {
	Database *analyticsducklake.Environment
	Cache    *resultcache.Pool
	MaxRows  int
	MaxBytes int64
}

func NewRuntimeBuilder(config RuntimeFactoryConfig) dashboardruntimefactory.Builder {
	return func(ctx context.Context, input dashboardruntimefactory.Input) (*dashboardruntime.Service, error) {
		return dashboardruntime.NewFromDefinition(ctx, input.Directory, NewFactory(Options{
			Database: config.Database, CredentialResolver: analyticsduckdb.EnvironmentCredentialResolver{},
			CachePool: config.Cache, ResultLimits: dataquery.ResultLimits{MaxRows: config.MaxRows, MaxBytes: config.MaxBytes},
			SnapshotID: input.SnapshotID, ServingStateID: input.ServingStateID, WorkspaceID: input.WorkspaceID,
			Environment: input.Environment, SemanticModelDigest: input.SemanticModelDigest,
			ArtifactDigest: input.ArtifactDigest, SourceDataDigest: input.SourceDataDigest,
		}), input.Definition)
	}
}
