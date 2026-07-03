package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/Yacobolo/libredash/internal/platform/db"
	"github.com/Yacobolo/libredash/internal/queryaudit"
)

type Repository struct {
	q *db.Queries
}

func NewRepository(sqlDB *sql.DB) *Repository {
	return &Repository{q: db.New(sqlDB)}
}

func (r *Repository) RecordQueryEvent(ctx context.Context, input queryaudit.EventInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(input.QueryJSON) == "" {
		input.QueryJSON = "{}"
	}
	return r.q.InsertQueryEvent(ctx, db.InsertQueryEventParams{
		ID:            newID("queryevent"),
		WorkspaceID:   input.WorkspaceID,
		PrincipalID:   input.PrincipalID,
		Surface:       input.Surface,
		Operation:     input.Operation,
		QueryKind:     input.QueryKind,
		ModelID:       input.ModelID,
		Target:        input.Target,
		ObjectType:    input.ObjectType,
		ObjectID:      input.ObjectID,
		RequestID:     input.RequestID,
		CorrelationID: input.CorrelationID,
		Status:        input.Status,
		DurationMs:    input.DurationMS,
		RowsReturned:  int64(input.RowsReturned),
		BytesEstimate: input.BytesEstimate,
		Error:         input.Error,
		SqlText:       input.SQL,
		PlanText:      input.PlanText,
		QueryJson:     input.QueryJSON,
	})
}

func (r *Repository) ListQueryEvents(ctx context.Context, filter queryaudit.Filter) ([]queryaudit.Event, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if filter.PageToken != "" && filter.CursorTime == "" && filter.CursorID == "" {
		filter.CursorTime, filter.CursorID = decodePageToken(filter.PageToken)
	}
	rows, err := r.q.ListQueryEvents(ctx, db.ListQueryEventsParams{
		WorkspaceID: filter.WorkspaceID,
		PrincipalID: filter.PrincipalID,
		Surface:     filter.Surface,
		Operation:   filter.Operation,
		QueryKind:   filter.QueryKind,
		ModelID:     filter.ModelID,
		Target:      filter.Target,
		Status:      filter.Status,
		FromTime:    sqliteTime(filter.From),
		ToTime:      sqliteTime(filter.To),
		Search:      filter.Search,
		CursorTime:  sqliteTime(filter.CursorTime),
		CursorID:    filter.CursorID,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, err
	}
	events := make([]queryaudit.Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, queryaudit.Event{
			ID: row.ID,
			EventInput: queryaudit.EventInput{
				WorkspaceID:   row.WorkspaceID,
				PrincipalID:   row.PrincipalID,
				Surface:       row.Surface,
				Operation:     row.Operation,
				QueryKind:     row.QueryKind,
				ModelID:       row.ModelID,
				Target:        row.Target,
				ObjectType:    row.ObjectType,
				ObjectID:      row.ObjectID,
				RequestID:     row.RequestID,
				CorrelationID: row.CorrelationID,
				Status:        row.Status,
				DurationMS:    row.DurationMs,
				RowsReturned:  int(row.RowsReturned),
				BytesEstimate: row.BytesEstimate,
				Error:         row.Error,
				SQL:           row.SqlText,
				PlanText:      row.PlanText,
				QueryJSON:     row.QueryJson,
			},
			CreatedAt: row.CreatedAt,
		})
	}
	return events, nil
}

func newID(prefix string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return prefix + "_" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}

func decodePageToken(token string) (string, string) {
	bytes, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(string(bytes), "\x00", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func sqliteTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC().Format("2006-01-02 15:04:05")
		}
	}
	return value
}
