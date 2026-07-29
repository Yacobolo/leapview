package module

import (
	"context"

	accesssqlite "github.com/flidai/leapview/internal/access/sqlite"
	"github.com/flidai/leapview/internal/platform/transaction"
)

func ApplySnapshot(ctx context.Context, tx transaction.Transaction, servingStateID string) error {
	return accesssqlite.ApplySnapshotTx(ctx, tx, servingStateID)
}

func ActivateDashboardPublicationPrincipal(ctx context.Context, tx transaction.Transaction, workspaceID, name string) error {
	return accesssqlite.ActivateDashboardPublicationPrincipalTx(ctx, tx, workspaceID, name)
}
