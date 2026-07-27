// Package transaction defines the minimal SQL transaction contract used by
// cross-capability atomic workflows without exposing concrete SQL handles.
package transaction

import (
	"context"
	"database/sql"
)

type Transaction interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
