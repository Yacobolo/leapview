package duckdb

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Yacobolo/leapview/internal/workload"
)

type fakeEngine struct {
	id     string
	closed atomic.Bool
}

func (e *fakeEngine) Close() error { e.closed.Store(true); return nil }

func TestEnginePoolIsLazyAndPrepareValidates(t *testing.T) {
	pool := mustEnginePool(t, 2)
	var opens atomic.Int32
	if opens.Load() != 0 {
		t.Fatal("engine opened before candidate preparation")
	}
	ctx := admitted(t, "a")
	_, err := pool.Prepare(ctx, Descriptor{WorkspaceID: "a", RuntimeID: "one", RequireAdmission: true, Open: func(context.Context, EngineBudget) (Engine, error) { opens.Add(1); return &fakeEngine{id: "one"}, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if opens.Load() != 1 || pool.Stats().Idle != 1 {
		t.Fatalf("opens=%d stats=%#v", opens.Load(), pool.Stats())
	}
}

func TestEnginePoolPrepareRejectsInitializationFailureWithoutPublishingHandle(t *testing.T) {
	pool := mustEnginePool(t, 2)
	want := errors.New("initialize engine")
	if _, err := pool.Prepare(admitted(t, "a"), Descriptor{WorkspaceID: "a", RuntimeID: "broken", RequireAdmission: true, Open: func(context.Context, EngineBudget) (Engine, error) {
		return nil, want
	}}); !errors.Is(err, want) {
		t.Fatalf("Prepare error = %v, want %v", err, want)
	}
	stats := pool.Stats()
	if stats.Open != 0 || len(stats.Workspaces) != 0 {
		t.Fatalf("stats after failed prepare = %#v", stats)
	}
}

func TestEnginePoolDividesFixedNodeBudgetsAcrossCapacity(t *testing.T) {
	tempRoot := t.TempDir()
	pool, err := NewEnginePool(EnginePoolConfig{MaxOpen: 2, NodeMemoryBytes: 1000, NodeTempBytes: 2000, NodeThreads: 6, TempRoot: tempRoot})
	if err != nil {
		t.Fatal(err)
	}
	var got EngineBudget
	_, err = pool.Prepare(admitted(t, "a"), Descriptor{WorkspaceID: "a", RuntimeID: "one", RequireAdmission: true, Open: func(_ context.Context, budget EngineBudget) (Engine, error) {
		got = budget
		return &fakeEngine{id: "one"}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.MemoryBytes != 500 || got.TempBytes != 1000 || got.Threads != 3 || got.TempDir == "" {
		t.Fatalf("engine budget = %#v", got)
	}
}

func TestEnginePoolEvictsOnlyIdleInLRUOrder(t *testing.T) {
	pool := mustEnginePool(t, 2)
	a, aEngine := fakeHandle(t, pool, "a", "a")
	b, bEngine := fakeHandle(t, pool, "b", "b")
	c, _ := fakeHandle(t, pool, "c", "c")
	prepare(t, a, "a")
	prepare(t, b, "b")
	lease, err := a.Acquire(admitted(t, "a"))
	if err != nil {
		t.Fatal(err)
	}
	prepare(t, c, "c")
	if aEngine.closed.Load() {
		t.Fatal("active engine was evicted")
	}
	if !bEngine.closed.Load() {
		t.Fatal("oldest idle engine was not evicted")
	}
	lease.Release()
	if pool.Stats().Open != 2 {
		t.Fatalf("open = %d", pool.Stats().Open)
	}
}

func TestEnginePoolRejectsUnadmittedAndConflictingNestedAccess(t *testing.T) {
	pool := mustEnginePool(t, 2)
	a, _ := fakeHandle(t, pool, "a", "a")
	b, _ := fakeHandle(t, pool, "b", "b")
	if _, err := a.Acquire(context.Background()); !errors.Is(err, ErrUnadmitted) {
		t.Fatalf("error = %v", err)
	}
	outer, err := a.Acquire(admitted(t, "a"))
	if err != nil {
		t.Fatal(err)
	}
	nested, err := a.Acquire(outer.Context())
	if err != nil {
		t.Fatal(err)
	}
	nested.Release()
	if _, err := b.Acquire(outer.Context()); !errors.Is(err, ErrConflictingNested) {
		t.Fatalf("error = %v", err)
	}
	outer.Release()
}

func TestEnginePoolDoesNotWaitWhenAllSlotsAreActive(t *testing.T) {
	pool := mustEnginePool(t, 1)
	a, _ := fakeHandle(t, pool, "a", "a")
	b, _ := fakeHandle(t, pool, "b", "b")
	lease, err := a.Acquire(admitted(t, "a"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Acquire(admitted(t, "b")); !errors.Is(err, ErrCapacityInvariant) {
		t.Fatalf("error = %v", err)
	}
	lease.Release()
}

func TestEnginePoolConcurrentReuseAndRetirement(t *testing.T) {
	pool := mustEnginePool(t, 2)
	handle, engine := fakeHandle(t, pool, "a", "one")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, err := handle.Acquire(admitted(t, "a"))
			if err != nil {
				t.Error(err)
				return
			}
			_ = pool.Stats()
			lease.Release()
		}()
	}
	wg.Wait()
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	if !engine.closed.Load() || pool.Stats().Open != 0 {
		t.Fatalf("closed=%v stats=%#v", engine.closed.Load(), pool.Stats())
	}
}

func mustEnginePool(t *testing.T, maxOpen int) *EnginePool {
	t.Helper()
	pool, err := NewEnginePool(EnginePoolConfig{MaxOpen: maxOpen, NodeMemoryBytes: int64(maxOpen) * (512 << 20), NodeTempBytes: int64(maxOpen) * (2 << 30), NodeThreads: maxOpen, TempRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return pool
}
func fakeHandle(t *testing.T, p *EnginePool, workspace, id string) (*Handle, *fakeEngine) {
	t.Helper()
	engine := &fakeEngine{id: id}
	handle, err := p.register(Descriptor{WorkspaceID: workspace, RuntimeID: id, RequireAdmission: true, Open: func(context.Context, EngineBudget) (Engine, error) { return engine, nil }})
	if err != nil {
		t.Fatal(err)
	}
	return handle, engine
}
func prepare(t *testing.T, h *Handle, workspace string) {
	t.Helper()
	lease, err := h.Acquire(admitted(t, workspace))
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
}
func admitted(t *testing.T, workspace string) context.Context {
	t.Helper()
	controller, err := workload.New(workload.Config{MaxRunning: 1, MaximumQueued: 1, Classes: map[workload.Class]workload.Policy{workload.Interactive: {MaximumRunning: 1, MaximumQueued: 1, MaximumQueuedPerWorkspace: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := controller.Acquire(context.Background(), workload.Request{Class: workload.Interactive, WorkspaceID: workspace, Operation: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lease.Release(); controller.Close() })
	return lease.Context()
}
