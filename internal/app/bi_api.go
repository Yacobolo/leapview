package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/api"
	"github.com/Yacobolo/libredash/internal/dashboard"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/go-chi/chi/v5"
)

const maxAgentRows = 50

func (s *Server) listDashboards(w http.ResponseWriter, r *http.Request) {
	catalog := s.metrics.Catalog()
	out := make([]api.DashboardSummary, 0, len(catalog.Dashboards))
	for _, row := range catalog.Dashboards {
		out = append(out, dashboardSummaryDTO(row))
	}
	page, nextCursor, ok := pageSliceForRequest(w, r, out)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, api.DashboardListResponse{Items: page, Page: api.PageInfo{NextCursor: nextCursor}})
}

func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) {
	dashboardID := chi.URLParam(r, "dashboard")
	report, model, ok := s.metrics.Report(dashboardID)
	if !ok {
		writeJSONError(w, fmt.Errorf("dashboard %q not found", dashboardID), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, dashboardManifest(report, model, s.metrics.Pages(dashboardID)))
}

func (s *Server) listSemanticModels(w http.ResponseWriter, r *http.Request) {
	catalog := s.metrics.Catalog()
	out := make([]api.SemanticModelSummary, 0, len(catalog.Models))
	for _, row := range catalog.Models {
		out = append(out, semanticModelSummaryDTO(row))
	}
	page, nextCursor, ok := pageSliceForRequest(w, r, out)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, api.SemanticModelListResponse{Items: page, Page: api.PageInfo{NextCursor: nextCursor}})
}

func (s *Server) getSemanticModel(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "model")
	model, ok := modelDescription(s.metrics, modelID)
	if !ok {
		writeJSONError(w, fmt.Errorf("model %q not found", modelID), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, model)
}

func (s *Server) queryDashboardPage(w http.ResponseWriter, r *http.Request) {
	var input api.DashboardPageQueryRequest
	if err := decodeOptionalJSONBody(r, &input); err != nil {
		writeJSONError(w, err, http.StatusBadRequest)
		return
	}
	dashboardID := chi.URLParam(r, "dashboard")
	filters := dashboardFilters(input.Filters)
	if filters.Controls == nil && filters.Selections == nil {
		filters = s.metrics.DefaultFilters(dashboardID)
	}
	patch, err := s.metrics.QueryDashboardPage(r.Context(), dashboardID, chi.URLParam(r, "page"), filters)
	if err != nil {
		writeJSONError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, boundedPatch(patch))
}

func (s *Server) queryDashboardTable(w http.ResponseWriter, r *http.Request) {
	var input api.DashboardTableQueryRequest
	if err := decodeOptionalJSONBody(r, &input); err != nil {
		writeJSONError(w, err, http.StatusBadRequest)
		return
	}
	dashboardID := chi.URLParam(r, "dashboard")
	count := input.Count
	if count <= 0 || count > maxAgentRows {
		count = maxAgentRows
	}
	filters := dashboardFilters(input.Filters)
	if filters.Controls == nil && filters.Selections == nil {
		filters = s.metrics.DefaultFilters(dashboardID)
	}
	request := s.metrics.NormalizeTableRequest(dashboardID, dashboard.TableRequest{Table: chi.URLParam(r, "table"), Block: "a", Count: count})
	request.Count = count
	table, err := s.metrics.QueryTablePage(r.Context(), dashboardID, input.PageID, filters, request)
	if err != nil {
		writeJSONError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, boundedTable(table))
}

func dashboardFilters(raw map[string]any) dashboard.Filters {
	if len(raw) == 0 {
		return dashboard.Filters{}
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		return dashboard.Filters{}
	}
	var filters dashboard.Filters
	if err := json.Unmarshal(bytes, &filters); err != nil {
		return dashboard.Filters{}
	}
	return filters
}

func boundedPatch(patch dashboard.Patch) dashboard.Patch {
	for key, visual := range patch.Visuals {
		if len(visual.Data) > maxAgentRows {
			visual.Data = visual.Data[:maxAgentRows]
		}
		patch.Visuals[key] = visual
	}
	return patch
}

func boundedTable(table dashboard.Table) dashboard.Table {
	for key, block := range table.Blocks {
		if len(block.Rows) > maxAgentRows {
			block.Rows = block.Rows[:maxAgentRows]
		}
		table.Blocks[key] = block
	}
	if table.AvailableRows > maxAgentRows {
		table.AvailableRows = maxAgentRows
	}
	return table
}

func dashboardSummaryDTO(row dashboard.CatalogDashboard) api.DashboardSummary {
	return api.DashboardSummary{
		ID:            row.ID,
		Title:         row.Title,
		Description:   row.Description,
		SemanticModel: row.SemanticModel,
		Tags:          row.Tags,
		PageCount:     row.PageCount,
	}
}

func semanticModelSummaryDTO(row dashboard.CatalogModel) api.SemanticModelSummary {
	return api.SemanticModelSummary{ID: row.ID, Title: row.Title, Description: row.Description}
}

func modelSummary(model *semanticmodel.Model) *api.ModelRef {
	if model == nil {
		return nil
	}
	return &api.ModelRef{ID: model.Name, Title: model.Title}
}

func modelDescription(metrics queryMetrics, id string) (api.SemanticModelDescriptionResponse, bool) {
	catalog := metrics.Catalog()
	var catalogModel dashboard.CatalogModel
	for _, model := range catalog.Models {
		if model.ID == id {
			catalogModel = model
			break
		}
	}
	if catalogModel.ID == "" {
		return api.SemanticModelDescriptionResponse{}, false
	}

	out := api.SemanticModelDescriptionResponse{
		ID:          catalogModel.ID,
		Title:       catalogModel.Title,
		Description: catalogModel.Description,
		Dashboards:  dashboardsForModel(metrics, id),
	}
	if model := semanticModelForID(metrics, id); model != nil {
		fieldCount := 0
		for _, table := range model.Tables {
			fieldCount += len(table.Dimensions)
		}
		out.Counts = &api.SemanticModelCounts{
			Sources:       len(model.Sources),
			ModelTables:   len(model.Tables),
			Fields:        fieldCount,
			Measures:      len(model.Measures),
			Relationships: len(model.Relationships),
		}
		tables := make([]api.SemanticModelTableSummary, 0, len(model.Tables))
		for tableID, table := range model.Tables {
			tables = append(tables, api.SemanticModelTableSummary{
				ID:          tableID,
				Kind:        table.Kind,
				Source:      table.Source,
				Description: table.Description,
				Fields:      len(table.Dimensions),
			})
		}
		out.Tables = tables
	}
	return out, true
}

func dashboardsForModel(metrics queryMetrics, modelID string) []api.ModelDashboardUsage {
	out := make([]api.ModelDashboardUsage, 0)
	for _, dashboardSummary := range metrics.Catalog().Dashboards {
		report, model, ok := metrics.Report(dashboardSummary.ID)
		if !ok || (report.SemanticModel != modelID && (model == nil || model.Name != modelID)) {
			continue
		}
		out = append(out, api.ModelDashboardUsage{
			ID:            report.ID,
			Title:         report.Title,
			SemanticModel: report.SemanticModel,
			Pages:         len(metrics.Pages(report.ID)),
		})
	}
	return out
}

func semanticModelForID(metrics queryMetrics, modelID string) *semanticmodel.Model {
	for _, dashboardSummary := range metrics.Catalog().Dashboards {
		_, model, ok := metrics.Report(dashboardSummary.ID)
		if ok && model != nil && model.Name == modelID {
			return model
		}
	}
	return nil
}

func dashboardManifest(report reportdef.Dashboard, model *semanticmodel.Model, pages []dashboard.Page) api.DashboardManifestResponse {
	if pages == nil {
		pages = report.Pages
	}
	out := api.DashboardManifestResponse{
		ID:            report.ID,
		Title:         report.Title,
		Description:   report.Description,
		SemanticModel: report.SemanticModel,
		Model:         modelSummary(model),
		Counts: api.DashboardManifestCounts{
			Pages:   len(pages),
			Visuals: len(report.Visuals),
			Tables:  len(report.Tables),
			Filters: len(report.Filters),
		},
		Pages: make([]api.DashboardManifestPage, 0, len(pages)),
		DetailTools: map[string]string{
			"model":      "describe_model",
			"page_data":  "query_dashboard_page",
			"table_data": "query_table",
		},
	}
	for _, page := range pages {
		pageSummary := api.DashboardManifestPage{
			ID:          page.ID,
			Title:       page.Title,
			Description: page.Description,
			Components:  make([]api.DashboardManifestComponent, 0, len(page.Visuals)),
		}
		for _, component := range page.Visuals {
			pageSummary.Components = append(pageSummary.Components, dashboardComponentSummary(component, report))
		}
		out.Pages = append(out.Pages, pageSummary)
	}
	return out
}

func dashboardComponentSummary(component dashboard.PageVisual, report reportdef.Dashboard) api.DashboardManifestComponent {
	switch {
	case component.Visual != "":
		title := component.Title
		if title == "" {
			title = report.Visuals[component.Visual].Title
		}
		return api.DashboardManifestComponent{ID: component.ID, Kind: "visual", Ref: component.Visual, Title: title}
	case component.Table != "":
		title := component.Title
		if title == "" {
			title = report.Tables[component.Table].Title
		}
		return api.DashboardManifestComponent{ID: component.ID, Kind: "table", Ref: component.Table, Title: title}
	case component.Filter != "":
		title := component.Title
		if title == "" {
			title = report.Filters[component.Filter].Label
		}
		return api.DashboardManifestComponent{ID: component.ID, Kind: "filter", Ref: component.Filter, Title: title}
	default:
		kind := component.Kind
		if kind == "" {
			kind = "component"
		}
		return api.DashboardManifestComponent{ID: component.ID, Kind: kind, Title: component.Title}
	}
}
