package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/flidai/leapview/rtest/internal/coordinator/sqlite"
	"github.com/flidai/leapview/rtest/internal/protocol"
)

func TestStoreClaimsAndCompletesJobsAtomically(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "rtest.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	created, err := store.CreateJob(ctx, protocol.CreateJob{
		ID: "job-1", Repository: "example/repo", Runner: "standard",
		Command: []string{"go", "test", "./..."}, SourceDigest: "sha256:abc", Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != protocol.StatusQueued {
		t.Fatalf("status = %q", created.Status)
	}

	claimed, ok, err := store.Claim(ctx, "worker-1", time.Now().Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("Claim = %#v, %v, %v", claimed, ok, err)
	}
	if claimed.ID != created.ID || claimed.Status != protocol.StatusRunning || claimed.WorkerID != "worker-1" {
		t.Fatalf("claimed = %#v", claimed)
	}
	if _, ok, err := store.Claim(ctx, "worker-2", time.Now().Add(time.Minute)); err != nil || ok {
		t.Fatalf("second Claim ok=%v err=%v", ok, err)
	}

	if err := store.AppendLog(ctx, created.ID, []byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	data, next, err := store.ReadLog(ctx, created.ID, 0, 1024)
	if err != nil || string(data) != "hello\n" || next != int64(len(data)) {
		t.Fatalf("ReadLog = %q, %d, %v", data, next, err)
	}

	exit := 0
	if err := store.Finish(ctx, created.ID, protocol.StatusSucceeded, &exit, ""); err != nil {
		t.Fatal(err)
	}
	got, err := store.Job(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != protocol.StatusSucceeded || got.ExitCode == nil || *got.ExitCode != 0 || got.FinishedAt == nil {
		t.Fatalf("finished job = %#v", got)
	}
}

func TestStoreCancellationIsVisibleToWorker(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "rtest.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.CreateJob(ctx, protocol.CreateJob{
		ID: "job-2", Repository: "example/repo", Runner: "standard",
		Command: []string{"sleep", "30"}, SourceDigest: "sha256:def", Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Claim(ctx, "worker-1", time.Now().Add(time.Minute)); err != nil || !ok {
		t.Fatalf("claim job: ok=%v err=%v", ok, err)
	}
	if err := store.RequestCancel(ctx, "job-2"); err != nil {
		t.Fatal(err)
	}
	job, err := store.Job(ctx, "job-2")
	if err != nil {
		t.Fatal(err)
	}
	if !job.CancelRequested {
		t.Fatalf("job = %#v", job)
	}
}

func TestStoreCancellingQueuedJobMakesItTerminal(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "rtest.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	job, err := store.CreateJob(ctx, protocol.CreateJob{
		ID: "queued-cancel", Repository: "example/repo", Runner: "standard",
		Command: []string{"true"}, SourceDigest: "sha256:abc", Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.RequestCancel(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	got, err := store.Job(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != protocol.StatusCancelled || got.FinishedAt == nil || !got.CancelRequested {
		t.Fatalf("queued cancellation = %#v", got)
	}
}

func TestStoreListsNewestJobsAndFiltersRepository(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "rtest.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, input := range []struct {
		id, repository string
	}{
		{"job-1", "example/one"},
		{"job-2", "example/two"},
		{"job-3", "example/one"},
	} {
		if _, err := store.CreateJob(ctx, protocol.CreateJob{
			ID: input.id, Repository: input.repository, Runner: "standard",
			Command: []string{"true"}, SourceDigest: "sha256:abc", Timeout: time.Minute,
		}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}

	jobs, err := store.ListJobs(ctx, "example/one", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 || jobs[0].ID != "job-3" || jobs[1].ID != "job-1" {
		t.Fatalf("jobs = %#v", jobs)
	}
}

func TestStoreMarksExpiredRunningJobsLost(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "rtest.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	job, err := store.CreateJob(ctx, protocol.CreateJob{
		ID: "expired", Repository: "example/repo", Runner: "standard",
		Command: []string{"sleep", "30"}, SourceDigest: "sha256:abc", Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Claim(ctx, "worker-1", time.Now().Add(-time.Second)); err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}

	count, err := store.MarkExpiredLeases(ctx, time.Now())
	if err != nil || count != 1 {
		t.Fatalf("MarkExpiredLeases = %d, %v", count, err)
	}
	got, err := store.Job(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != protocol.StatusLost || got.FinishedAt == nil || got.LeaseExpiresAt != nil {
		t.Fatalf("expired job = %#v", got)
	}
}
