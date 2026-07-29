package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yacobolo/leapview/internal/access"
	accessdb "github.com/Yacobolo/leapview/internal/access/internal/db"
	"github.com/Yacobolo/leapview/internal/platform/transaction"
)

// ActivateDashboardPublicationPrincipalTx installs the Access-owned identity
// for an anonymous dashboard publication inside the caller's transaction.
func ActivateDashboardPublicationPrincipalTx(ctx context.Context, tx transaction.Transaction, workspaceID, name string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	name = strings.TrimSpace(name)
	if workspaceID == "" || name == "" {
		return fmt.Errorf("dashboard publication principal requires workspace and name")
	}
	return accessdb.New(tx).UpsertPrincipal(ctx, accessdb.UpsertPrincipalParams{
		ID:          access.DashboardPublicationSubjectID(workspaceID, name),
		Kind:        string(access.PrincipalKindDashboardPublication),
		DisplayName: name,
	})
}
