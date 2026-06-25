package app

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/api"
	"github.com/Yacobolo/libredash/internal/dashboard"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/go-chi/chi/v5"
)

var allowedSearchTypes = map[string]struct{}{
	"asset":          {},
	"dashboard":      {},
	"dataset":        {},
	"field":          {},
	"filter":         {},
	"measure":        {},
	"page":           {},
	"semantic_model": {},
	"source":         {},
	"table":          {},
	"visual":         {},
}

type searchCandidate struct {
	result api.SearchResult
	terms  []string
	score  int
}

func (s *Server) searchWorkspace(w http.ResponseWriter, r *http.Request) {
	types, err := searchTypeFilter(r.URL.Query().Get("types"))
	if err != nil {
		writeJSONError(w, err, http.StatusBadRequest)
		return
	}
	workspaceID := s.workspaceID(chi.URLParam(r, "workspace"))
	results, err := s.workspaceSearchResults(r, workspaceID, r.URL.Query().Get("q"), types)
	if err != nil {
		writeJSONError(w, err, statusForNotFound(err))
		return
	}
	items, nextCursor, ok := pageSliceForRequest(w, r, results)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, api.SearchResponse{Items: items, Page: api.PageInfo{NextCursor: nextCursor}})
}

func (s *Server) workspaceSearchResults(r *http.Request, workspaceID, query string, types map[string]struct{}) ([]api.SearchResult, error) {
	candidates := make([]searchCandidate, 0)
	if s.workspaceUsesRuntimeCatalog(workspaceID) {
		candidates = append(candidates, s.workspaceSearchCandidates(workspaceID)...)
	}
	assets, _, err := s.workspaceAssetsAndEdges(r, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, asset := range assets {
		name := firstNonEmpty(asset.Title, asset.Key, asset.ID)
		candidates = append(candidates, searchCandidate{
			result: api.SearchResult{
				ID:          asset.ID,
				Type:        "asset",
				Name:        name,
				Description: firstNonEmpty(asset.Description, asset.Type+" asset "+asset.Key),
				AssetID:     asset.ID,
			},
			terms: []string{asset.ID, asset.Type, asset.Key, asset.Title, asset.Description},
			score: -10,
		})
	}
	return filterRankSearchCandidates(candidates, query, types), nil
}

func (s *Server) workspaceUsesRuntimeCatalog(workspaceID string) bool {
	workspaceID = s.workspaceID(workspaceID)
	if workspaceID == s.workspaceID("") {
		return true
	}
	if catalogWorkspaceID := s.metrics.Catalog().Workspace.ID; catalogWorkspaceID != "" && workspaceID == catalogWorkspaceID {
		return true
	}
	return false
}

func (s *Server) workspaceSearchCandidates(workspaceID string) []searchCandidate {
	catalog := s.metrics.Catalog()
	candidates := make([]searchCandidate, 0)
	for _, dashboardSummary := range catalog.Dashboards {
		report, model, ok := s.metrics.Report(dashboardSummary.ID)
		if !ok {
			continue
		}
		candidates = append(candidates, dashboardSearchCandidates(report, model, s.metrics.Pages(report.ID))...)
	}
	for _, modelSummary := range catalog.Models {
		model := semanticModelForID(s.metrics, modelSummary.ID)
		candidates = append(candidates, semanticModelSearchCandidates(modelSummary.ID, modelSummary.Title, modelSummary.Description, model)...)
	}
	if workspaceID != "" {
		candidates = append(candidates, searchCandidate{
			result: api.SearchResult{
				ID:          workspaceID,
				Type:        "asset",
				Name:        workspaceID,
				Description: "Workspace " + workspaceID,
			},
			terms: []string{workspaceID, "workspace"},
			score: -20,
		})
	}
	return candidates
}

func dashboardSearchCandidates(report reportdef.Dashboard, model *semanticmodel.Model, pages []dashboard.Page) []searchCandidate {
	out := []searchCandidate{{
		result: api.SearchResult{
			ID:          report.ID,
			Type:        "dashboard",
			Name:        firstNonEmpty(report.Title, report.ID),
			Description: firstNonEmpty(report.Description, "Dashboard "+report.ID),
			DashboardID: report.ID,
			ModelID:     report.SemanticModel,
		},
		terms: append([]string{report.ID, report.Title, report.Description, report.SemanticModel}, dashboardDeepTerms(report)...),
		score: 20,
	}}
	if len(pages) == 0 {
		pages = report.Pages
	}
	for _, page := range pages {
		page = page.WithDefaults()
		out = append(out, searchCandidate{
			result: api.SearchResult{
				ID:          report.ID + "." + page.ID,
				Type:        "page",
				Name:        firstNonEmpty(page.Title, page.ID),
				Description: firstNonEmpty(page.Description, "Page "+page.ID+" in "+firstNonEmpty(report.Title, report.ID)),
				DashboardID: report.ID,
				PageID:      page.ID,
				ModelID:     report.SemanticModel,
			},
			terms: []string{report.ID, report.Title, page.ID, page.Title, page.Description, report.SemanticModel},
			score: 10,
		})
		for _, component := range page.PlacedVisuals() {
			out = append(out, dashboardComponentSearchCandidate(report, page, component))
		}
	}
	if model != nil && report.SemanticModel == "" {
		for i := range out {
			out[i].result.ModelID = model.Name
			out[i].terms = append(out[i].terms, model.Name, model.Title)
		}
	}
	return out
}

func dashboardDeepTerms(report reportdef.Dashboard) []string {
	terms := make([]string, 0)
	for id, filter := range report.Filters {
		terms = append(terms, id, filter.Label, filter.Description, filter.Dimension, filter.Type, filter.URLParam)
	}
	for id, visual := range report.Visuals {
		terms = append(terms, id, visual.Title, visual.Description, visual.Type, visual.Kind, visual.Shape, visual.Query.Time.Field)
		terms = append(terms, fieldRefTerms(visual.Query.Dimensions)...)
		terms = append(terms, fieldRefTerms(visual.Query.Measures)...)
	}
	for id, table := range report.Tables {
		terms = append(terms, id, table.Title, table.Description, table.Query.Table)
		terms = append(terms, table.Query.Fields...)
		terms = append(terms, fieldRefTerms(table.Query.Columns)...)
		terms = append(terms, fieldRefTerms(table.Query.Rows)...)
		terms = append(terms, fieldRefTerms(table.Query.Measures)...)
	}
	for _, page := range report.Pages {
		terms = append(terms, page.ID, page.Title, page.Description)
		for _, component := range page.Visuals {
			terms = append(terms, component.ID, component.Kind, component.Title, component.Description, component.Visual, component.Table, component.Filter)
		}
	}
	return terms
}

func dashboardComponentSearchCandidate(report reportdef.Dashboard, page dashboard.Page, component dashboard.PageVisual) searchCandidate {
	switch {
	case component.Visual != "":
		visual := report.Visuals[component.Visual]
		name := firstNonEmpty(component.Title, visual.Title, component.Visual)
		description := firstNonEmpty(component.Description, visual.Description, "Visual "+component.Visual+" on "+firstNonEmpty(page.Title, page.ID))
		return searchCandidate{
			result: api.SearchResult{
				ID:          "visual:" + report.ID + "." + page.ID + "." + component.Visual,
				Type:        "visual",
				Name:        name,
				Description: description,
				DashboardID: report.ID,
				PageID:      page.ID,
				VisualID:    component.Visual,
				ModelID:     report.SemanticModel,
			},
			terms: []string{
				report.ID, report.Title, page.ID, page.Title, component.ID, component.Kind,
				component.Visual, component.Title, component.Description, visual.Title, visual.Description,
				visual.Type, visual.Kind, visual.Shape, report.SemanticModel, strings.Join(fieldRefTerms(visual.Query.Dimensions), " "),
				strings.Join(fieldRefTerms(visual.Query.Measures), " "), visual.Query.Time.Field,
			},
			score: 30,
		}
	case component.Table != "":
		table := report.Tables[component.Table]
		name := firstNonEmpty(component.Title, table.Title, component.Table)
		description := firstNonEmpty(component.Description, table.Description, "Table "+component.Table+" on "+firstNonEmpty(page.Title, page.ID))
		return searchCandidate{
			result: api.SearchResult{
				ID:          "table:" + report.ID + "." + page.ID + "." + component.Table,
				Type:        "table",
				Name:        name,
				Description: description,
				DashboardID: report.ID,
				PageID:      page.ID,
				TableID:     component.Table,
				ModelID:     report.SemanticModel,
				DatasetID:   table.Query.Table,
			},
			terms: []string{
				report.ID, report.Title, page.ID, page.Title, component.ID, component.Kind,
				component.Table, component.Title, component.Description, table.Title, table.Description,
				table.Query.Table, strings.Join(table.Query.Fields, " "), strings.Join(fieldRefTerms(table.Query.Columns), " "),
				strings.Join(fieldRefTerms(table.Query.Rows), " "), strings.Join(fieldRefTerms(table.Query.Measures), " "),
			},
			score: 25,
		}
	case component.Filter != "":
		filter := report.Filters[component.Filter]
		name := firstNonEmpty(component.Title, filter.Label, component.Filter)
		description := firstNonEmpty(component.Description, filter.Description, "Filter "+component.Filter+" on "+firstNonEmpty(page.Title, page.ID))
		return searchCandidate{
			result: api.SearchResult{
				ID:          "filter:" + report.ID + "." + page.ID + "." + component.Filter,
				Type:        "filter",
				Name:        name,
				Description: description,
				DashboardID: report.ID,
				PageID:      page.ID,
				FilterID:    component.Filter,
				ModelID:     report.SemanticModel,
				FieldID:     filter.Dimension,
			},
			terms: []string{
				report.ID, report.Title, page.ID, page.Title, component.ID, component.Kind,
				component.Filter, component.Title, component.Description, filter.Label, filter.Description,
				filter.Dimension, filter.Type, filter.URLParam,
			},
			score: 20,
		}
	default:
		name := firstNonEmpty(component.Title, component.ID)
		return searchCandidate{
			result: api.SearchResult{
				ID:          report.ID + "." + page.ID + "." + component.ID,
				Type:        "page",
				Name:        name,
				Description: firstNonEmpty(component.Description, component.Kind+" component on "+firstNonEmpty(page.Title, page.ID)),
				DashboardID: report.ID,
				PageID:      page.ID,
			},
			terms: []string{report.ID, report.Title, page.ID, page.Title, component.ID, component.Kind, component.Title, component.Description},
		}
	}
}

func semanticModelSearchCandidates(modelID, title, description string, model *semanticmodel.Model) []searchCandidate {
	out := []searchCandidate{{
		result: api.SearchResult{
			ID:          modelID,
			Type:        "semantic_model",
			Name:        firstNonEmpty(title, modelID),
			Description: firstNonEmpty(description, "Semantic model "+modelID),
			ModelID:     modelID,
		},
		terms: []string{modelID, title, description},
		score: 20,
	}}
	if model == nil {
		return out
	}
	for _, sourceID := range sortedMapKeys(model.Sources) {
		source := model.Sources[sourceID]
		out = append(out, searchCandidate{
			result: api.SearchResult{
				ID:          modelID + "." + sourceID,
				Type:        "source",
				Name:        sourceID,
				Description: firstNonEmpty(source.Description, "Source "+sourceID),
				ModelID:     modelID,
			},
			terms: []string{modelID, model.Title, sourceID, source.Description, source.Format, source.Connection, source.Object, source.Path},
			score: 10,
		})
	}
	for _, datasetID := range sortedMapKeys(model.Tables) {
		table := model.Tables[datasetID]
		out = append(out, searchCandidate{
			result: api.SearchResult{
				ID:          modelID + "." + datasetID,
				Type:        "dataset",
				Name:        datasetID,
				Description: firstNonEmpty(table.Description, "Dataset "+datasetID),
				ModelID:     modelID,
				DatasetID:   datasetID,
			},
			terms: []string{modelID, model.Title, datasetID, table.Kind, table.Source, strings.Join(table.Sources, " "), table.Description, table.PrimaryKey, table.Grain},
			score: 15,
		})
		for _, field := range semanticDatasetFields(model, datasetID, table) {
			typ := "field"
			if field.Kind == "measure" {
				typ = "measure"
			}
			out = append(out, searchCandidate{
				result: api.SearchResult{
					ID:          modelID + "." + field.ID,
					Type:        typ,
					Name:        firstNonEmpty(field.Label, field.Name, field.ID),
					Description: firstNonEmpty(field.Description, typ+" "+field.ID),
					ModelID:     modelID,
					DatasetID:   datasetID,
					FieldID:     field.ID,
				},
				terms: []string{modelID, model.Title, datasetID, field.ID, field.Name, field.Label, field.Description, field.Table, field.Unit, field.Format, field.Grain, field.Time, strings.Join(field.Grains, " ")},
				score: 25,
			})
		}
	}
	return out
}

func fieldRefTerms(fields []reportdef.FieldRef) []string {
	out := make([]string, 0, len(fields)*2)
	for _, field := range fields {
		out = append(out, field.Field, field.Alias)
	}
	return out
}

func searchTypeFilter(raw string) (map[string]struct{}, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		typ := strings.TrimSpace(part)
		if typ == "" {
			continue
		}
		if _, ok := allowedSearchTypes[typ]; !ok {
			return nil, fmt.Errorf("unknown search type %q", typ)
		}
		out[typ] = struct{}{}
	}
	return out, nil
}

func filterRankSearchCandidates(candidates []searchCandidate, query string, types map[string]struct{}) []api.SearchResult {
	tokens := searchTokens(query)
	ranked := make([]searchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if len(types) > 0 {
			if _, ok := types[candidate.result.Type]; !ok {
				continue
			}
		}
		score, ok := searchCandidateScore(candidate, tokens)
		if !ok {
			continue
		}
		candidate.score += score
		ranked = append(ranked, candidate)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].result.Type != ranked[j].result.Type {
			return ranked[i].result.Type < ranked[j].result.Type
		}
		if ranked[i].result.Name != ranked[j].result.Name {
			return ranked[i].result.Name < ranked[j].result.Name
		}
		return ranked[i].result.ID < ranked[j].result.ID
	})
	out := make([]api.SearchResult, 0, len(ranked))
	for _, candidate := range ranked {
		out = append(out, candidate.result)
	}
	return out
}

func searchCandidateScore(candidate searchCandidate, tokens []string) (int, bool) {
	if len(tokens) == 0 {
		return 0, true
	}
	name := strings.ToLower(candidate.result.Name)
	id := strings.ToLower(candidate.result.ID)
	description := strings.ToLower(candidate.result.Description)
	terms := strings.ToLower(strings.Join(candidate.terms, " "))
	total := 0
	for _, token := range tokens {
		score := 0
		switch {
		case name == token:
			score = 120
		case wordHasPrefix(name, token):
			score = 90
		case strings.Contains(name, token):
			score = 75
		case id == token || wordHasPrefix(id, token):
			score = 70
		case strings.Contains(id, token):
			score = 55
		case strings.Contains(description, token):
			score = 35
		case strings.Contains(terms, token):
			score = 20
		}
		if score == 0 {
			return 0, false
		}
		total += score
	}
	return total, true
}

func searchTokens(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	out := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return out
}

func wordHasPrefix(value, prefix string) bool {
	for _, word := range searchTokens(value) {
		if strings.HasPrefix(word, prefix) {
			return true
		}
	}
	return false
}
