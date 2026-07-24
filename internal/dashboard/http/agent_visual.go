package http

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"

	agentcontracts "github.com/Yacobolo/leapview/internal/agent/contracts"
	"github.com/Yacobolo/leapview/internal/dashboard"
	visualizationir "github.com/Yacobolo/leapview/internal/visualization/ir"
	"github.com/go-chi/chi/v5"
)

func requestsCompactDashboardVisual(r *nethttp.Request) bool {
	return agentcontracts.RequestsDashboardVisualProjection(r.Context())
}

func (h Handler) dashboardVisualAgentProjection(
	r *nethttp.Request,
	metrics Metrics,
	envelope visualizationir.VisualizationEnvelope,
	filters dashboard.Filters,
	start, limit int,
	cursorScope, snapshot string,
) (agentcontracts.DashboardVisualQueryResult, error) {
	base, err := visualizationir.SpecificationBase(envelope.Spec)
	if err != nil {
		return agentcontracts.DashboardVisualQueryResult{}, err
	}
	if limit <= 0 || limit > maxAgentDashboardVisualRows {
		limit = maxAgentDashboardVisualRows
	}
	datasetID, fields, rows, completeness, err := dashboardVisualRows(envelope, base, start, limit)
	if err != nil {
		return agentcontracts.DashboardVisualQueryResult{}, err
	}
	hasMore := completeness.AvailableRows != nil && int64(start+len(rows)) < *completeness.AvailableRows
	var nextCursor *string
	if hasMore {
		value := encodeIndexCursor(start+len(rows), cursorScope, snapshot)
		nextCursor = &value
	}
	queryID := r.Header.Get("X-Request-ID")
	if queryID == "" {
		digest := sha256String(cursorScope)
		queryID = "query_" + digest[:24]
	}
	result := agentcontracts.DashboardVisualQueryResult{
		QueryID:         queryID,
		ServingSnapshot: snapshot,
		VisualID:        envelope.VisualID,
		Title:           base.Title,
		Type:            base.Kind,
		Mark:            dashboardVisualMark(envelope.Spec),
		DatasetID:       datasetID,
		Columns:         dashboardVisualColumns(fields),
		Rows:            rows,
		AppliedFilters:  dashboardAppliedFilters(filters),
		Status: agentcontracts.DashboardVisualStatus{
			Kind: string(envelope.Status.Kind), Message: envelope.Status.Message,
		},
		Diagnostics:  dashboardVisualDiagnostics(envelope.Diagnostics),
		Completeness: completeness,
		HasMore:      hasMore,
		NextCursor:   nextCursor,
	}
	workspaceID := chi.URLParam(r, "workspace")
	if workspaceID == "" {
		workspaceID = metrics.Catalog().Workspace.ID
	}
	if h.QueryFreshness != nil {
		modelID := metrics.ModelIDForDashboard(chi.URLParam(r, "dashboard"))
		if freshness, ok := h.QueryFreshness(r.Context(), workspaceID, modelID, snapshot); ok {
			result.Freshness = &freshness
		}
	}
	return result, nil
}

const maxAgentDashboardVisualRows = 50

func dashboardVisualRows(
	envelope visualizationir.VisualizationEnvelope,
	base visualizationir.VisualizationSpecBase,
	start, limit int,
) (string, []visualizationir.VisualizationField, [][]any, agentcontracts.DashboardVisualCompleteness, error) {
	switch state := envelope.DataState.Value.(type) {
	case *visualizationir.InlineVisualizationDataState:
		if len(state.Datasets) == 0 {
			return "", nil, nil, agentcontracts.DashboardVisualCompleteness{}, fmt.Errorf("visualization %q has no inline dataset", envelope.VisualID)
		}
		dataset := state.Datasets[0]
		schema := dashboardVisualSchema(base.Datasets, dataset.ID)
		rows := dashboardVisualPage(dataset.Rows, start, limit)
		available := int64(len(dataset.Rows))
		count := available
		return dataset.ID, dashboardVisualFieldsForColumns(schema.Fields, dataset.Columns), rows, agentcontracts.DashboardVisualCompleteness{
			ReturnedRows: int32(len(rows)), AvailableRows: &available,
			Cardinality: "exact", CardinalityCount: &count, State: string(dataset.Completeness),
		}, nil
	case *visualizationir.WindowedVisualizationDataState:
		block, ok := state.Blocks["a"]
		if !ok {
			return "", nil, nil, agentcontracts.DashboardVisualCompleteness{}, fmt.Errorf("visualization %q omitted window block a", envelope.VisualID)
		}
		rows := block.Rows
		if len(rows) > limit {
			rows = rows[:limit]
		}
		completeness := dashboardWindowCompleteness(len(rows), state.AvailableRows, start, state.Cardinality)
		return state.Schema.ID, state.Schema.Fields, rows, completeness, nil
	case *visualizationir.SpatialWindowedVisualizationDataState:
		rows := [][]any{}
		if state.Window != nil {
			rows = dashboardVisualPage(state.Window.Rows, start, limit)
		}
		available := int64(len(rows))
		if state.Cardinality.Count != nil {
			available = *state.Cardinality.Count
		}
		completeness := dashboardWindowCompleteness(len(rows), available, start, state.Cardinality)
		return state.Schema.ID, state.Schema.Fields, rows, completeness, nil
	default:
		return "", nil, nil, agentcontracts.DashboardVisualCompleteness{}, fmt.Errorf("visualization %q has unsupported data state %T", envelope.VisualID, envelope.DataState.Value)
	}
}

func dashboardWindowCompleteness(returned int, available int64, start int, cardinality visualizationir.VisualizationCardinality) agentcontracts.DashboardVisualCompleteness {
	state := "partial"
	switch {
	case available == 0:
		state = "empty"
	case int64(start+returned) >= available && cardinality.Kind == visualizationir.VisualizationCardinalityKindExact:
		state = "complete"
	}
	return agentcontracts.DashboardVisualCompleteness{
		ReturnedRows: int32(returned), AvailableRows: &available,
		Cardinality: string(cardinality.Kind), CardinalityCount: cardinality.Count, State: state,
	}
}

func dashboardVisualPage(rows [][]any, start, limit int) [][]any {
	if start >= len(rows) {
		return [][]any{}
	}
	end := min(len(rows), start+limit)
	return rows[start:end]
}

func dashboardVisualSchema(schemas []visualizationir.VisualizationDatasetSchema, id string) visualizationir.VisualizationDatasetSchema {
	for _, schema := range schemas {
		if schema.ID == id {
			return schema
		}
	}
	return visualizationir.VisualizationDatasetSchema{ID: id}
}

func dashboardVisualFieldsForColumns(fields []visualizationir.VisualizationField, columns []string) []visualizationir.VisualizationField {
	byID := make(map[string]visualizationir.VisualizationField, len(fields))
	for _, field := range fields {
		byID[field.ID] = field
	}
	out := make([]visualizationir.VisualizationField, 0, len(columns))
	for _, id := range columns {
		if field, ok := byID[id]; ok {
			out = append(out, field)
			continue
		}
		out = append(out, visualizationir.VisualizationField{
			ID: id, Label: id, Role: visualizationir.VisualizationFieldRoleMetadata,
			DataType: visualizationir.VisualizationDataTypeString, Nullable: true,
		})
	}
	return out
}

func dashboardVisualColumns(fields []visualizationir.VisualizationField) []agentcontracts.DashboardVisualColumn {
	out := make([]agentcontracts.DashboardVisualColumn, 0, len(fields))
	for _, field := range fields {
		column := agentcontracts.DashboardVisualColumn{
			ID: field.ID, SourceRef: field.SourceRef, Label: field.Label, Role: string(field.Role),
			DataType: string(field.DataType), Nullable: field.Nullable,
		}
		if field.Format != nil {
			var format map[string]any
			if encoded, err := json.Marshal(field.Format); err == nil && json.Unmarshal(encoded, &format) == nil {
				column.Format = &format
			}
		}
		if field.Time != nil && field.Time.Grain != "" {
			grain := field.Time.Grain
			column.Grain = &grain
		}
		out = append(out, column)
	}
	return out
}

func dashboardAppliedFilters(filters dashboard.Filters) agentcontracts.DashboardAppliedFilters {
	filters = filters.WithDefaults()
	var result agentcontracts.DashboardAppliedFilters
	if encoded, err := json.Marshal(filters); err == nil && json.Unmarshal(encoded, &result) == nil {
		return result
	}
	return agentcontracts.DashboardAppliedFilters{
		Controls:   map[string]agentcontracts.DashboardFilterControl{},
		Selections: []map[string]any{}, SpatialSelections: []map[string]any{},
	}
}

func dashboardVisualDiagnostics(input []visualizationir.VisualizationDiagnostic) []agentcontracts.DashboardVisualDiagnostic {
	out := make([]agentcontracts.DashboardVisualDiagnostic, 0, len(input))
	for _, diagnostic := range input {
		out = append(out, agentcontracts.DashboardVisualDiagnostic{
			Code: diagnostic.Code, Severity: string(diagnostic.Severity),
			Message: diagnostic.Message, FieldID: diagnostic.FieldID,
		})
	}
	return out
}

func dashboardVisualMark(spec visualizationir.VisualizationSpec) *string {
	var mark string
	switch value := spec.Value.(type) {
	case *visualizationir.CartesianVisualizationSpec:
		mark = string(value.Mark)
	case *visualizationir.ProportionalVisualizationSpec:
		mark = string(value.Mark)
	case *visualizationir.HierarchyVisualizationSpec:
		mark = string(value.Mark)
	case *visualizationir.PolarVisualizationSpec:
		mark = string(value.Mark)
	}
	if mark == "" {
		return nil
	}
	return &mark
}
