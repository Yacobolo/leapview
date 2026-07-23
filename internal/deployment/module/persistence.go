package module

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Yacobolo/leapview/internal/deployment"
	deploymentsqlite "github.com/Yacobolo/leapview/internal/deployment/sqlite"
	"github.com/Yacobolo/leapview/internal/platform/transaction"
)

type ActivationHooks struct {
	ApplyAccessSnapshot   func(context.Context, transaction.Transaction, string) error
	ReconcilePublications func(context.Context, transaction.Transaction, PublicationActivationInput) error
}

type PublicationActivationInput struct {
	ProjectID, WorkspaceID, ServingStateID, ActorID string
	Publications                                    map[string]json.RawMessage
}

func newRepository(database *sql.DB, hooks ActivationHooks) deployment.Repository {
	sqliteHooks := deploymentsqlite.ActivationHooks{
		ApplyAccessSnapshot: hooks.ApplyAccessSnapshot,
	}
	if hooks.ReconcilePublications != nil {
		sqliteHooks.ReconcilePublications = func(ctx context.Context, tx transaction.Transaction, input deploymentsqlite.PublicationReconcileInput) error {
			return hooks.ReconcilePublications(ctx, tx, PublicationActivationInput{
				ProjectID: input.ProjectID, WorkspaceID: input.WorkspaceID, ServingStateID: input.ServingStateID,
				ActorID: input.ActorID, Publications: input.Publications,
			})
		}
	}
	return deploymentsqlite.NewRepositoryWithHooks(database, sqliteHooks)
}
