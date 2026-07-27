package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Yacobolo/leapview/internal/platform/transaction"
	refreshrun "github.com/Yacobolo/leapview/internal/refresh/run"
	refreshschedule "github.com/Yacobolo/leapview/internal/refresh/schedule"
	"github.com/Yacobolo/leapview/internal/refresh/sqlite/materializedb"
	servingstate "github.com/Yacobolo/leapview/internal/servingstate"
)

// PublicationUnitOfWork owns the fenced cross-table transaction that makes a
// prepared refresh candidate visible and completes its durable work.
type PublicationUnitOfWork struct {
	db                  *sql.DB
	applyAccessSnapshot func(context.Context, transaction.Transaction, string) error
}

func NewPublicationUnitOfWork(database *sql.DB, applyAccessSnapshot func(context.Context, transaction.Transaction, string) error) *PublicationUnitOfWork {
	return &PublicationUnitOfWork{db: database, applyAccessSnapshot: applyAccessSnapshot}
}

func (u *PublicationUnitOfWork) Publish(ctx context.Context, workspaceID servingstate.WorkspaceID, environment servingstate.Environment, servingStateID servingstate.ID, version refreshschedule.DataVersion) error {
	if u == nil || u.db == nil {
		return fmt.Errorf("refresh publication database is required")
	}
	environment = servingstate.NormalizeEnvironment(environment)
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := materializedb.New(tx)
	active, err := q.RefreshPublicationFenceActive(ctx, materializedb.RefreshPublicationFenceActiveParams{
		RunID: version.RunID, TargetGeneration: version.TargetGeneration,
		LeaseOwner: version.LeaseOwner, LeaseGeneration: version.LeaseGeneration,
	})
	if err != nil {
		return err
	}
	if active != 1 {
		return refreshrun.ErrLeaseLost
	}
	candidate, err := q.RefreshPublicationCandidate(ctx, string(servingStateID))
	if err != nil {
		return err
	}
	if candidate.WorkspaceID != string(workspaceID) {
		return fmt.Errorf("serving state %s is not in workspace %s", servingStateID, workspaceID)
	}
	if candidate.Environment != string(environment) {
		return fmt.Errorf("serving state %s environment = %q, want %q", servingStateID, candidate.Environment, environment)
	}
	status := servingstate.Status(candidate.Status)
	if status != servingstate.StatusValidated && status != servingstate.StatusInactive && status != servingstate.StatusActive {
		return fmt.Errorf("serving state %s has status %q, want validated", servingStateID, status)
	}
	if err := validatePublicationVersion(candidate, version); err != nil {
		return err
	}
	if u.applyAccessSnapshot != nil {
		if err := u.applyAccessSnapshot(ctx, tx, string(servingStateID)); err != nil {
			return err
		}
	}
	if err := q.DrainOtherRefreshServingStates(ctx, materializedb.DrainOtherRefreshServingStatesParams{
		WorkspaceID: string(workspaceID), Environment: string(environment), ServingStateID: string(servingStateID),
	}); err != nil {
		return err
	}
	if err := q.ActivateRefreshServingState(ctx, string(servingStateID)); err != nil {
		return err
	}
	if err := q.SetRefreshActiveServingState(ctx, materializedb.SetRefreshActiveServingStateParams{
		WorkspaceID: string(workspaceID), Environment: string(environment), ServingStateID: string(servingStateID),
	}); err != nil {
		return err
	}
	if err := q.AdvanceRefreshSemanticModelDataVersions(ctx, materializedb.AdvanceRefreshSemanticModelDataVersionsParams{
		SnapshotID: version.SnapshotID, ServingStateID: version.ServingStateID, WorkspaceID: version.WorkspaceID,
		Environment: version.Environment, SemanticModelID: version.SemanticModel,
	}); err != nil {
		return err
	}
	if err := q.UpsertRefreshPublicationDataVersion(ctx, materializedb.UpsertRefreshPublicationDataVersionParams{
		WorkspaceID: version.WorkspaceID, Environment: version.Environment, SemanticModelID: version.SemanticModel,
		SnapshotID: version.SnapshotID, ServingStateID: version.ServingStateID,
		RefreshedAt: version.RefreshedAt.UTC().Format(time.RFC3339Nano), PipelineID: version.PipelineID, RunID: version.RunID,
	}); err != nil {
		return err
	}
	completed, err := q.CompleteRefreshPublicationRun(ctx, materializedb.CompleteRefreshPublicationRunParams{
		RunID: version.RunID, TargetGeneration: version.TargetGeneration,
		LeaseOwner: version.LeaseOwner, LeaseGeneration: version.LeaseGeneration,
	})
	if err != nil {
		return err
	}
	if completed != 1 {
		return refreshrun.ErrLeaseLost
	}
	completed, err = q.CompleteRefreshPublicationJob(ctx, materializedb.CompleteRefreshPublicationJobParams{
		RunID: version.RunID, LeaseOwner: version.LeaseOwner, LeaseGeneration: version.LeaseGeneration,
	})
	if err != nil {
		return err
	}
	if completed != 1 {
		return refreshrun.ErrLeaseLost
	}
	return tx.Commit()
}

func validatePublicationVersion(candidate materializedb.RefreshPublicationCandidateRow, version refreshschedule.DataVersion) error {
	if candidate.DucklakeSnapshotID <= 0 || version.SemanticModel == "" || version.RefreshedAt.IsZero() ||
		version.WorkspaceID != candidate.WorkspaceID || version.Environment != candidate.Environment ||
		version.SnapshotID != candidate.DucklakeSnapshotID || version.ServingStateID != candidate.ID ||
		version.Source != refreshschedule.DataVersionSourceRefresh || version.TargetGeneration <= 0 ||
		strings.TrimSpace(version.LeaseOwner) == "" || version.LeaseGeneration <= 0 {
		return fmt.Errorf("refresh publication requires a matching semantic-model data version")
	}
	return nil
}
