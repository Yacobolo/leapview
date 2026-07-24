package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Yacobolo/leapview/internal/platform"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
)

func TestRepositoryPersistsClaimsReclaimsAndOrderedEvents(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "platform.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := NewRepository(store.SQLDB())

	created, err := repo.Enqueue(t.Context(), jobs.EnqueueInput{
		ID: "job-1", Kind: "release.finalize", WorkloadClass: "control", WorkspaceID: "_node", ResourceKind: "release", ResourceID: "release-1", Payload: []byte(`{"project":"project-a"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != jobs.StatusQueued {
		t.Fatalf("status = %q", created.Status)
	}

	candidates, err := repo.Candidates(t.Context(), "control", 16)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates = %#v, %v", candidates, err)
	}
	claimed, ok, err := repo.ClaimByID(t.Context(), candidates[0].ID, "control", "worker-a", time.Minute)
	if err != nil || !ok || claimed.ID != created.ID || claimed.Attempts != 1 || claimed.LeaseGeneration != 1 {
		t.Fatalf("claim = %#v, %v, %v", claimed, ok, err)
	}
	if _, ok, err := repo.ClaimByID(t.Context(), created.ID, "control", "worker-b", time.Minute); err != nil || ok {
		t.Fatalf("leased job claimed twice: ok=%v err=%v", ok, err)
	}
	if _, err := store.SQLDB().ExecContext(t.Context(), `UPDATE api_async_jobs SET lease_expires_at = datetime('now', '-1 second') WHERE id = ?`, created.ID); err != nil {
		t.Fatal(err)
	}
	// Reusing the same owner proves that the monotonically increasing
	// generation, rather than owner identity alone, fences a stale claim.
	reclaimed, ok, err := repo.ClaimByID(t.Context(), created.ID, "control", "worker-a", time.Minute)
	if err != nil || !ok || reclaimed.Attempts != 2 || reclaimed.LeaseGeneration != 2 {
		t.Fatalf("reclaim = %#v, %v, %v", reclaimed, ok, err)
	}
	if err := repo.Renew(t.Context(), claimed.ID, claimed.Fence(), time.Minute); !errors.Is(err, jobs.ErrConflict) {
		t.Fatalf("stale renewal error = %v, want conflict", err)
	}
	if err := repo.Complete(t.Context(), claimed.ID, claimed.Fence()); !errors.Is(err, jobs.ErrConflict) {
		t.Fatalf("stale completion error = %v, want conflict", err)
	}

	for _, eventType := range []string{"release.validating", "release.ready"} {
		if _, err := repo.AppendEvent(t.Context(), "release", "release-1", eventType, []byte(`{"status":"ok"}`)); err != nil {
			t.Fatal(err)
		}
	}
	events, err := repo.ListEvents(t.Context(), "release", "release-1", 0, 50)
	if err != nil || len(events) != 2 || events[0].ID != 1 || events[1].ID != 2 {
		t.Fatalf("events = %#v, err=%v", events, err)
	}

	if err := repo.Complete(t.Context(), reclaimed.ID, reclaimed.Fence()); err != nil {
		t.Fatal(err)
	}
	finished, err := repo.Get(t.Context(), reclaimed.ID)
	if err != nil || finished.Status != jobs.StatusSucceeded || finished.FinishedAt == "" {
		t.Fatalf("finished = %#v, err=%v", finished, err)
	}
}

func TestRepositoryRejectsIdempotentJobIDWithDifferentPayload(t *testing.T) {
	store, err := platform.Open(context.Background(), filepath.Join(t.TempDir(), "platform.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := NewRepository(store.SQLDB())
	input := jobs.EnqueueInput{ID: "job-1", Kind: "release.finalize", WorkloadClass: "control", WorkspaceID: "_node", ResourceKind: "release", ResourceID: "release-1", Payload: []byte(`{"a":1}`)}
	if _, err := repo.Enqueue(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	input.Payload = []byte(`{"a":2}`)
	if _, err := repo.Enqueue(t.Context(), input); err == nil {
		t.Fatal("different payload reused the same durable job ID")
	}
}

func TestRepositoryRejectsWorkerMutationsAfterLeaseExpiry(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(context.Context, *Repository, jobs.Job) error
	}{
		{
			name: "renew",
			mutate: func(ctx context.Context, repo *Repository, job jobs.Job) error {
				return repo.Renew(ctx, job.ID, job.Fence(), time.Minute)
			},
		},
		{
			name: "complete",
			mutate: func(ctx context.Context, repo *Repository, job jobs.Job) error {
				return repo.Complete(ctx, job.ID, job.Fence())
			},
		},
		{
			name: "fail",
			mutate: func(ctx context.Context, repo *Repository, job jobs.Job) error {
				return repo.Fail(ctx, job.ID, job.Fence(), []byte(`{"code":"FAILED"}`))
			},
		},
		{
			name: "cancel claimed",
			mutate: func(ctx context.Context, repo *Repository, job jobs.Job) error {
				return repo.CancelClaimed(ctx, job.ID, job.Fence())
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, repo, claimed := claimAsyncJob(t)
			if _, err := store.SQLDB().ExecContext(t.Context(),
				`UPDATE api_async_jobs SET lease_expires_at = datetime('now', '-1 second') WHERE id = ?`,
				claimed.ID,
			); err != nil {
				t.Fatal(err)
			}

			if err := test.mutate(t.Context(), repo, claimed); !errors.Is(err, jobs.ErrConflict) {
				t.Fatalf("expired %s error = %v, want ErrConflict", test.name, err)
			}
			stored, err := repo.Get(t.Context(), claimed.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Status != jobs.StatusRunning || stored.FinishedAt != "" {
				t.Fatalf("job after expired %s = %#v, want recoverable running claim", test.name, stored)
			}
		})
	}
}

func claimAsyncJob(t *testing.T) (*platform.Store, *Repository, jobs.Job) {
	t.Helper()
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "platform.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := NewRepository(store.SQLDB())
	created, err := repo.Enqueue(t.Context(), jobs.EnqueueInput{
		ID: "job-1", Kind: "release.finalize", WorkloadClass: "control", WorkspaceID: "_node",
		ResourceKind: "release", ResourceID: "release-1", Payload: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := repo.ClaimByID(t.Context(), created.ID, "control", "worker-a", time.Minute)
	if err != nil || !ok {
		t.Fatalf("ClaimByID() = %#v, %v, %v", claimed, ok, err)
	}
	return store, repo, claimed
}

func TestRepositoryListsOnlyEachWorkspacesDurableHead(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "platform.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := NewRepository(store.SQLDB())
	for _, input := range []jobs.EnqueueInput{
		{ID: "a-1", Kind: "agent.run", WorkloadClass: "background", WorkspaceID: "a", ResourceKind: "agent", ResourceID: "a-1", Payload: []byte(`{}`)},
		{ID: "a-2", Kind: "agent.run", WorkloadClass: "background", WorkspaceID: "a", ResourceKind: "agent", ResourceID: "a-2", Payload: []byte(`{}`)},
		{ID: "b-1", Kind: "agent.run", WorkloadClass: "background", WorkspaceID: "b", ResourceKind: "agent", ResourceID: "b-1", Payload: []byte(`{}`)},
	} {
		if _, err := repo.Enqueue(t.Context(), input); err != nil {
			t.Fatal(err)
		}
	}
	candidates, err := repo.Candidates(t.Context(), "background", 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %#v", candidates)
	}
	ids := map[string]bool{}
	for _, candidate := range candidates {
		ids[candidate.ID] = true
	}
	if !ids["a-1"] || !ids["b-1"] || ids["a-2"] {
		t.Fatalf("candidate heads = %#v", ids)
	}
}

func TestRepositoryAppendsConcurrentEventsWithContiguousResourceSequence(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "platform.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := NewRepository(store.SQLDB())

	const count = 20
	errors := make(chan error, count)
	var group sync.WaitGroup
	for index := 0; index < count; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, appendErr := repo.AppendEvent(t.Context(), "deployment", "deploy-1", "deployment.progress", []byte(`{"status":"running"}`))
			errors <- appendErr
		}()
	}
	group.Wait()
	close(errors)
	for appendErr := range errors {
		if appendErr != nil {
			t.Fatalf("append concurrent event: %v", appendErr)
		}
	}
	events, err := repo.ListEvents(t.Context(), "deployment", "deploy-1", 0, count)
	if err != nil || len(events) != count {
		t.Fatalf("events=%d err=%v", len(events), err)
	}
	for index, event := range events {
		if event.ID != int64(index+1) {
			t.Fatalf("event %d ID=%d", index, event.ID)
		}
	}
}
