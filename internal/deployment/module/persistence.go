package module

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Yacobolo/leapview/internal/deployment"
	deploymentsqlite "github.com/Yacobolo/leapview/internal/deployment/sqlite"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
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

func newPersistence(database *sql.DB, hooks ActivationHooks, releases ReleasePort, workflow jobs.WorkflowRecorder) (deployment.Repository, deployment.ActivationUnitOfWork) {
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
	if releases != nil {
		sqliteHooks.LinkRelease = func(ctx context.Context, tx transaction.Transaction, input deployment.CreateInput) error {
			return releases.LinkDeploymentTx(ctx, tx, input.ProjectID, input.ID, input.ReleaseID, input.RollbackOf)
		}
	}
	sqliteHooks.RecordWorkflow = workflow
	owned := deploymentsqlite.NewRepositoryWithHooks(database, sqliteHooks)
	return owned, owned
}
