package module

import (
	"context"

	"github.com/Yacobolo/leapview/internal/dataquery"
	"github.com/Yacobolo/leapview/internal/queryruntime"
	workspacehttp "github.com/Yacobolo/leapview/internal/workspace/http"
)

type MetricsAdapter struct{ queryruntime.Metrics }

func (m MetricsAdapter) DataExplorerModel(modelID string) (workspacehttp.DataExplorerModel, bool) {
	model, ok := m.Metrics.SemanticModel(modelID)
	if !ok || model == nil {
		return workspacehttp.DataExplorerModel{}, false
	}
	projection := workspacehttp.DataExplorerModel{
		Sources: make(map[string]workspacehttp.DataExplorerSource, len(model.Sources)),
		Tables:  make(map[string]workspacehttp.DataExplorerTable, len(model.Tables)),
	}
	for name, source := range model.Sources {
		projected := workspacehttp.DataExplorerSource{
			Fields:  make(map[string]workspacehttp.DataExplorerField, len(source.Fields)),
			Columns: make([]workspacehttp.DataExplorerColumn, 0, len(source.Schema.Columns)),
		}
		for fieldName, field := range source.Fields {
			projected.Fields[fieldName] = workspacehttp.DataExplorerField{Name: field.Name, Label: field.Name, Type: field.Type}
		}
		for _, column := range source.Schema.Columns {
			projected.Columns = append(projected.Columns, workspacehttp.DataExplorerColumn{
				Name: column.Name, PhysicalType: column.PhysicalType, Ordinal: column.Ordinal,
			})
		}
		projection.Sources[name] = projected
	}
	for name, table := range model.Tables {
		projected := workspacehttp.DataExplorerTable{
			Dimensions: make(map[string]workspacehttp.DataExplorerField, len(table.Dimensions)),
			Columns:    make(map[string]workspacehttp.DataExplorerField, len(table.Columns)),
			Schema:     make([]workspacehttp.DataExplorerColumn, 0, len(table.Schema.Columns)),
		}
		for fieldName, field := range table.Dimensions {
			projected.Dimensions[fieldName] = workspacehttp.DataExplorerField{Name: field.Name, Label: field.Label, Type: field.Type}
		}
		for fieldName, field := range table.Columns {
			projected.Columns[fieldName] = workspacehttp.DataExplorerField{Name: field.Name, Label: field.Name, Type: field.Type}
		}
		for _, column := range table.Schema.Columns {
			projected.Schema = append(projected.Schema, workspacehttp.DataExplorerColumn{
				Name: column.Name, PhysicalType: column.PhysicalType, Ordinal: column.Ordinal,
			})
		}
		projection.Tables[name] = projected
	}
	return projection, true
}

func (m MetricsAdapter) ExecuteDataPreview(ctx context.Context, request workspacehttp.DataPreviewRequest) (workspacehttp.DataPreviewResult, error) {
	sortSpec := []dataquery.Sort(nil)
	if request.SortColumn != "" {
		sortSpec = []dataquery.Sort{{Field: request.SortColumn, Direction: request.Direction}}
	}
	var query dataquery.Query
	switch request.Layer {
	case "model_table":
		query = dataquery.ModelTableRows(request.ModelID, request.Table, request.Columns, sortSpec, request.Offset, request.Limit, request.IncludeTotal)
	case "semantic_view":
		fields := make([]dataquery.Field, 0, len(request.Columns))
		for _, column := range request.Columns {
			fields = append(fields, dataquery.Field{Field: request.Table + "." + column, Alias: column})
		}
		query = dataquery.SemanticRows(request.ModelID, request.Table, fields, nil, nil, sortSpec, request.Offset, request.Limit, request.IncludeTotal)
	default:
		query = dataquery.Query{
			ModelID: request.ModelID, Kind: dataquery.Kind(request.Layer), Target: request.Table,
			Limit: request.Limit, Offset: request.Offset, IncludeTotal: request.IncludeTotal,
		}
	}
	query.WorkspaceID = request.WorkspaceID
	query = query.WithMetadata(dataquery.Metadata{
		Surface: dataquery.SurfaceDataExplorer, Operation: dataquery.OperationPreviewWindow,
		ObjectType: request.Layer, ObjectID: request.WorkspaceID + ":" + request.ObjectKey,
	})
	result, err := m.Metrics.ExecuteDataQuery(ctx, query)
	if err != nil {
		return workspacehttp.DataPreviewResult{}, err
	}
	rows := make([]map[string]any, 0, len(result.Rows))
	for _, row := range result.Rows {
		converted := make(map[string]any, len(row))
		for key, value := range row {
			converted[key] = value
		}
		rows = append(rows, converted)
	}
	return workspacehttp.DataPreviewResult{
		Rows: rows, TotalRows: result.TotalRows, TotalRowsKnown: result.TotalRowsKnown, SQL: result.SQL,
	}, nil
}
