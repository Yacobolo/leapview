package sqlite

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Yacobolo/leapview/internal/platform"
	refreshrun "github.com/Yacobolo/leapview/internal/refresh/run"
)

func TestSQLRunRepositoryAcceptsActiveLeaseFence(t *testing.T) {
	store, repo, job := seedRefreshJob(t, refreshrun.RunStatusRunning, "+5 minutes")

	if err := repo.RenewJobLease(t.Context(), job, time.Minute); err != nil {
		t.Fatalf("RenewJobLease() error = %v", err)
	}
	if _, err := repo.MarkRunPrepared(t.Context(), job); err != nil {
		t.Fatalf("MarkRunPrepared() error = %v", err)
	}
	allowed, err := repo.RunMayPublish(t.Context(), job)
	if err != nil {
		t.Fatalf("RunMayPublish() error = %v", err)
	}
	if !allowed {
		t.Fatal("RunMayPublish() = false for active lease")
	}
	assertRefreshStatuses(t, store, refreshrun.RunStatusRunning, "prepared")
}

func TestSQLRunRepositoryRejectsExpiredLeaseFence(t *testing.T) {
	t.Run("renew", func(t *testing.T) {
		store, repo, job := seedExpiredRefreshJob(t, refreshrun.RunStatusRunning)

		err := repo.RenewJobLease(t.Context(), job, time.Minute)
		if !errors.Is(err, refreshrun.ErrLeaseLost) {
			t.Fatalf("RenewJobLease() error = %v, want ErrLeaseLost", err)
		}
		assertRefreshStatuses(t, store, refreshrun.RunStatusRunning, refreshrun.RunStatusRunning)
	})

	t.Run("prepare", func(t *testing.T) {
		store, repo, job := seedExpiredRefreshJob(t, refreshrun.RunStatusRunning)

		_, err := repo.MarkRunPrepared(t.Context(), job)
		if !errors.Is(err, refreshrun.ErrLeaseLost) {
			t.Fatalf("MarkRunPrepared() error = %v, want ErrLeaseLost", err)
		}
		assertRefreshStatuses(t, store, refreshrun.RunStatusRunning, refreshrun.RunStatusRunning)
	})

	t.Run("publish eligibility", func(t *testing.T) {
		store, repo, job := seedExpiredRefreshJob(t, "prepared")

		allowed, err := repo.RunMayPublish(t.Context(), job)
		if err != nil {
			t.Fatalf("RunMayPublish() error = %v", err)
		}
		if allowed {
			t.Fatal("RunMayPublish() = true for expired lease")
		}
		assertRefreshStatuses(t, store, refreshrun.RunStatusRunning, "prepared")
	})
}

func seedExpiredRefreshJob(t *testing.T, runStatus string) (*platform.Store, *SQLRunRepository, refreshrun.JobRecord) {
	t.Helper()
	return seedRefreshJob(t, runStatus, "-1 second")
}

func seedRefreshJob(t *testing.T, runStatus, leaseOffset string) (*platform.Store, *SQLRunRepository, refreshrun.JobRecord) {
	t.Helper()
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "platform.db"))
	if err != nil {
		t.Fatalf("open platform store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.SQLDB().ExecContext(t.Context(), `
INSERT INTO workspaces (id, title) VALUES ('sales', 'Sales');
INSERT INTO refresh_jobs (
  id, workspace_id, model_id, kind, status, lease_owner, lease_generation
) VALUES (
  'job_1', 'sales', 'sales', 'refresh_pipeline', 'running', 'worker-1', 1
);
INSERT INTO refresh_job_runs (
  id, job_id, environment, target_type, target_id, target_generation, trigger_type, status, created_sequence
) VALUES (
  'run_1', 'job_1', 'dev', 'refresh_pipeline', 'sales.daily', 1, 'manual', ?, 1
);`, runStatus); err != nil {
		t.Fatalf("seed refresh job: %v", err)
	}
	if _, err := store.SQLDB().ExecContext(t.Context(),
		`UPDATE refresh_jobs SET lease_expires_at = datetime('now', ?) WHERE id = 'job_1'`,
		leaseOffset,
	); err != nil {
		t.Fatalf("set refresh job lease: %v", err)
	}
	job := refreshrun.JobRecord{
		ID: "job_1", WorkspaceID: "sales", Environment: "dev", ModelID: "sales",
		Kind: refreshrun.JobKindRefreshPipeline, RunID: "run_1",
		TargetType: refreshrun.TargetRefreshPipeline, TargetID: "sales.daily", TargetGeneration: 1,
		TriggerType: refreshrun.TriggerManual, LeaseOwner: "worker-1", LeaseGeneration: 1,
	}
	return store, NewSQLRunRepository(store.SQLDB()), job
}

func assertRefreshStatuses(t *testing.T, store *platform.Store, wantJob, wantRun string) {
	t.Helper()
	var jobStatus, runStatus string
	if err := store.SQLDB().QueryRowContext(t.Context(), `
SELECT j.status, r.status
FROM refresh_jobs j
JOIN refresh_job_runs r ON r.job_id = j.id
WHERE j.id = 'job_1'`).Scan(&jobStatus, &runStatus); err != nil {
		t.Fatalf("read refresh statuses: %v", err)
	}
	if jobStatus != wantJob || runStatus != wantRun {
		t.Fatalf("refresh statuses = %q/%q, want %q/%q", jobStatus, runStatus, wantJob, wantRun)
	}
}
