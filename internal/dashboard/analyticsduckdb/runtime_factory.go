package dashboardadapter

import (
	"context"
	"fmt"

	analyticsduckdb "github.com/Yacobolo/leapview/internal/analytics/duckdb"
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
		if config.Resources == nil || config.Resources.WorkspaceDatabase() == nil {
			return nil, fmt.Errorf("analytical runtime database is unavailable")
		}
		return dashboardruntime.NewFromDefinition(ctx, input.Directory, NewFactory(Options{
			Database: config.Resources.WorkspaceDatabase(), CredentialResolver: analyticsduckdb.EnvironmentCredentialResolver{},
			CachePool: config.Resources.ResultCache(), ResultLimits: dataquery.ResultLimits{MaxRows: config.MaxRows, MaxBytes: config.MaxBytes},
			SnapshotID: input.SnapshotID, ServingStateID: input.ServingStateID, WorkspaceID: input.WorkspaceID,
			Environment: input.Environment, SemanticModelDigest: input.SemanticModelDigest,
			ArtifactDigest: input.ArtifactDigest, SourceDataDigest: input.SourceDataDigest,
		}), input.Definition)
	}
}
