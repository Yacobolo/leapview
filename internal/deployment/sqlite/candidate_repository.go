package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flidai/leapview/internal/deployment"
	platformdb "github.com/flidai/leapview/internal/deployment/internal/db"
)

func (r *Repository) StartCandidate(ctx context.Context, candidate deployment.Candidate, maxActivePerOwner int) (deployment.Candidate, bool, error) {
	if r == nil || r.db == nil || maxActivePerOwner <= 0 {
		return deployment.Candidate{}, false, fmt.Errorf("candidate repository and positive quota are required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return deployment.Candidate{}, false, err
	}
	defer tx.Rollback()
	queries := r.q.WithTx(tx)
	now := formatCandidateTime(candidate.CreatedAt)
	if _, err := queries.ExpireProjectCandidates(ctx, platformdb.ExpireProjectCandidatesParams{
		ExpiredAt: nullableCandidateTime(candidate.CreatedAt), UpdatedAt: now,
		TargetID: candidate.TargetID, ExpiresAt: now,
	}); err != nil {
		return deployment.Candidate{}, false, err
	}
	existing, err := queries.GetActiveProjectCandidateSession(ctx, platformdb.GetActiveProjectCandidateSessionParams{
		TargetID: candidate.TargetID, ProjectID: candidate.ProjectID, OwnerPrincipalID: candidate.OwnerID,
	})
	if err == nil {
		mapped, mapErr := mapCandidate(existing)
		if mapErr != nil {
			return deployment.Candidate{}, false, mapErr
		}
		if sameCandidateStart(mapped, candidate) {
			return mapped, true, nil
		}
		return deployment.Candidate{}, false, fmt.Errorf("%w: active candidate must be updated explicitly", deployment.ErrCandidateConflict)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return deployment.Candidate{}, false, err
	}
	count, err := queries.CountActiveProjectCandidatesForOwner(ctx, candidate.OwnerID)
	if err != nil {
		return deployment.Candidate{}, false, err
	}
	if count >= int64(maxActivePerOwner) {
		return deployment.Candidate{}, false, deployment.ErrCandidateQuota
	}
	if err := queries.CreateProjectCandidate(ctx, candidateCreateParams(candidate)); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "constraint") {
			return deployment.Candidate{}, false, fmt.Errorf("%w: active candidate already exists", deployment.ErrCandidateConflict)
		}
		return deployment.Candidate{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return deployment.Candidate{}, false, err
	}
	return candidate, false, nil
}

func (r *Repository) CandidateByID(ctx context.Context, id string) (deployment.Candidate, error) {
	if r == nil || r.q == nil || strings.TrimSpace(id) == "" {
		return deployment.Candidate{}, deployment.ErrCandidateNotFound
	}
	row, err := r.q.GetProjectCandidate(ctx, strings.TrimSpace(id))
	if errors.Is(err, sql.ErrNoRows) {
		return deployment.Candidate{}, deployment.ErrCandidateNotFound
	}
	if err != nil {
		return deployment.Candidate{}, err
	}
	return mapCandidate(row)
}

func (r *Repository) ActiveCandidateBaseGeneration(ctx context.Context, projectID, environment string) (string, error) {
	if r == nil || r.q == nil || strings.TrimSpace(projectID) == "" || strings.TrimSpace(environment) == "" {
		return "", fmt.Errorf("candidate project and environment are required")
	}
	generation, err := r.q.GetActiveProjectCandidateBaseGeneration(ctx, platformdb.GetActiveProjectCandidateBaseGenerationParams{
		ProjectID: strings.TrimSpace(projectID), Environment: strings.TrimSpace(environment),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return deployment.CandidateBaseGenerationEmpty, nil
	}
	if err != nil {
		return "", err
	}
	return generation, nil
}

func (r *Repository) SaveCandidate(ctx context.Context, candidate deployment.Candidate, expectedRevision int64) (deployment.Candidate, error) {
	if r == nil || r.q == nil || candidate.ID == "" || expectedRevision <= 0 || candidate.Revision != expectedRevision+1 {
		return deployment.Candidate{}, fmt.Errorf("%w: invalid candidate revision", deployment.ErrCandidateConflict)
	}
	count, err := r.q.UpdateProjectCandidate(ctx, platformdb.UpdateProjectCandidateParams{
		ArtifactDigest: candidate.ArtifactDigest, Status: string(candidate.Status), FailureReason: candidate.FailureReason,
		ExpiresAt: formatCandidateTime(candidate.ExpiresAt), UpdatedAt: formatCandidateTime(candidate.UpdatedAt),
		ReadyAt: nullableCandidateTime(candidate.ReadyAt), CancelledAt: nullableCandidateTime(candidate.CancelledAt),
		ExpiredAt: nullableCandidateTime(candidate.ExpiredAt), Revision: candidate.Revision,
		ID: candidate.ID, Revision_2: expectedRevision,
	})
	if err != nil {
		return deployment.Candidate{}, err
	}
	if count != 1 {
		return deployment.Candidate{}, deployment.ErrCandidateConflict
	}
	return candidate, nil
}

func (r *Repository) ExpireCandidates(ctx context.Context, targetID string, now time.Time) (int64, error) {
	if r == nil || r.q == nil || strings.TrimSpace(targetID) == "" || now.IsZero() {
		return 0, fmt.Errorf("candidate target and reconciliation time are required")
	}
	value := formatCandidateTime(now)
	return r.q.ExpireProjectCandidates(ctx, platformdb.ExpireProjectCandidatesParams{
		ExpiredAt: nullableCandidateTime(now), UpdatedAt: value, TargetID: strings.TrimSpace(targetID), ExpiresAt: value,
	})
}

func candidateCreateParams(candidate deployment.Candidate) platformdb.CreateProjectCandidateParams {
	return platformdb.CreateProjectCandidateParams{
		ID: candidate.ID, ProjectID: candidate.ProjectID, TargetID: candidate.TargetID,
		Environment: candidate.Environment, OwnerPrincipalID: candidate.OwnerID,
		BaseGeneration: candidate.BaseGeneration, ArtifactDigest: candidate.ArtifactDigest,
		Status: string(candidate.Status), FailureReason: candidate.FailureReason,
		ExpiresAt: formatCandidateTime(candidate.ExpiresAt), CreatedAt: formatCandidateTime(candidate.CreatedAt),
		UpdatedAt: formatCandidateTime(candidate.UpdatedAt), ReadyAt: nullableCandidateTime(candidate.ReadyAt),
		CancelledAt: nullableCandidateTime(candidate.CancelledAt), ExpiredAt: nullableCandidateTime(candidate.ExpiredAt),
		Revision: candidate.Revision,
	}
}

func mapCandidate(row platformdb.ProjectCandidate) (deployment.Candidate, error) {
	expiresAt, err := parseCandidateTime(row.ExpiresAt)
	if err != nil {
		return deployment.Candidate{}, fmt.Errorf("parse candidate expiry: %w", err)
	}
	createdAt, err := parseCandidateTime(row.CreatedAt)
	if err != nil {
		return deployment.Candidate{}, fmt.Errorf("parse candidate creation: %w", err)
	}
	updatedAt, err := parseCandidateTime(row.UpdatedAt)
	if err != nil {
		return deployment.Candidate{}, fmt.Errorf("parse candidate update: %w", err)
	}
	readyAt, err := parseNullableCandidateTime(row.ReadyAt)
	if err != nil {
		return deployment.Candidate{}, fmt.Errorf("parse candidate readiness: %w", err)
	}
	cancelledAt, err := parseNullableCandidateTime(row.CancelledAt)
	if err != nil {
		return deployment.Candidate{}, fmt.Errorf("parse candidate cancellation: %w", err)
	}
	expiredAt, err := parseNullableCandidateTime(row.ExpiredAt)
	if err != nil {
		return deployment.Candidate{}, fmt.Errorf("parse candidate expiration: %w", err)
	}
	return deployment.Candidate{
		ID: row.ID, ProjectID: row.ProjectID, TargetID: row.TargetID, Environment: row.Environment,
		OwnerID: row.OwnerPrincipalID, BaseGeneration: row.BaseGeneration, ArtifactDigest: row.ArtifactDigest,
		Status: deployment.CandidateStatus(row.Status), FailureReason: row.FailureReason,
		ExpiresAt: expiresAt, CreatedAt: createdAt, UpdatedAt: updatedAt, ReadyAt: readyAt,
		CancelledAt: cancelledAt, ExpiredAt: expiredAt, Revision: row.Revision,
	}, nil
}

func sameCandidateStart(existing, candidate deployment.Candidate) bool {
	return existing.ProjectID == candidate.ProjectID && existing.TargetID == candidate.TargetID &&
		existing.Environment == candidate.Environment && existing.OwnerID == candidate.OwnerID &&
		existing.ArtifactDigest == candidate.ArtifactDigest
}

func formatCandidateTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableCandidateTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: formatCandidateTime(value), Valid: true}
}

func parseCandidateTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func parseNullableCandidateTime(value sql.NullString) (time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return time.Time{}, nil
	}
	return parseCandidateTime(value.String)
}
