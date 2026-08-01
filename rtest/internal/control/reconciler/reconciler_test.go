package reconciler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flidai/leapview/rtest/internal/control"
	"github.com/flidai/leapview/rtest/internal/protocol"
)

func TestRunOnceConvergesTerminalOrphanAndMissingJobs(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	finished := now.Add(-time.Hour)
	store := &fakeStore{jobs: map[string]control.Job{
		"job-finished": {ID: "job-finished", ProjectID: "project-one", Status: protocol.StatusRunning},
		"job-live":     {ID: "job-live", ProjectID: "project-two", Status: protocol.StatusRunning},
		"job-missing":  {ID: "job-missing", ProjectID: "project-one", Status: protocol.StatusRunning},
	}}
	scheduler := &fakeScheduler{managed: []protocol.Job{
		{ID: "job-finished", Status: protocol.StatusSucceeded, FinishedAt: &finished, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "job-live", Status: protocol.StatusRunning, CreatedAt: now.Add(-time.Minute)},
		{ID: "orphan", Status: protocol.StatusRunning, CreatedAt: now.Add(-2 * time.Hour)},
	}}

	reconciler := New(Config{Store: store, Scheduler: scheduler, ServiceRetention: 30 * time.Minute, Now: func() time.Time { return now }})
	if err := reconciler.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := store.jobs["job-finished"].Status; got != protocol.StatusSucceeded {
		t.Fatalf("finished status = %s", got)
	}
	if got := store.jobs["job-live"].Status; got != protocol.StatusRunning {
		t.Fatalf("live status = %s", got)
	}
	if got := store.jobs["job-missing"].Status; got != protocol.StatusLost {
		t.Fatalf("missing status = %s", got)
	}
	if len(scheduler.removed) != 2 || scheduler.removed[0] != "job-finished" || scheduler.removed[1] != "orphan" {
		t.Fatalf("removed = %#v", scheduler.removed)
	}
}

type fakeStore struct {
	jobs map[string]control.Job
}

func (f *fakeStore) ScheduledJobs(context.Context) ([]control.Job, error) {
	var jobs []control.Job
	for _, job := range f.jobs {
		if job.Status == protocol.StatusQueued || job.Status == protocol.StatusRunning {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func (f *fakeStore) Job(_ context.Context, id string) (control.Job, error) {
	job, ok := f.jobs[id]
	if !ok {
		return control.Job{}, control.ErrNotFound
	}
	return job, nil
}

func (f *fakeStore) SyncJob(_ context.Context, id string, remote protocol.Job) (control.Job, error) {
	job, ok := f.jobs[id]
	if !ok {
		return control.Job{}, control.ErrNotFound
	}
	job.Status, job.FinishedAt, job.ExitCode, job.ErrorMessage = remote.Status, remote.FinishedAt, remote.ExitCode, remote.ErrorMessage
	f.jobs[id] = job
	return job, nil
}

type fakeScheduler struct {
	managed []protocol.Job
	removed []string
}

func (f *fakeScheduler) ManagedJobs(context.Context) ([]protocol.Job, error) {
	return append([]protocol.Job(nil), f.managed...), nil
}

func (f *fakeScheduler) Remove(_ context.Context, id string) error {
	f.removed = append(f.removed, id)
	return nil
}

func TestRunOnceReturnsDependencyErrors(t *testing.T) {
	want := errors.New("unavailable")
	reconciler := New(Config{Store: errorStore{err: want}, Scheduler: &fakeScheduler{}})
	if err := reconciler.RunOnce(context.Background()); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

type errorStore struct{ err error }

func (e errorStore) ScheduledJobs(context.Context) ([]control.Job, error) { return nil, e.err }
func (e errorStore) Job(context.Context, string) (control.Job, error)     { return control.Job{}, e.err }
func (e errorStore) SyncJob(context.Context, string, protocol.Job) (control.Job, error) {
	return control.Job{}, e.err
}
