package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/deployment"
)

func TestCandidateRepositoryPersistsResumeAndOptimisticReplacementAcrossRestart(t *testing.T) {
	ctx, db, repository := testRepository(t)
	insertCandidatePrincipal(t, ctx, db, "principal_1")
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	firstDigest := "sha256:" + strings.Repeat("a", 64)
	secondDigest := "sha256:" + strings.Repeat("b", 64)
	candidate := candidateRecord(t, now, "cand_1", "finance", "principal_1", firstDigest)

	created, resumed, err := repository.StartCandidate(ctx, candidate, 4)
	if err != nil || resumed {
		t.Fatalf("StartCandidate() = %#v, resumed=%v, err=%v", created, resumed, err)
	}
	restarted := NewRepositoryWithHooks(db, ActivationHooks{})
	replayRequest := candidateRecord(t, now.Add(time.Minute), "cand_other", "finance", "principal_1", firstDigest)
	replayRequest.BaseGeneration = "deployment_advanced_after_candidate_started"
	replayed, resumed, err := restarted.StartCandidate(ctx, replayRequest, 4)
	if err != nil || !resumed || replayed.ID != created.ID {
		t.Fatalf("resumed StartCandidate() = %#v, resumed=%v, err=%v", replayed, resumed, err)
	}
	conflicting := candidateRecord(t, now, "cand_conflict", "finance", "principal_1", secondDigest)
	if _, _, err := restarted.StartCandidate(ctx, conflicting, 4); !errors.Is(err, deployment.ErrCandidateConflict) {
		t.Fatalf("conflicting StartCandidate() error = %v", err)
	}

	next, err := created.ReplaceArtifact(firstDigest, secondDigest, now.Add(time.Minute), now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := restarted.SaveCandidate(ctx, next, created.Revision)
	if err != nil || saved.ArtifactDigest != secondDigest {
		t.Fatalf("SaveCandidate() = %#v, %v", saved, err)
	}
	if _, err := restarted.SaveCandidate(ctx, next, created.Revision); !errors.Is(err, deployment.ErrCandidateConflict) {
		t.Fatalf("stale SaveCandidate() error = %v", err)
	}
}

func TestCandidateRepositoryEnforcesQuotaAndExpiresOnlyMatchingTarget(t *testing.T) {
	ctx, db, repository := testRepository(t)
	insertCandidatePrincipal(t, ctx, db, "principal_1")
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	first := candidateRecord(t, now, "cand_1", "finance", "principal_1", "sha256:"+strings.Repeat("a", 64))
	first.ExpiresAt = now.Add(time.Minute)
	if _, _, err := repository.StartCandidate(ctx, first, 1); err != nil {
		t.Fatal(err)
	}
	second := candidateRecord(t, now, "cand_2", "marketing", "principal_1", "sha256:"+strings.Repeat("b", 64))
	if _, _, err := repository.StartCandidate(ctx, second, 1); !errors.Is(err, deployment.ErrCandidateQuota) {
		t.Fatalf("quota error = %v", err)
	}

	foreign := candidateRecord(t, now, "cand_foreign", "marketing", "principal_1", "sha256:"+strings.Repeat("c", 64))
	foreign.TargetID = "lvinst_other"
	foreign.ExpiresAt = now.Add(time.Minute)
	if _, _, err := repository.StartCandidate(ctx, foreign, 2); err != nil {
		t.Fatal(err)
	}
	count, err := repository.ExpireCandidates(ctx, "lvinst_prod", now.Add(2*time.Minute))
	if err != nil || count != 1 {
		t.Fatalf("ExpireCandidates() = %d, %v", count, err)
	}
	expired, err := repository.CandidateByID(ctx, first.ID)
	if err != nil || expired.Status != deployment.CandidateExpired {
		t.Fatalf("expired candidate = %#v, %v", expired, err)
	}
	unchanged, err := repository.CandidateByID(ctx, foreign.ID)
	if err != nil || unchanged.Status != deployment.CandidatePreparing {
		t.Fatalf("foreign candidate = %#v, %v", unchanged, err)
	}
}

func TestCandidateRepositoryNeverChangesActiveServingState(t *testing.T) {
	ctx, db, repository := testRepository(t)
	insertCandidatePrincipal(t, ctx, db, "principal_1")
	insertWorkspaceCandidate(t, ctx, db, "sales", "sales_old", "sales_new", "prod")
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	candidate := candidateRecord(t, now, "cand_1", "finance", "principal_1", "sha256:"+strings.Repeat("a", 64))
	if _, _, err := repository.StartCandidate(ctx, candidate, 4); err != nil {
		t.Fatal(err)
	}
	cancelled, err := candidate.Cancel(now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCandidate(ctx, cancelled, candidate.Revision); err != nil {
		t.Fatal(err)
	}
	assertActiveState(t, ctx, db, "sales", "prod", "sales_old")
}

func TestCandidateRepositoryRejectsReadyCandidateWithoutReleaseProvenance(t *testing.T) {
	ctx, db, repository := testRepository(t)
	insertCandidatePrincipal(t, ctx, db, "principal_1")
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	candidate := candidateRecord(
		t,
		now,
		"cand_1",
		"finance",
		"principal_1",
		"sha256:"+strings.Repeat("a", 64),
	)
	if _, _, err := repository.StartCandidate(ctx, candidate, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(
		ctx,
		`UPDATE project_candidates
		 SET status = 'ready', ready_at = ?, revision = revision + 1
		 WHERE id = ?`,
		now.Add(time.Minute).Format(time.RFC3339Nano),
		candidate.ID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CandidateByID(
		ctx,
		candidate.ID,
	); !errors.Is(err, deployment.ErrCandidateInvalid) {
		t.Fatalf("CandidateByID() error = %v, want ErrCandidateInvalid", err)
	}
}

func candidateRecord(t *testing.T, now time.Time, id, project, owner, artifactDigest string) deployment.Candidate {
	t.Helper()
	candidate, err := deployment.NewCandidate(deployment.CandidateStartInput{
		ID: id, ProjectID: project, TargetID: "lvinst_prod", Environment: "prod", OwnerID: owner,
		BaseGeneration: "deployment_7", ArtifactDigest: artifactDigest, ExpiresAt: now.Add(time.Hour), Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

func insertCandidatePrincipal(t *testing.T, ctx context.Context, db *sql.DB, id string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO principals (id, email, display_name) VALUES (?, ?, ?)`, id, id+"@example.test", id); err != nil {
		t.Fatal(err)
	}
}
