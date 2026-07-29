package connectionbinding

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestPoolDirectoryCreatesOneManagerPerBindingRevision(t *testing.T) {
	binding := validTargetBinding(t)
	var builds int
	var managers []*PoolManager
	directory, err := NewPoolDirectory(PoolDirectoryConfig{
		Build: func(current TargetBinding) (*PoolManager, error) {
			builds++
			manager, err := NewPoolManager(PoolManagerConfig{
				Binding: current,
				Resolver: &sequenceResolver{
					snapshots: []CredentialSnapshot{testSnapshot(t, "version-1", current.UpdatedAt)},
				},
				Factory:    &recordingPoolFactory{},
				Store:      &recordingBindingStore{},
				Now:        func() time.Time { return current.UpdatedAt },
				StaleAfter: time.Hour,
			})
			if err == nil {
				managers = append(managers, manager)
			}
			return manager, err
		},
		RefreshTimeout: time.Second,
		MaxConcurrent:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = directory.Close() })

	first, err := directory.Pool(binding)
	if err != nil {
		t.Fatal(err)
	}
	same, err := directory.Pool(binding)
	if err != nil {
		t.Fatal(err)
	}
	if first != same || builds != 1 {
		t.Fatalf("same revision returned pools %p and %p after %d builds", first, same, builds)
	}

	updated, err := binding.UpdateConfiguration(TargetBindingConfiguration{
		ConnectorKind: binding.ConnectorKind, AuthenticationMode: binding.AuthenticationMode,
		Endpoint:            EndpointConfig{Host: "warehouse-next.internal", Port: binding.Endpoint.Port},
		CredentialReference: binding.CredentialReference,
	}, binding.UpdatedAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := directory.Pool(updated)
	if err != nil {
		t.Fatal(err)
	}
	if replacement == first || builds != 2 {
		t.Fatalf("new revision returned pool %p after %d builds; old=%p", replacement, builds, first)
	}
	if _, err := managers[0].Lease(); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("retired manager lease error = %v", err)
	}
}

func TestPoolDirectoryBoundsRefreshConcurrencyAndTimeout(t *testing.T) {
	binding := validTargetBinding(t)
	second := binding
	second.ID = "binding_reporting"
	second.LogicalConnectionID = "reporting"
	resolvers := map[string]*blockingResolver{}
	directory, err := NewPoolDirectory(PoolDirectoryConfig{
		Build: func(current TargetBinding) (*PoolManager, error) {
			resolver := &blockingResolver{started: make(chan struct{}), release: make(chan struct{})}
			resolvers[current.ID] = resolver
			return NewPoolManager(PoolManagerConfig{
				Binding: current, Resolver: resolver, Factory: &recordingPoolFactory{},
				Store: &recordingBindingStore{}, Now: time.Now, StaleAfter: time.Hour,
			})
		},
		RefreshTimeout: 40 * time.Millisecond,
		MaxConcurrent:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	first, err := directory.Pool(binding)
	if err != nil {
		t.Fatal(err)
	}
	other, err := directory.Pool(second)
	if err != nil {
		t.Fatal(err)
	}

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- first.Refresh(context.Background(), RefreshRequest{
			Actor: "principal:operator-1", Operation: RefreshTest,
		})
	}()
	<-resolvers[binding.ID].started

	start := time.Now()
	err = other.Refresh(context.Background(), RefreshRequest{
		Actor: "principal:operator-1", Operation: RefreshTest,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued refresh error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("bounded refresh took %s", elapsed)
	}
	if resolvers[second.ID].calls != 0 {
		t.Fatalf("queued binding resolver calls = %d", resolvers[second.ID].calls)
	}
	if err := <-firstDone; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("active refresh error = %v", err)
	}
}

func TestPoolDirectoryCloseRetiresManagersAndRejectsNewPools(t *testing.T) {
	binding := validTargetBinding(t)
	factory := &recordingPoolFactory{}
	directory, err := NewPoolDirectory(PoolDirectoryConfig{
		Build: func(current TargetBinding) (*PoolManager, error) {
			return NewPoolManager(PoolManagerConfig{
				Binding: current,
				Resolver: &sequenceResolver{
					snapshots: []CredentialSnapshot{testSnapshot(t, "version-1", current.UpdatedAt)},
				},
				Factory: factory, Store: &recordingBindingStore{},
				Now: func() time.Time { return current.UpdatedAt }, StaleAfter: time.Hour,
			})
		},
		RefreshTimeout: time.Second,
		MaxConcurrent:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := directory.Pool(binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Refresh(context.Background(), RefreshRequest{
		Actor: "principal:operator-1", Operation: RefreshTest,
	}); err != nil {
		t.Fatal(err)
	}
	if err := directory.Close(); err != nil {
		t.Fatal(err)
	}
	if len(factory.pools) != 1 || !factory.pools[0].closed {
		t.Fatalf("closed pools = %#v", factory.pools)
	}
	if _, err := directory.Pool(binding); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("Pool() after Close error = %v", err)
	}
}

type blockingResolver struct {
	once    sync.Once
	started chan struct{}
	release chan struct{}
	calls   int
}

func (resolver *blockingResolver) Resolve(ctx context.Context, _ CredentialReference) (CredentialSnapshot, error) {
	resolver.calls++
	resolver.once.Do(func() { close(resolver.started) })
	select {
	case <-ctx.Done():
		return CredentialSnapshot{}, ctx.Err()
	case <-resolver.release:
		return CredentialSnapshot{}, ErrProviderUnavailable
	}
}
