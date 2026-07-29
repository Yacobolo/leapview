package connectionbinding

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPoolManagerActivatesValidatedReplacementAndDrainsPreviousLeases(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	resolver := &sequenceResolver{snapshots: []CredentialSnapshot{
		testSnapshot(t, "version-1", now),
		testSnapshot(t, "version-2", now.Add(time.Minute)),
	}}
	factory := &recordingPoolFactory{}
	store := &recordingBindingStore{}
	manager, err := NewPoolManager(PoolManagerConfig{
		Binding: validTargetBinding(t), Resolver: resolver, Factory: factory, Store: store,
		Now: func() time.Time { return now }, StaleAfter: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	firstLease, err := manager.Lease()
	if err != nil {
		t.Fatal(err)
	}
	first := firstLease.Pool().(*recordingRuntimePool)

	now = now.Add(time.Minute)
	if err := manager.RefreshNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	secondLease, err := manager.Lease()
	if err != nil {
		t.Fatal(err)
	}
	second := secondLease.Pool().(*recordingRuntimePool)
	if first == second || first.closed {
		t.Fatalf("first=%p second=%p first.closed=%t", first, second, first.closed)
	}
	if got := manager.Evidence().ValidatedVersion; got != "version-2" {
		t.Fatalf("validated version = %q", got)
	}
	secondLease.Release()
	if first.closed {
		t.Fatal("previous pool closed before its outstanding lease drained")
	}
	firstLease.Release()
	if !first.closed || second.closed {
		t.Fatalf("first.closed=%t second.closed=%t", first.closed, second.closed)
	}
}

func TestPoolManagerKeepsHealthyPoolWhenNewVersionFailsValidation(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	resolver := &sequenceResolver{snapshots: []CredentialSnapshot{
		testSnapshot(t, "version-1", now),
		testSnapshot(t, "version-bad", now.Add(time.Minute)),
	}}
	factory := &recordingPoolFactory{healthFailures: map[string]error{"version-bad": errors.New("source-secret-must-not-leak")}}
	store := &recordingBindingStore{}
	manager, err := NewPoolManager(PoolManagerConfig{
		Binding: validTargetBinding(t), Resolver: resolver, Factory: factory, Store: store,
		Now: func() time.Time { return now }, StaleAfter: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	healthy, err := manager.Lease()
	if err != nil {
		t.Fatal(err)
	}
	active := healthy.Pool()
	healthy.Release()

	now = now.Add(time.Minute)
	err = manager.RefreshNow(context.Background())
	if !errors.Is(err, ErrInvalidCredentialBundle) || containsSecret(err) {
		t.Fatalf("RefreshNow() error = %v", err)
	}
	current, err := manager.Lease()
	if err != nil {
		t.Fatal(err)
	}
	defer current.Release()
	if current.Pool() != active || manager.Evidence().Health != HealthDegraded ||
		manager.Evidence().ValidatedVersion != "version-1" {
		t.Fatalf("active replaced after invalid rotation: evidence=%#v", manager.Evidence())
	}
	if !factory.pools[1].closed {
		t.Fatal("invalid replacement pool was not closed")
	}
}

func TestPoolManagerCoalescesConcurrentRefreshAndDisableFailsClosed(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	resolver := &sequenceResolver{snapshots: []CredentialSnapshot{testSnapshot(t, "version-1", now)}}
	resolver.delay = 20 * time.Millisecond
	factory := &recordingPoolFactory{}
	manager, err := NewPoolManager(PoolManagerConfig{
		Binding: validTargetBinding(t), Resolver: resolver, Factory: factory, Store: &recordingBindingStore{},
		Now: func() time.Time { return now }, StaleAfter: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errs <- manager.RefreshNow(context.Background())
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if resolver.calls != 1 || len(factory.pools) != 1 {
		t.Fatalf("resolver calls=%d pools=%d", resolver.calls, len(factory.pools))
	}
	lease, err := manager.Lease()
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Disable(context.Background(), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Lease(); !errors.Is(err, ErrDisabledBinding) {
		t.Fatalf("Lease() after disable error = %v", err)
	}
	active := lease.Pool().(*recordingRuntimePool)
	if active.closed {
		t.Fatal("disabled pool closed before outstanding lease drained")
	}
	lease.Release()
	if !active.closed {
		t.Fatal("disabled pool did not drain")
	}
}

func TestPoolManagerRetainsValidatedPoolOnlyWithinStalePolicyDuringProviderOutage(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	resolver := &sequenceResolver{
		snapshots: []CredentialSnapshot{testSnapshot(t, "version-1", now)},
		errs:      []error{nil, ErrProviderUnavailable},
	}
	manager, err := NewPoolManager(PoolManagerConfig{
		Binding: validTargetBinding(t), Resolver: resolver, Factory: &recordingPoolFactory{}, Store: &recordingBindingStore{},
		Now: func() time.Time { return now }, StaleAfter: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := manager.RefreshNow(context.Background()); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("provider outage error = %v", err)
	}
	if manager.Evidence().Health != HealthDegraded || manager.Evidence().ValidatedVersion != "version-1" {
		t.Fatalf("outage evidence = %#v", manager.Evidence())
	}
	lease, err := manager.Lease()
	if err != nil {
		t.Fatalf("lease inside stale policy: %v", err)
	}
	lease.Release()
	now = now.Add(5 * time.Minute)
	if _, err := manager.Lease(); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("lease outside stale policy error = %v", err)
	}
}

func testSnapshot(t *testing.T, version string, now time.Time) CredentialSnapshot {
	t.Helper()
	snapshot, err := NewCredentialSnapshot(map[string]string{"connection_string": "source-secret"}, version, now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

type sequenceResolver struct {
	mu        sync.Mutex
	snapshots []CredentialSnapshot
	errs      []error
	calls     int
	delay     time.Duration
}

func (resolver *sequenceResolver) Resolve(context.Context, CredentialReference) (CredentialSnapshot, error) {
	if resolver.delay > 0 {
		time.Sleep(resolver.delay)
	}
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	resolver.calls++
	index := resolver.calls - 1
	if index < len(resolver.errs) && resolver.errs[index] != nil {
		return CredentialSnapshot{}, resolver.errs[index]
	}
	if index >= len(resolver.snapshots) {
		index = len(resolver.snapshots) - 1
	}
	return resolver.snapshots[index], nil
}

type recordingPoolFactory struct {
	mu             sync.Mutex
	pools          []*recordingRuntimePool
	healthFailures map[string]error
}

func (factory *recordingPoolFactory) Prepare(_ context.Context, _ TargetBinding, snapshot CredentialSnapshot) (RuntimePool, error) {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	pool := &recordingRuntimePool{version: snapshot.ProviderVersion(), healthError: factory.healthFailures[snapshot.ProviderVersion()]}
	factory.pools = append(factory.pools, pool)
	return pool, nil
}

type recordingRuntimePool struct {
	version     string
	healthError error
	closed      bool
}

func (pool *recordingRuntimePool) HealthCheck(context.Context) error { return pool.healthError }
func (pool *recordingRuntimePool) Close() error {
	pool.closed = true
	return nil
}

type recordingBindingStore struct {
	mu      sync.Mutex
	binding TargetBinding
}

func (store *recordingBindingStore) Save(_ context.Context, binding TargetBinding, expectedRevision int64) (TargetBinding, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if binding.Revision != expectedRevision+1 {
		return TargetBinding{}, ErrIncompatibleBinding
	}
	store.binding = binding
	return binding, nil
}

func containsSecret(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "source-secret") || strings.Contains(err.Error(), "connection_string"))
}
