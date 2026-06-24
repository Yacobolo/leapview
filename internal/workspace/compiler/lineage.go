package compiler

import (
	"fmt"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/workspace"
)

func ExtractLineage(workspaceID workspace.WorkspaceID, deploymentID workspace.DeploymentID, definition *workspace.Definition) (workspace.AssetGraph, error) {
	graph := workspace.AssetGraph{}
	byKey := map[string]workspace.AssetID{}
	seenEdges := map[string]struct{}{}
	add := func(typ workspace.AssetType, key string, parentID workspace.AssetID, title, description string, content any) (workspace.AssetID, error) {
		asset, err := workspace.NewAsset(workspaceID, deploymentID, typ, key, parentID, title, description, content)
		if err != nil {
			return "", err
		}
		graph.Assets = append(graph.Assets, asset)
		byKey[string(typ)+":"+key] = asset.ID
		return asset.ID, nil
	}
	edge := func(fromID, toID workspace.AssetID, typ workspace.AssetEdgeType) {
		if fromID == "" || toID == "" {
			return
		}
		key := string(fromID) + "|" + string(toID) + "|" + string(typ)
		if _, ok := seenEdges[key]; ok {
			return
		}
		seenEdges[key] = struct{}{}
		graph.Edges = append(graph.Edges, workspace.NewAssetEdge(workspaceID, deploymentID, fromID, toID, typ))
	}
	assetID := func(typ workspace.AssetType, key string) (workspace.AssetID, error) {
		id := byKey[string(typ)+":"+key]
		if id == "" {
			return "", fmt.Errorf("missing extracted %s asset %q", typ, key)
		}
		return id, nil
	}

	catalogID, err := add(workspace.AssetTypeCatalog, string(workspaceID), "", workspaceTitle(definition.Catalog.Workspace.Title), definition.Catalog.Workspace.Description, definition.Catalog)
	if err != nil {
		return workspace.AssetGraph{}, err
	}
	for _, modelEntry := range definition.Catalog.SemanticModels {
		model := definition.Models[modelEntry.ID]
		modelID, err := add(workspace.AssetTypeSemanticModel, modelEntry.ID, catalogID, modelEntry.Title, modelEntry.Description, model)
		if err != nil {
			return workspace.AssetGraph{}, err
		}
		edge(catalogID, modelID, workspace.AssetEdgeContains)
		for connectionName, connection := range model.Connections {
			id, err := add(workspace.AssetTypeConnection, modelEntry.ID+"."+connectionName, modelID, connectionName, connectionDescription(connection), connection)
			if err != nil {
				return workspace.AssetGraph{}, err
			}
			edge(modelID, id, workspace.AssetEdgeContains)
		}
		for sourceName, source := range model.Sources {
			id, err := add(workspace.AssetTypeSource, modelEntry.ID+"."+sourceName, modelID, sourceName, source.Description(), source)
			if err != nil {
				return workspace.AssetGraph{}, err
			}
			edge(modelID, id, workspace.AssetEdgeContains)
			connectionID, err := assetID(workspace.AssetTypeConnection, modelEntry.ID+"."+source.Connection)
			if err != nil {
				return workspace.AssetGraph{}, err
			}
			edge(id, connectionID, workspace.AssetEdgeUsesConnection)
		}
		for tableName, table := range model.Tables {
			id, err := add(workspace.AssetTypeModelTable, modelEntry.ID+"."+tableName, modelID, tableName, table.Description, table)
			if err != nil {
				return workspace.AssetGraph{}, err
			}
			edge(modelID, id, workspace.AssetEdgeContains)
			for _, sourceName := range table.SourceDependencies {
				sourceID, err := assetID(workspace.AssetTypeSource, modelEntry.ID+"."+sourceName)
				if err != nil {
					return workspace.AssetGraph{}, err
				}
				edge(id, sourceID, workspace.AssetEdgeReadsSource)
			}
			for fieldName, field := range table.Dimensions {
				fieldID, err := add(workspace.AssetTypeField, modelEntry.ID+"."+tableName+"."+fieldName, id, dimensionLabel(fieldName, field.Label), "", field)
				if err != nil {
					return workspace.AssetGraph{}, err
				}
				edge(id, fieldID, workspace.AssetEdgeContains)
			}
		}
		for measureName, measure := range model.Measures {
			id, err := add(workspace.AssetTypeMeasure, modelEntry.ID+"."+measureName, modelID, measureLabel(measureName, measure.Label), measure.Description, measure)
			if err != nil {
				return workspace.AssetGraph{}, err
			}
			edge(modelID, id, workspace.AssetEdgeContains)
			tableID, err := assetID(workspace.AssetTypeModelTable, modelEntry.ID+"."+measure.Table)
			if err != nil {
				return workspace.AssetGraph{}, err
			}
			edge(id, tableID, workspace.AssetEdgeUsesModelTable)
		}
	}
	for _, reportEntry := range definition.Catalog.Dashboards {
		report := definition.Dashboards[reportEntry.ID]
		reportID, err := add(workspace.AssetTypeDashboard, reportEntry.ID, catalogID, reportEntry.Title, reportEntry.Description, report)
		if err != nil {
			return workspace.AssetGraph{}, err
		}
		modelID, err := assetID(workspace.AssetTypeSemanticModel, report.SemanticModel)
		if err != nil {
			return workspace.AssetGraph{}, err
		}
		edge(reportID, modelID, workspace.AssetEdgeUsesSemanticModel)
		model := definition.Models[report.SemanticModel]
		usedTables := map[string]bool{}
		addTableUse := func(tableName string) error {
			if tableName == "" || usedTables[tableName] {
				return nil
			}
			tableID, err := assetID(workspace.AssetTypeModelTable, report.SemanticModel+"."+tableName)
			if err != nil {
				return err
			}
			edge(reportID, tableID, workspace.AssetEdgeUsesModelTable)
			usedTables[tableName] = true
			return nil
		}
		addMeasureUse := func(ref reportdef.FieldRef) error {
			if ref.Measure.Expression != "" || ref.Measure.Expr != "" {
				return addTableUse(ref.Measure.Table)
			}
			measure, err := model.ResolveMeasure(ref.Field)
			if err != nil {
				return err
			}
			measureID, err := assetID(workspace.AssetTypeMeasure, report.SemanticModel+"."+measure.Name)
			if err != nil {
				return err
			}
			edge(reportID, measureID, workspace.AssetEdgeUsesMeasure)
			return addTableUse(measure.Table)
		}
		addFieldUse := func(fromID workspace.AssetID, ref string, edgeType workspace.AssetEdgeType) error {
			if ref == "" {
				return nil
			}
			if dimension, err := model.ResolveDimension(ref); err == nil {
				fieldID, err := assetID(workspace.AssetTypeField, report.SemanticModel+"."+dimension.Field)
				if err != nil {
					return err
				}
				edge(fromID, fieldID, edgeType)
				return addTableUse(dimension.Table)
			}
			measure, err := model.ResolveMeasure(ref)
			if err != nil {
				return err
			}
			measureID, err := assetID(workspace.AssetTypeMeasure, report.SemanticModel+"."+measure.Name)
			if err != nil {
				return err
			}
			edge(fromID, measureID, workspace.AssetEdgeUsesMeasure)
			return addTableUse(measure.Table)
		}
		for _, visual := range report.Visuals {
			for _, measureRef := range visual.Query.Measures {
				if err := addMeasureUse(measureRef); err != nil {
					return workspace.AssetGraph{}, err
				}
			}
		}
		for _, table := range report.Tables {
			for _, column := range table.DataColumns {
				if err := addFieldUse(reportID, column.Field, workspace.AssetEdgeUsesField); err != nil {
					return workspace.AssetGraph{}, err
				}
			}
			for _, measureRef := range table.Query.Measures {
				if err := addMeasureUse(measureRef); err != nil {
					return workspace.AssetGraph{}, err
				}
			}
		}
		for _, page := range report.Pages {
			pageID, err := add(workspace.AssetTypePage, report.ID+"."+page.ID, reportID, page.Title, page.Description, page)
			if err != nil {
				return workspace.AssetGraph{}, err
			}
			edge(reportID, pageID, workspace.AssetEdgeContains)
		}
		for filterName, filter := range report.Filters {
			filterID, err := add(workspace.AssetTypeFilter, report.ID+"."+filterName, reportID, filter.Label, "", filter)
			if err != nil {
				return workspace.AssetGraph{}, err
			}
			if err := addFieldUse(filterID, filter.Dimension, workspace.AssetEdgeFiltersField); err != nil {
				return workspace.AssetGraph{}, err
			}
		}
		for visualName, visual := range report.Visuals {
			visualID, err := add(workspace.AssetTypeVisual, report.ID+"."+visualName, reportID, visual.Title, "", visual)
			if err != nil {
				return workspace.AssetGraph{}, err
			}
			for _, measure := range visual.Query.Measures {
				if measure.Measure.Expression != "" || measure.Measure.Expr != "" {
					if err := addTableUse(measure.Measure.Table); err != nil {
						return workspace.AssetGraph{}, err
					}
					continue
				}
				resolved, err := model.ResolveMeasure(measure.Field)
				if err != nil {
					return workspace.AssetGraph{}, err
				}
				measureID, err := assetID(workspace.AssetTypeMeasure, report.SemanticModel+"."+resolved.Name)
				if err != nil {
					return workspace.AssetGraph{}, err
				}
				edge(visualID, measureID, workspace.AssetEdgeUsesMeasure)
			}
			for _, dimension := range visual.Query.Dimensions {
				if err := addFieldUse(visualID, dimension.Field, workspace.AssetEdgeUsesField); err != nil {
					return workspace.AssetGraph{}, err
				}
			}
			if !visual.Query.Series.IsZero() {
				if err := addFieldUse(visualID, visual.Query.Series.Field, workspace.AssetEdgeUsesField); err != nil {
					return workspace.AssetGraph{}, err
				}
			}
		}
		for tableName, table := range report.Tables {
			tableID, err := add(workspace.AssetTypeTable, report.ID+"."+tableName, reportID, table.Title, "", table)
			if err != nil {
				return workspace.AssetGraph{}, err
			}
			if err := addTableUse(table.Query.Table); err != nil {
				return workspace.AssetGraph{}, err
			}
			for _, column := range table.DataColumns {
				if err := addFieldUse(tableID, column.Field, workspace.AssetEdgeUsesField); err != nil {
					return workspace.AssetGraph{}, err
				}
			}
			for _, row := range table.Rows {
				if err := addFieldUse(tableID, row, workspace.AssetEdgeUsesField); err != nil {
					return workspace.AssetGraph{}, err
				}
			}
			for _, dimension := range table.ColumnDims {
				if err := addFieldUse(tableID, dimension, workspace.AssetEdgeUsesField); err != nil {
					return workspace.AssetGraph{}, err
				}
			}
			for _, measure := range table.Measures {
				resolved, err := model.ResolveMeasure(measure)
				if err != nil {
					return workspace.AssetGraph{}, err
				}
				measureID, err := assetID(workspace.AssetTypeMeasure, report.SemanticModel+"."+resolved.Name)
				if err != nil {
					return workspace.AssetGraph{}, err
				}
				edge(tableID, measureID, workspace.AssetEdgeUsesMeasure)
			}
		}
	}
	return graph, nil
}

func connectionDescription(connection semanticmodel.Connection) string {
	if connection.Kind == "" {
		return "connection"
	}
	return connection.Kind + " connection"
}

func dimensionLabel(name, label string) string {
	if label != "" {
		return label
	}
	return name
}

func measureLabel(name, label string) string {
	if label != "" {
		return label
	}
	return name
}
