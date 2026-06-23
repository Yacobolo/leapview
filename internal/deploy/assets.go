package deploy

import (
	"regexp"
	"sort"

	"github.com/Yacobolo/libredash/internal/platform"
	"github.com/Yacobolo/libredash/internal/semantic"
)

func ExtractAssets(workspaceID, deploymentID string, workspace *semantic.Workspace) ([]platform.Asset, []platform.AssetEdge, error) {
	assets := []platform.Asset{}
	edges := []platform.AssetEdge{}
	byKey := map[string]string{}
	seenEdges := map[string]struct{}{}
	add := func(typ, key, parentID, title, description string, content any) (string, error) {
		asset, err := platform.NewAsset(workspaceID, deploymentID, typ, key, parentID, title, description, content)
		if err != nil {
			return "", err
		}
		assets = append(assets, asset)
		byKey[typ+":"+key] = asset.ID
		return asset.ID, nil
	}
	edge := func(fromID, toID, typ string) {
		if fromID == "" || toID == "" {
			return
		}
		key := fromID + "|" + toID + "|" + typ
		if _, ok := seenEdges[key]; ok {
			return
		}
		seenEdges[key] = struct{}{}
		edges = append(edges, platform.NewAssetEdge(workspaceID, deploymentID, fromID, toID, typ))
	}

	catalogID, err := add("catalog", workspaceID, "", workspaceTitle(workspace.Catalog.Workspace.Title), workspace.Catalog.Workspace.Description, workspace.Catalog)
	if err != nil {
		return nil, nil, err
	}
	for _, modelEntry := range workspace.Catalog.SemanticModels {
		model := workspace.Models[modelEntry.ID]
		modelID, err := add("semantic_model", modelEntry.ID, catalogID, modelEntry.Title, modelEntry.Description, model)
		if err != nil {
			return nil, nil, err
		}
		edge(catalogID, modelID, "contains")
		for connectionName, connection := range model.Connections {
			id, err := add("connection", modelEntry.ID+"."+connectionName, modelID, connectionName, connectionDescription(connection), connection)
			if err != nil {
				return nil, nil, err
			}
			edge(modelID, id, "contains")
		}
		for sourceName, source := range model.Sources {
			id, err := add("source", modelEntry.ID+"."+sourceName, modelID, sourceName, source.Description(), source)
			if err != nil {
				return nil, nil, err
			}
			edge(modelID, id, "contains")
			edge(id, byKey["connection:"+modelEntry.ID+"."+source.Connection], "uses_connection")
		}
		for tableName, table := range model.Tables {
			id, err := add("model_table", modelEntry.ID+"."+tableName, modelID, tableName, table.Description, table)
			if err != nil {
				return nil, nil, err
			}
			edge(modelID, id, "contains")
			if table.Source != "" {
				edge(id, byKey["source:"+modelEntry.ID+"."+table.Source], "reads_source")
			} else {
				for _, sourceName := range transformSourceRefs(table.Transform.SQL, model.Sources) {
					edge(id, byKey["source:"+modelEntry.ID+"."+sourceName], "reads_source")
				}
			}
			for fieldName, field := range table.Dimensions {
				fieldID, err := add("field", modelEntry.ID+"."+tableName+"."+fieldName, id, dimensionLabel(fieldName, field.Label), "", field)
				if err != nil {
					return nil, nil, err
				}
				edge(id, fieldID, "contains")
			}
		}
		for measureName, measure := range model.Measures {
			id, err := add("measure", modelEntry.ID+"."+measureName, modelID, measureLabel(measureName, measure.Label), measure.Description, measure)
			if err != nil {
				return nil, nil, err
			}
			edge(modelID, id, "contains")
			edge(id, byKey["model_table:"+modelEntry.ID+"."+measure.Table], "uses_model_table")
		}
	}
	for _, reportEntry := range workspace.Catalog.Dashboards {
		report := workspace.Dashboards[reportEntry.ID]
		reportID, err := add("dashboard", reportEntry.ID, catalogID, reportEntry.Title, reportEntry.Description, report)
		if err != nil {
			return nil, nil, err
		}
		modelID := byKey["semantic_model:"+report.SemanticModel]
		edge(reportID, modelID, "uses_semantic_model")
		model := workspace.Models[report.SemanticModel]
		usedTables := map[string]bool{}
		addMeasureUse := func(ref string) {
			measure, err := model.ResolveMeasure(ref)
			if err != nil {
				return
			}
			edge(reportID, byKey["measure:"+report.SemanticModel+"."+measure.Name], "uses_measure")
			if !usedTables[measure.Table] {
				edge(reportID, byKey["model_table:"+report.SemanticModel+"."+measure.Table], "uses_model_table")
				usedTables[measure.Table] = true
			}
		}
		for _, visual := range report.Visuals {
			for _, measureRef := range visual.Query.Measures {
				addMeasureUse(measureRef.Field)
			}
		}
		for _, table := range report.Tables {
			for _, measureRef := range table.Query.Measures {
				addMeasureUse(measureRef.Field)
			}
		}
		for _, page := range report.Pages {
			pageID, err := add("page", report.ID+"."+page.ID, reportID, page.Title, page.Description, page)
			if err != nil {
				return nil, nil, err
			}
			edge(reportID, pageID, "contains")
		}
		for filterName, filter := range report.Filters {
			filterID, err := add("filter", report.ID+"."+filterName, reportID, filter.Label, "", filter)
			if err != nil {
				return nil, nil, err
			}
			edge(filterID, byKey["field:"+report.SemanticModel+"."+filter.Dimension], "filters_field")
		}
		for visualName, visual := range report.Visuals {
			visualID, err := add("visual", report.ID+"."+visualName, reportID, visual.Title, "", visual)
			if err != nil {
				return nil, nil, err
			}
			for _, measure := range visual.Query.Measures {
				if resolved, err := model.ResolveMeasure(measure.Field); err == nil {
					edge(visualID, byKey["measure:"+report.SemanticModel+"."+resolved.Name], "uses_measure")
				}
			}
			for _, dimension := range visual.Query.Dimensions {
				edge(visualID, byKey["field:"+report.SemanticModel+"."+dimension.Field], "uses_field")
			}
			if !visual.Query.Series.IsZero() {
				edge(visualID, byKey["field:"+report.SemanticModel+"."+visual.Query.Series.Field], "uses_field")
			}
		}
		for tableName, table := range report.Tables {
			tableID, err := add("table", report.ID+"."+tableName, reportID, table.Title, "", table)
			if err != nil {
				return nil, nil, err
			}
			for _, row := range table.Rows {
				edge(tableID, byKey["field:"+report.SemanticModel+"."+row], "uses_field")
			}
			for _, dimension := range table.ColumnDims {
				edge(tableID, byKey["field:"+report.SemanticModel+"."+dimension], "uses_field")
			}
			for _, measure := range table.Measures {
				if resolved, err := model.ResolveMeasure(measure); err == nil {
					edge(tableID, byKey["measure:"+report.SemanticModel+"."+resolved.Name], "uses_measure")
				}
			}
		}
	}
	return assets, edges, nil
}

func transformSourceRefs(sql string, sources map[string]semantic.Source) []string {
	if sql == "" || len(sources) == 0 {
		return nil
	}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		pattern := regexp.MustCompile(`(?i)\braw\.` + regexp.QuoteMeta(name) + `\b`)
		if pattern.MatchString(sql) {
			out = append(out, name)
		}
	}
	return out
}

func connectionDescription(connection semantic.Connection) string {
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
