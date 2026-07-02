package runtime

import (
	"context"
	"time"

	analyticsduckdb "github.com/Yacobolo/libredash/internal/analytics/duckdb"
	materializeruntime "github.com/Yacobolo/libredash/internal/analytics/materialize"
	semanticquery "github.com/Yacobolo/libredash/internal/analytics/query"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
)

type testDataRuntimeFactory struct{}

func (testDataRuntimeFactory) OpenDashboardDataRuntime(ctx context.Context, config DataRuntimeConfig) (DataRuntime, error) {
	runtime, err := analyticsduckdb.OpenMaterializeRuntime(ctx, materializeruntime.RuntimeConfig{
		ModelID: config.ModelID,
		Model:   config.Model,
		DataDir: config.DataDir,
		DBDir:   config.DBDir,
	})
	if err != nil {
		return nil, err
	}
	queries := runtime.Queries
	return testDataRuntime{
		runtime: runtime,
		data:    reportdef.NewAnalyticsDataService(queries()),
	}, nil
}

type testDataRuntime struct {
	runtime *materializeruntime.Runtime
	data    reportdef.DataService
}

func (r testDataRuntime) Query(ctx context.Context, request reportdef.AggregateQuery) (reportdef.QueryRows, error) {
	return r.data.Query(ctx, request)
}

func (r testDataRuntime) Rows(ctx context.Context, request reportdef.RowQuery) (reportdef.QueryRows, error) {
	return r.data.Rows(ctx, request)
}

func (r testDataRuntime) Count(ctx context.Context, request reportdef.CountQuery) (int, error) {
	return r.data.Count(ctx, request)
}

func (r testDataRuntime) Histogram(ctx context.Context, request reportdef.RawValueQuery, binCount int) ([]reportdef.HistogramBin, error) {
	return r.data.Histogram(ctx, request, binCount)
}

func (r testDataRuntime) Distribution(ctx context.Context, request reportdef.RawValueQuery, sort []reportdef.QuerySort, limit int) (reportdef.QueryRows, error) {
	return r.data.Distribution(ctx, request, sort, limit)
}

func (r testDataRuntime) CountModelTable(ctx context.Context, table string) (int, error) {
	return r.runtime.CountModelTable(ctx, table)
}

func (r testDataRuntime) PreviewModelTable(ctx context.Context, request reportdef.ModelTableQuery) (reportdef.QueryRows, error) {
	rows, err := r.runtime.ModelTableRows(ctx, materializeruntime.ModelTableQuery{
		Table:   request.Table,
		Columns: request.Columns,
		Sort:    testReportSortToSemanticSort(request.Sort),
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

func (r testDataRuntime) Refresh(ctx context.Context) error {
	return r.runtime.Refresh(ctx)
}

func (r testDataRuntime) RefreshTables(ctx context.Context, tableNames []string) error {
	return r.runtime.RefreshModelTables(ctx, tableNames)
}

func (r testDataRuntime) Close() error {
	return r.runtime.Close()
}

func (r testDataRuntime) LastRefresh() time.Time {
	return r.runtime.LastRefresh()
}

func testReportSortToSemanticSort(sort []reportdef.QuerySort) []semanticquery.Sort {
	out := make([]semanticquery.Sort, 0, len(sort))
	for _, item := range sort {
		out = append(out, semanticquery.Sort{Field: item.Field, Direction: item.Direction})
	}
	return out
}
