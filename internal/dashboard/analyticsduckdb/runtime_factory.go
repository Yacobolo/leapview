package dashboardadapter

import (
	"context"
	"fmt"

	analyticsduckdb "github.com/Yacobolo/leapview/internal/analytics/duckdb"
	analyticsducklake "github.com/Yacobolo/leapview/internal/analytics/ducklake"
	"github.com/Yacobolo/leapview/internal/analytics/resultcache"
	analyticsruntime "github.com/Yacobolo/leapview/internal/analytics/runtime"
	dashboardruntime "github.com/Yacobolo/leapview/internal/dashboard/runtime"
	dashboardruntimefactory "github.com/Yacobolo/leapview/internal/dashboard/runtimefactory"
	"github.com/Yacobolo/leapview/internal/dataquery"
)

type RuntimeFactoryConfig struct {
	Resources analyticsruntime.Resources
	MaxRows   int
	MaxBytes  int64
}

func NewRuntimeBuilder(config RuntimeFactoryConfig) dashboardruntimefactory.Builder {
	return func(ctx context.Context, input dashboardruntimefactory.Input) (*dashboardruntime.Service, error) {
		databaseValue, cacheValue := analyticsruntime.Unwrap(config.Resources)
		database, ok := databaseValue.(*analyticsducklake.Environment)
		if !ok || database == nil {
			return nil, fmt.Errorf("analytical runtime database is unavailable")
		}
		var cache *resultcache.Pool
		if cacheValue != nil {
			cache, ok = cacheValue.(*resultcache.Pool)
			if !ok {
				return nil, fmt.Errorf("analytical runtime cache is invalid")
			}
		}
		return dashboardruntime.NewFromDefinition(ctx, input.Directory, NewFactory(Options{
			Database: database, CredentialResolver: analyticsduckdb.EnvironmentCredentialResolver{},
			CachePool: cache, ResultLimits: dataquery.ResultLimits{MaxRows: config.MaxRows, MaxBytes: config.MaxBytes},
			SnapshotID: input.SnapshotID, ServingStateID: input.ServingStateID, WorkspaceID: input.WorkspaceID,
			Environment: input.Environment, SemanticModelDigest: input.SemanticModelDigest,
			ArtifactDigest: input.ArtifactDigest, SourceDataDigest: input.SourceDataDigest,
		}), input.Definition)
	}
}
