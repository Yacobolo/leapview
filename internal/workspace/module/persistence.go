package module

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Yacobolo/leapview/internal/access"
	"github.com/Yacobolo/leapview/internal/workspace"
	workspacesqlite "github.com/Yacobolo/leapview/internal/workspace/sqlite"
)

type SecurableRegistrar interface {
	UpsertSecurableObject(context.Context, access.ObjectRef, string) (access.SecurableObject, error)
}

// Persistence is the workspace-owned control-plane service used by consumers
// that need workspace identity or activation metadata. It does not expose the
// underlying SQLite adapter.
type Persistence struct {
	repository workspace.Repository
}

func BuildPersistence(database *sql.DB, securables SecurableRegistrar) (*Persistence, error) {
	if database == nil {
		return nil, errors.New("workspace database is required")
	}
	return &Persistence{repository: workspacesqlite.NewRepositoryWithSecurables(database, securables)}, nil
}

func (p *Persistence) Ensure(ctx context.Context, input workspace.EnsureInput) error {
	return p.repository.Ensure(ctx, input)
}

func (p *Persistence) WorkspaceIDs(ctx context.Context) ([]string, error) {
	rows, err := p.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, string(row.ID))
	}
	return ids, nil
}

func (p *Persistence) ActiveServingStateID(ctx context.Context, workspaceID string) (string, error) {
	row, err := p.repository.ByID(ctx, workspace.WorkspaceID(workspaceID))
	return string(row.ActiveServingStateID), err
}
