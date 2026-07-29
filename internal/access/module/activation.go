package module

import (
	"context"

	accesssqlite "github.com/Yacobolo/leapview/internal/access/sqlite"
	"github.com/Yacobolo/leapview/internal/platform/transaction"
)

func ApplySnapshot(ctx context.Context, tx transaction.Transaction, servingStateID string) error {
	return accesssqlite.ApplySnapshotTx(ctx, tx, servingStateID)
}

func ActivateDashboardPublicationPrincipal(ctx context.Context, tx transaction.Transaction, workspaceID, name string) error {
	return accesssqlite.ActivateDashboardPublicationPrincipalTx(ctx, tx, workspaceID, name)
}
