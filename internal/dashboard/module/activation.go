package module

import (
	"context"
	"encoding/json"

	"github.com/Yacobolo/leapview/internal/dashboard/publication"
	publicationsqlite "github.com/Yacobolo/leapview/internal/dashboard/publication/sqlite"
	"github.com/Yacobolo/leapview/internal/platform/transaction"
)

type PublicationActivationInput struct {
	ProjectID, WorkspaceID, ServingStateID, ActorID string
	Publications                                    map[string]json.RawMessage
}

func ReconcilePublications(ctx context.Context, tx transaction.Transaction, input PublicationActivationInput) error {
	publications := make(map[string]publication.Definition, len(input.Publications))
	for name, raw := range input.Publications {
		var definition publication.Definition
		if err := json.Unmarshal(raw, &definition); err != nil {
			return err
		}
		publications[name] = definition
	}
	return publicationsqlite.ReconcileTx(ctx, tx, publication.ReconcileInput{
		ProjectID: input.ProjectID, WorkspaceID: input.WorkspaceID,
		ServingStateID: input.ServingStateID, ActorID: input.ActorID,
		Publications: publications,
	})
}
