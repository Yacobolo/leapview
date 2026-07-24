package module

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Yacobolo/leapview/internal/platform"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
	"github.com/Yacobolo/leapview/internal/workload"
)

func testAdmission(controller workload.Admitter) jobs.Admitter {
	return jobs.AdmitterFunc(func(ctx context.Context, request jobs.AdmissionRequest) (jobs.AdmissionLease, error) {
		return controller.Acquire(ctx, workload.Request{
			Class: workload.Class(request.Class), WorkspaceID: request.WorkspaceID, Operation: request.Operation,
		})
	})
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
	module, err := Build(t.Context(), Config{Database: store.SQLDB(), Admission: testAdmission(admission)})
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
	module, err := Build(t.Context(), Config{Database: store.SQLDB(), Admission: testAdmission(admission)})
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
	module, err := Build(t.Context(), Config{Database: store.SQLDB(), Admission: testAdmission(admission)})
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
