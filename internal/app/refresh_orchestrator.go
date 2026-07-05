package app

import (
	"context"
	"errors"

	"github.com/Yacobolo/libredash/internal/analytics/materialize"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
)

type modelTableRefreshMetrics interface {
	RefreshModelTables(context.Context, string, []string) error
}

type modelTableRefreshRuntimeMetrics interface {
	RefreshTables(context.Context, string, []string) error
}

type appRefreshRunner struct {
	metrics QueryMetrics
}

func (r appRefreshRunner) RefreshMaterializations(ctx context.Context, modelID string) error {
	if r.metrics == nil {
		return errors.New("materialization refresh is not configured")
	}
	return r.metrics.RefreshMaterializations(ctx, modelID)
}

func (r appRefreshRunner) RefreshModelTables(ctx context.Context, modelID string, tableNames []string) error {
	if r.metrics == nil {
		return errors.New("model table refresh is not configured")
	}
	if port, ok := r.metrics.(modelTableRefreshMetrics); ok {
		return port.RefreshModelTables(ctx, modelID, tableNames)
	}
	if port, ok := r.metrics.(modelTableRefreshRuntimeMetrics); ok {
		return port.RefreshTables(ctx, modelID, tableNames)
	}
	return errors.New("model table refresh is not configured")
}

func refreshModelLookup(metrics QueryMetrics) materialize.ModelLookup {
	if metrics == nil {
		return nil
	}
	return func(modelID string) (*semanticmodel.Model, bool) {
		return metrics.SemanticModel(modelID)
	}
}
