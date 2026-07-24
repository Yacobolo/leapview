package analyticsruntime

import (
	"context"
	"fmt"

	"github.com/Yacobolo/leapview/internal/analytics/dataquery"
	analyticscontract "github.com/Yacobolo/leapview/internal/analytics/runtime"
	dashboardruntime "github.com/Yacobolo/leapview/internal/dashboard/runtime"
	dashboardruntimefactory "github.com/Yacobolo/leapview/internal/dashboard/runtimefactory"
)

type RuntimeFactoryConfig struct {
	Workspaces analyticscontract.WorkspaceFactory
	MaxRows    int
	MaxBytes   int64
}

func NewRuntimeBuilder(config RuntimeFactoryConfig) dashboardruntimefactory.Builder {
	return func(ctx context.Context, input dashboardruntimefactory.Input) (*dashboardruntime.Service, error) {
		if config.Workspaces == nil {
			return nil, fmt.Errorf("analytical workspace factory is unavailable")
		}
		return dashboardruntime.NewFromDefinition(ctx, input.Directory, NewFactory(Options{
			Workspaces: config.Workspaces, ResultLimits: dataquery.ResultLimits{MaxRows: config.MaxRows, MaxBytes: config.MaxBytes},
			SnapshotID: input.SnapshotID, ServingStateID: input.ServingStateID, WorkspaceID: input.WorkspaceID,
			Environment: input.Environment, SemanticModelDigest: input.SemanticModelDigest,
			ArtifactDigest: input.ArtifactDigest, SourceDataDigest: input.SourceDataDigest,
		}), input.Definition)
	}
}
