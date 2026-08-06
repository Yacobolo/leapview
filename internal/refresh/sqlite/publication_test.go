package sqlite

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/platform"
	refreshrun "github.com/flidai/leapview/internal/refresh/run"
	refreshschedule "github.com/flidai/leapview/internal/refresh/schedule"
	servingstate "github.com/flidai/leapview/internal/servingstate"
)

func TestPublicationCompletesRootAndChildrenAtomically(t *testing.T) {
	store, version := seedPublicationTree(t, "running")
	defer store.Close()
	unit := NewPublicationUnitOfWork(store.SQLDB(), nil)
	if err := unit.Publish(t.Context(), servingstate.WorkspaceID("sales"), servingstate.Environment("dev"), servingstate.ID("candidate"), version); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	assertPublicationTreeStatuses(t, store, refreshrun.RunStatusSucceeded, refreshrun.RunStatusSucceeded, refreshrun.RunStatusSucceeded, refreshrun.RunStatusSucceeded)
}

func TestPublicationRejectsExpiredFenceWithoutMutation(t *testing.T) {
	store, version := seedPublicationTree(t, "running")
	defer store.Close()
	if _, err := store.SQLDB().ExecContext(t.Context(), `UPDATE refresh_jobs SET lease_expires_at = datetime('now', '-1 second') WHERE id = 'root_job'`); err != nil {
		t.Fatal(err)
	}
	before := publicationSnapshot(t, store)
	unit := NewPublicationUnitOfWork(store.SQLDB(), nil)
	if err := unit.Publish(t.Context(), "sales", "dev", "candidate", version); !errors.Is(err, refreshrun.ErrLeaseLost) {
		t.Fatalf("Publish() error = %v, want ErrLeaseLost", err)
	}
	assertPublicationTreeStatuses(t, store, refreshrun.RunStatusPrepared, refreshrun.RunStatusRunning, refreshrun.RunStatusRunning, refreshrun.RunStatusQueued)
	if after := publicationSnapshot(t, store); after != before {
		t.Fatalf("expired publication mutated durable fields: before=%q after=%q", before, after)
	}
}

func publicationSnapshot(t *testing.T, store *platform.Store) string {
	t.Helper()
	var snapshot string
	err := store.SQLDB().QueryRowContext(t.Context(), `SELECT group_concat(value, '|') FROM (SELECT printf('%s|%s|%s|%s|%d|%s|%s', j.status, r.status, r.error, COALESCE(r.finished_at,''), j.lease_generation, COALESCE(j.lease_owner,''), COALESCE(j.lease_expires_at,'')) AS value FROM refresh_jobs j JOIN refresh_job_runs r ON r.job_id=j.id WHERE j.id IN ('root_job','child_job') ORDER BY j.id)`).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestPublicationIneligibleChildRollsBackWholeTree(t *testing.T) {
	store, version := seedPublicationTree(t, "succeeded")
	defer store.Close()
	unit := NewPublicationUnitOfWork(store.SQLDB(), nil)
	if err := unit.Publish(t.Context(), "sales", "dev", "candidate", version); !errors.Is(err, refreshrun.ErrLeaseLost) {
		t.Fatalf("Publish() error = %v, want ErrLeaseLost", err)
	}
	assertPublicationTreeStatuses(t, store, refreshrun.RunStatusPrepared, refreshrun.RunStatusSucceeded, refreshrun.RunStatusRunning, refreshrun.RunStatusQueued)
}

func seedPublicationTree(t *testing.T, childStatus string) (*platform.Store, refreshschedule.DataVersion) {
	t.Helper()
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "platform.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SQLDB().ExecContext(t.Context(), `
INSERT INTO workspaces (id, title) VALUES ('sales', 'Sales');
INSERT INTO serving_states (id, workspace_id, environment, status, source, digest, manifest_json, created_by, ducklake_snapshot_id)
VALUES ('candidate', 'sales', 'dev', 'validated', 'refresh', 'digest', '{}', 'test', 42);
INSERT INTO refresh_jobs (id, workspace_id, serving_state_id, model_id, kind, status, lease_owner, lease_generation, lease_expires_at)
VALUES ('root_job', 'sales', 'candidate', 'sales', 'refresh_pipeline', 'running', 'worker-1', 1, datetime('now', '+5 minutes'));
INSERT INTO refresh_jobs (id, workspace_id, serving_state_id, model_id, kind, status)
VALUES ('child_job', 'sales', 'candidate', 'sales', 'child_run', 'queued');
INSERT INTO refresh_job_runs (id, job_id, environment, target_type, target_id, target_generation, trigger_type, status, created_sequence)
VALUES ('root_run', 'root_job', 'dev', 'refresh_pipeline', 'sales.daily', 1, 'manual', 'prepared', 1);
INSERT INTO refresh_job_runs (id, job_id, environment, target_type, target_id, target_generation, trigger_type, parent_run_id, status, created_sequence)
VALUES ('child_run', 'child_job', 'dev', 'model_table', 'sales.orders', 1, 'dependency', 'root_run', ?, 2);`, childStatus); err != nil {
		t.Fatal(err)
	}
	version := refreshschedule.DataVersion{WorkspaceID: "sales", Environment: "dev", SemanticModel: "sales", SnapshotID: 42, ServingStateID: "candidate", RefreshedAt: time.Now().UTC(), Source: refreshschedule.DataVersionSourceRefresh, PipelineID: "daily", RunID: "root_run", TargetGeneration: 1, LeaseOwner: "worker-1", LeaseGeneration: 1}
	return store, version
}

func assertPublicationTreeStatuses(t *testing.T, store *platform.Store, wantRoot, wantChild, wantRootJob, wantChildJob string) {
	t.Helper()
	var root, child, rootJob, childJob string
	if err := store.SQLDB().QueryRowContext(t.Context(), `SELECT status FROM refresh_job_runs WHERE id='root_run'`).Scan(&root); err != nil {
		t.Fatal(err)
	}
	if err := store.SQLDB().QueryRowContext(t.Context(), `SELECT status FROM refresh_job_runs WHERE id='child_run'`).Scan(&child); err != nil {
		t.Fatal(err)
	}
	if err := store.SQLDB().QueryRowContext(t.Context(), `SELECT status FROM refresh_jobs WHERE id='root_job'`).Scan(&rootJob); err != nil {
		t.Fatal(err)
	}
	if err := store.SQLDB().QueryRowContext(t.Context(), `SELECT status FROM refresh_jobs WHERE id='child_job'`).Scan(&childJob); err != nil {
		t.Fatal(err)
	}
	if root != wantRoot || child != wantChild || rootJob != wantRootJob || childJob != wantChildJob {
		t.Fatalf("tree statuses = %q/%q jobs %q/%q, want %q/%q", root, child, rootJob, childJob, wantRoot, wantChild)
	}
}
