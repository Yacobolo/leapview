package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Yacobolo/libredash/internal/dataquery"
	"github.com/Yacobolo/libredash/internal/queryaudit"
	queryauditsqlite "github.com/Yacobolo/libredash/internal/queryaudit/sqlite"
)

type queryAuditMetrics struct {
	QueryMetrics
	recorder           queryaudit.Repository
	defaultWorkspaceID string
}

func (s *Server) queryAuditRepository() (queryaudit.Repository, error) {
	if s.store == nil {
		return nil, nil
	}
	return queryauditsqlite.NewRepository(s.store.SQLDB()), nil
}

func (m queryAuditMetrics) MetricsForWorkspace(workspaceID string) (QueryMetrics, bool) {
	provider, ok := m.QueryMetrics.(workspaceMetrics)
	if ok {
		metrics, ok := provider.MetricsForWorkspace(workspaceID)
		if !ok || metrics == nil {
			return nil, ok
		}
		return queryAuditMetrics{QueryMetrics: metrics, recorder: m.recorder, defaultWorkspaceID: workspaceID}, true
	}
	if m.QueryMetrics == nil {
		return nil, false
	}
	if m.defaultWorkspaceID != "" && workspaceID == m.defaultWorkspaceID {
		return m, true
	}
	catalog := m.QueryMetrics.Catalog()
	if catalog.Workspace.ID == "" || catalog.Workspace.ID == workspaceID {
		return m, true
	}
	return nil, false
}

func (m queryAuditMetrics) RefreshModelTables(ctx context.Context, modelID string, tableNames []string) error {
	if port, ok := m.QueryMetrics.(modelTableRefreshMetrics); ok {
		return port.RefreshModelTables(ctx, modelID, tableNames)
	}
	if port, ok := m.QueryMetrics.(modelTableRefreshRuntimeMetrics); ok {
		return port.RefreshTables(ctx, modelID, tableNames)
	}
	return errors.New("model table refresh is not configured")
}

func (m queryAuditMetrics) ExecuteDataQuery(ctx context.Context, request dataquery.Query) (dataquery.Result, error) {
	if m.QueryMetrics == nil {
		return dataquery.Result{}, errors.New("query metrics are not configured")
	}
	request = request.WithMetadata(dataquery.MetadataFromContext(ctx))
	if request.PrincipalID == "" {
		if principal, ok := principalFromContext(ctx); ok {
			request.PrincipalID = principal.ID
		}
	}
	start := time.Now()
	result, err := m.QueryMetrics.ExecuteDataQuery(ctx, request)
	duration := time.Since(start).Milliseconds()
	if result.DurationMS == 0 {
		result.DurationMS = duration
	}
	if result.RowsReturned == 0 && len(result.Rows) > 0 {
		result.RowsReturned = len(result.Rows)
	}
	if err == nil {
		result.Status = queryFirstNonEmpty(result.Status, dataquery.StatusSuccess)
	} else {
		result.Status = queryStatus(ctx, err)
		result.Error = sanitizeQueryError(err)
	}
	if m.recorder != nil {
		_ = m.recorder.RecordQueryEvent(ctx, queryEventInput(request, result))
	}
	return result, err
}

func queryEventInput(request dataquery.Query, result dataquery.Result) queryaudit.EventInput {
	return queryaudit.EventInput{
		WorkspaceID:   request.WorkspaceID,
		PrincipalID:   request.PrincipalID,
		Surface:       request.Surface,
		Operation:     request.Operation,
		QueryKind:     string(request.Kind),
		ModelID:       request.ModelID,
		Target:        request.Target,
		ObjectType:    request.ObjectType,
		ObjectID:      request.ObjectID,
		RequestID:     request.RequestID,
		CorrelationID: request.CorrelationID,
		Status:        queryFirstNonEmpty(result.Status, dataquery.StatusSuccess),
		DurationMS:    result.DurationMS,
		RowsReturned:  result.RowsReturned,
		BytesEstimate: result.BytesEstimate,
		Error:         result.Error,
		SQL:           result.SQL,
		PlanText:      result.PlanText,
		QueryJSON:     queryShapeJSON(request),
	}
}

func queryStatus(ctx context.Context, err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return dataquery.StatusCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return dataquery.StatusTimeout
	}
	return dataquery.StatusError
}

func sanitizeQueryError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 1000 {
		message = message[:1000]
	}
	return message
}

func queryShapeJSON(request dataquery.Query) string {
	type queryShape struct {
		WorkspaceID   string             `json:"workspaceId,omitempty"`
		Surface       string             `json:"surface,omitempty"`
		Operation     string             `json:"operation,omitempty"`
		RequestID     string             `json:"requestId,omitempty"`
		ObjectType    string             `json:"objectType,omitempty"`
		ObjectID      string             `json:"objectId,omitempty"`
		CorrelationID string             `json:"correlationId,omitempty"`
		ModelID       string             `json:"modelId,omitempty"`
		Kind          dataquery.Kind     `json:"kind"`
		Target        string             `json:"target,omitempty"`
		Fields        []dataquery.Field  `json:"fields,omitempty"`
		Measures      []dataquery.Field  `json:"measures,omitempty"`
		Value         dataquery.Field    `json:"value,omitempty"`
		Time          dataquery.Time     `json:"time,omitempty"`
		Filters       []dataquery.Filter `json:"filters,omitempty"`
		Sort          []dataquery.Sort   `json:"sort,omitempty"`
		Offset        int                `json:"offset,omitempty"`
		Limit         int                `json:"limit,omitempty"`
		BinCount      int                `json:"binCount,omitempty"`
		IncludeTotal  bool               `json:"includeTotal,omitempty"`
	}
	bytes, err := json.Marshal(queryShape{
		WorkspaceID:   request.WorkspaceID,
		Surface:       request.Surface,
		Operation:     request.Operation,
		RequestID:     request.RequestID,
		ObjectType:    request.ObjectType,
		ObjectID:      request.ObjectID,
		CorrelationID: request.CorrelationID,
		ModelID:       request.ModelID,
		Kind:          request.Kind,
		Target:        request.Target,
		Fields:        request.Fields,
		Measures:      request.Measures,
		Value:         request.Value,
		Time:          request.Time,
		Filters:       request.Filters,
		Sort:          request.Sort,
		Offset:        request.Offset,
		Limit:         request.Limit,
		BinCount:      request.BinCount,
		IncludeTotal:  request.IncludeTotal,
	})
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func requestQueryMetadata(r *http.Request, surface, operation, objectType, objectID string) dataquery.Metadata {
	metadata := dataquery.Metadata{
		Surface:       surface,
		Operation:     operation,
		ObjectType:    objectType,
		ObjectID:      objectID,
		RequestID:     r.Header.Get("X-Request-ID"),
		CorrelationID: r.Header.Get("X-Correlation-ID"),
	}
	if principal, ok := principalFromContext(r.Context()); ok {
		metadata.PrincipalID = principal.ID
	}
	return metadata
}

func queryFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
