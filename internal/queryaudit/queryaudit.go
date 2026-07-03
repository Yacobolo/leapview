package queryaudit

import (
	"context"
	"fmt"
	"strings"
)

type EventInput struct {
	WorkspaceID   string
	PrincipalID   string
	Surface       string
	Operation     string
	QueryKind     string
	ModelID       string
	Target        string
	ObjectType    string
	ObjectID      string
	RequestID     string
	CorrelationID string
	Status        string
	DurationMS    int64
	RowsReturned  int
	BytesEstimate int64
	Error         string
	SQL           string
	PlanText      string
	QueryJSON     string
}

type Event struct {
	ID string
	EventInput
	CreatedAt string
}

type Filter struct {
	WorkspaceID string
	PrincipalID string
	Surface     string
	Operation   string
	QueryKind   string
	ModelID     string
	Target      string
	Status      string
	Search      string
	From        string
	To          string
	CursorTime  string
	CursorID    string
	PageToken   string
	Limit       int
}

type Repository interface {
	RecordQueryEvent(ctx context.Context, input EventInput) error
	GetQueryEvent(ctx context.Context, id string) (Event, error)
	ListQueryEvents(ctx context.Context, filter Filter) ([]Event, error)
}

func (input EventInput) Validate() error {
	if strings.TrimSpace(input.PrincipalID) == "" {
		return fmt.Errorf("query event principal id is required")
	}
	return nil
}
