package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	analyticsduckdb "github.com/Yacobolo/libredash/internal/analytics/duckdb"
	analyticsmaterialize "github.com/Yacobolo/libredash/internal/analytics/materialize"
	semanticquery "github.com/Yacobolo/libredash/internal/analytics/query"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	dashboardruntime "github.com/Yacobolo/libredash/internal/dashboard/runtime"
	deploymentfs "github.com/Yacobolo/libredash/internal/deployment/filesystem"
	"github.com/Yacobolo/libredash/internal/runtimehost"
)

type deploymentRuntimeFactory struct {
	dataDir    string
	duckDBDir  string
	runtimeDir string
}

func (f deploymentRuntimeFactory) Prepare(_ context.Context, input runtimehost.RuntimeInput) (runtimehost.Runtime, error) {
	dataDir := runtimeFirstNonEmpty(input.DataDir, f.dataDir)
	duckDBDir := runtimeFirstNonEmpty(input.DuckDBDir, f.duckDBDir)
	runtimeDir := runtimeFirstNonEmpty(input.RuntimeDir, f.runtimeDir)
	targetDir := filepath.Join(runtimeDir, string(input.Deployment.ID)+"-"+shortDigest(input.Artifact.Digest))
	if err := os.RemoveAll(targetDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, err
	}
	if err := deploymentfs.ExtractArtifact(input.Artifact.Path, targetDir); err != nil {
		return nil, err
	}
	duckDir := filepath.Join(duckDBDir, string(input.Deployment.ID))
	compiled, _, err := deploymentfs.LoadCompiledWorkspaceArtifact(targetDir)
	if err != nil {
		return nil, err
	}
	if compiled.WorkspaceID != string(input.Deployment.WorkspaceID) {
		return nil, fmt.Errorf("compiled artifact workspace = %q, want %q", compiled.WorkspaceID, input.Deployment.WorkspaceID)
	}
	service, err := dashboardruntime.NewFromDefinition(dataDir, duckDir, dashboardDataRuntimeFactory{}, compiled.Definition)
	if err != nil {
		return nil, err
	}
	return service, nil
}

type dashboardDataRuntimeFactory struct{}

func (dashboardDataRuntimeFactory) OpenDashboardDataRuntime(ctx context.Context, config dashboardruntime.DataRuntimeConfig) (dashboardruntime.DataRuntime, error) {
	runtime, err := analyticsduckdb.OpenMaterializeRuntime(ctx, analyticsmaterialize.RuntimeConfig{
		ModelID: config.ModelID,
		Model:   config.Model,
		DataDir: config.DataDir,
		DBDir:   config.DBDir,
	})
	if err != nil {
		return nil, err
	}
	return dashboardDataRuntime{
		runtime: runtime,
		data:    reportdef.NewAnalyticsDataService(runtime.Queries()),
	}, nil
}

type dashboardDataRuntime struct {
	runtime *analyticsmaterialize.Runtime
	data    reportdef.DataService
}

func (r dashboardDataRuntime) Query(ctx context.Context, request reportdef.AggregateQuery) (reportdef.QueryRows, error) {
	return r.data.Query(ctx, request)
}

func (r dashboardDataRuntime) Rows(ctx context.Context, request reportdef.RowQuery) (reportdef.QueryRows, error) {
	return r.data.Rows(ctx, request)
}

func (r dashboardDataRuntime) Count(ctx context.Context, request reportdef.CountQuery) (int, error) {
	return r.data.Count(ctx, request)
}

func (r dashboardDataRuntime) Histogram(ctx context.Context, request reportdef.RawValueQuery, binCount int) ([]reportdef.HistogramBin, error) {
	return r.data.Histogram(ctx, request, binCount)
}

func (r dashboardDataRuntime) Distribution(ctx context.Context, request reportdef.RawValueQuery, sort []reportdef.QuerySort, limit int) (reportdef.QueryRows, error) {
	return r.data.Distribution(ctx, request, sort, limit)
}

func (r dashboardDataRuntime) CountModelTable(ctx context.Context, table string) (int, error) {
	return r.runtime.CountModelTable(ctx, table)
}

func (r dashboardDataRuntime) PreviewModelTable(ctx context.Context, request reportdef.ModelTableQuery) (reportdef.QueryRows, error) {
	rows, err := r.runtime.ModelTableRows(ctx, analyticsmaterialize.ModelTableQuery{
		Table:   request.Table,
		Columns: request.Columns,
		Sort:    reportSortToSemanticSort(request.Sort),
		Limit:   request.Limit,
		Offset:  request.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make(reportdef.QueryRows, 0, len(rows))
	for _, row := range rows {
		out = append(out, reportdef.QueryRow(row))
	}
	return out, nil
}

func (r dashboardDataRuntime) Refresh(ctx context.Context) error {
	return r.runtime.Refresh(ctx)
}

func (r dashboardDataRuntime) RefreshTables(ctx context.Context, tableNames []string) error {
	return r.runtime.RefreshModelTables(ctx, tableNames)
}

func (r dashboardDataRuntime) Close() error {
	return r.runtime.Close()
}

func (r dashboardDataRuntime) LastRefresh() time.Time {
	return r.runtime.LastRefresh()
}

func runtimeFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func reportSortToSemanticSort(sort []reportdef.QuerySort) []semanticquery.Sort {
	out := make([]semanticquery.Sort, 0, len(sort))
	for _, item := range sort {
		out = append(out, semanticquery.Sort{Field: item.Field, Direction: item.Direction})
	}
	return out
}
