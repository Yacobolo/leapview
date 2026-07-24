package module

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Yacobolo/leapview/internal/platform"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
	"github.com/Yacobolo/leapview/internal/workload"
)

func TestModuleRestartRecoversInterruptedClaim(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	admission, err := workload.New(workload.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer admission.Close()

	first, err := Build(t.Context(), Config{
		Database: store.SQLDB(), Admission: admission,
		LeaseTimeout: time.Minute, PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	if err := first.RegisterHandlers([]jobs.Handler{jobs.HandlerFunc{
		JobKind: "release.finalize",
		Run: func(ctx context.Context, _ jobs.Job) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := first.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Enqueue(t.Context(), jobs.EnqueueInput{
		ID: "release:one:finalize", Kind: "release.finalize", WorkloadClass: "control", WorkspaceID: "_node",
		ResourceKind: "release", ResourceID: "one", Payload: []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first worker did not claim the job")
	}
	stopContext, cancelStop := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancelStop()
	if err := first.Stop(stopContext); err != nil {
		t.Fatalf("stop first module: %v", err)
	}
	interrupted, err := first.Get(t.Context(), "release:one:finalize")
	if err != nil {
		t.Fatal(err)
	}
	if interrupted.Status != jobs.StatusRunning || interrupted.FinishedAt != "" {
		t.Fatalf("interrupted job = %#v, want recoverable running claim", interrupted)
	}
	if _, err := store.SQLDB().ExecContext(t.Context(),
		`UPDATE api_async_jobs SET lease_expires_at = datetime('now', '-1 second') WHERE id = ?`,
		interrupted.ID,
	); err != nil {
		t.Fatal(err)
	}

	second, err := Build(t.Context(), Config{
		Database: store.SQLDB(), Admission: admission,
		LeaseTimeout: time.Minute, PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	handled := make(chan struct{})
	if err := second.RegisterHandlers([]jobs.Handler{jobs.HandlerFunc{
		JobKind: "release.finalize",
		Run: func(context.Context, jobs.Job) error {
			close(handled)
			return nil
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := second.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer second.Stop(context.Background())
	select {
	case <-handled:
	case <-time.After(2 * time.Second):
		t.Fatal("replacement worker did not reclaim the interrupted job")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		finished, getErr := second.Get(t.Context(), interrupted.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if finished.Status == jobs.StatusSucceeded {
			if finished.Attempts != 2 {
				t.Fatalf("recovered job attempts = %d, want 2", finished.Attempts)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("recovered job status = %q, want succeeded", finished.Status)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestModuleRejectsDuplicateKindsBeforeStarting(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	admission, err := workload.New(workload.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer admission.Close()
	module, err := Build(t.Context(), Config{Database: store.SQLDB(), Admission: admission})
	if err != nil {
		t.Fatal(err)
	}
	handler := jobs.HandlerFunc{JobKind: "duplicate", Run: func(context.Context, jobs.Job) error { return nil }}
	if err := module.RegisterHandlers([]jobs.Handler{handler, handler}); err == nil {
		t.Fatal("duplicate handler kinds were accepted")
	}
}

func TestModuleLifecycleIsIdempotent(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	admission, err := workload.New(workload.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer admission.Close()
	module, err := Build(t.Context(), Config{Database: store.SQLDB(), Admission: admission})
	if err != nil {
		t.Fatal(err)
	}
	if err := module.RegisterHandlers(nil); err != nil {
		t.Fatal(err)
	}
	if err := module.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := module.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := module.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := module.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestModuleRejectsUnknownEnqueuedKind(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	admission, err := workload.New(workload.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer admission.Close()
	module, err := Build(t.Context(), Config{Database: store.SQLDB(), Admission: admission})
	if err != nil {
		t.Fatal(err)
	}
	handler := jobs.HandlerFunc{JobKind: "known", Run: func(context.Context, jobs.Job) error { return nil }}
	if err := module.RegisterHandlers([]jobs.Handler{handler}); err != nil {
		t.Fatal(err)
	}
	_, err = module.Enqueue(t.Context(), jobs.EnqueueInput{
		ID: "unknown-1", Kind: "unknown", WorkloadClass: "control", WorkspaceID: "_node",
		ResourceKind: "test", ResourceID: "unknown-1", Payload: []byte(`{}`),
	})
	if !errors.Is(err, jobs.ErrUnknownKind) {
		t.Fatalf("Enqueue() error = %v, want ErrUnknownKind", err)
	}
}
