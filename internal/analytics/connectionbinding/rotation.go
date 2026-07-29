package connectionbinding

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type RuntimePool interface {
	HealthCheck(context.Context) error
	Close() error
}

type RuntimePoolFactory interface {
	Prepare(context.Context, TargetBinding, CredentialSnapshot) (RuntimePool, error)
}

type BindingStateStore interface {
	Save(context.Context, TargetBinding, int64) (TargetBinding, error)
}

type PoolManagerConfig struct {
	Binding    TargetBinding
	Resolver   CredentialResolver
	Factory    RuntimePoolFactory
	Store      BindingStateStore
	Now        func() time.Time
	StaleAfter time.Duration
}

type PoolManager struct {
	resolver CredentialResolver
	factory  RuntimePoolFactory
	store    BindingStateStore
	now      func() time.Time
	stale    time.Duration

	refreshGroup singleflight.Group
	refreshMu    sync.Mutex
	mu           sync.Mutex
	binding      TargetBinding
	active       *poolGeneration
	lastRun      time.Time
}

type poolGeneration struct {
	pool     RuntimePool
	version  string
	leases   int
	draining bool
}

func NewPoolManager(config PoolManagerConfig) (*PoolManager, error) {
	if err := config.Binding.Validate(); err != nil {
		return nil, err
	}
	if config.Resolver == nil || config.Factory == nil || config.Store == nil || config.Now == nil || config.StaleAfter <= 0 {
		return nil, fmt.Errorf("%w: resolver, pool factory, binding store, clock, and stale policy are required", ErrInvalidBinding)
	}
	return &PoolManager{
		resolver: config.Resolver, factory: config.Factory, store: config.Store,
		now: config.Now, stale: config.StaleAfter, binding: config.Binding,
	}, nil
}

func (manager *PoolManager) RefreshNow(ctx context.Context) error {
	if manager == nil {
		return ErrProviderUnavailable
	}
	_, err, _ := manager.refreshGroup.Do("refresh", func() (any, error) {
		return nil, manager.refresh(ctx)
	})
	return err
}

func (manager *PoolManager) refresh(ctx context.Context) error {
	manager.refreshMu.Lock()
	defer manager.refreshMu.Unlock()
	now := manager.now().UTC()

	manager.mu.Lock()
	if !manager.binding.Enabled {
		manager.mu.Unlock()
		return ErrDisabledBinding
	}
	binding := manager.binding
	manager.mu.Unlock()

	snapshot, err := manager.resolver.Resolve(ctx, binding.CredentialReference)
	if err != nil {
		manager.recordRefresh(now)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return manager.degrade(ctx, providerFailureReason(err), now, err)
	}
	defer snapshot.Destroy()
	version := snapshot.ProviderVersion()

	manager.mu.Lock()
	if manager.active != nil && manager.active.version == version {
		manager.mu.Unlock()
		validated, err := binding.MarkValidated(version, now)
		if err != nil {
			return err
		}
		if validated.Revision != binding.Revision {
			saved, err := manager.store.Save(ctx, validated, binding.Revision)
			if err != nil {
				return err
			}
			manager.mu.Lock()
			if manager.binding.Revision == binding.Revision {
				manager.binding = saved
			}
			manager.lastRun = now
			manager.mu.Unlock()
		} else {
			manager.recordRefresh(now)
		}
		return nil
	}
	manager.mu.Unlock()

	replacement, err := manager.factory.Prepare(ctx, binding, snapshot)
	if err != nil {
		manager.recordRefresh(now)
		return manager.degrade(ctx, "POOL_PREPARE_FAILED", now, ErrInvalidCredentialBundle)
	}
	if replacement == nil {
		manager.recordRefresh(now)
		return manager.degrade(ctx, "POOL_PREPARE_FAILED", now, ErrInvalidCredentialBundle)
	}
	if err := replacement.HealthCheck(ctx); err != nil {
		_ = replacement.Close()
		manager.recordRefresh(now)
		return manager.degrade(ctx, "POOL_HEALTH_CHECK_FAILED", now, ErrInvalidCredentialBundle)
	}

	validated, err := binding.MarkValidated(version, now)
	if err != nil {
		_ = replacement.Close()
		return err
	}
	saved, err := manager.store.Save(ctx, validated, binding.Revision)
	if err != nil {
		_ = replacement.Close()
		return err
	}
	manager.mu.Lock()
	if !manager.binding.Enabled || manager.binding.Revision != binding.Revision {
		manager.mu.Unlock()
		_ = replacement.Close()
		return ErrIncompatibleBinding
	}
	previous := manager.active
	manager.binding = saved
	manager.active = &poolGeneration{pool: replacement, version: version}
	manager.lastRun = now
	closePrevious := markDraining(previous)
	manager.mu.Unlock()
	if closePrevious != nil {
		_ = closePrevious.Close()
	}
	return nil
}

func (manager *PoolManager) recordRefresh(now time.Time) {
	manager.mu.Lock()
	manager.lastRun = now
	manager.mu.Unlock()
}

func (manager *PoolManager) degrade(ctx context.Context, reason string, now time.Time, result error) error {
	manager.mu.Lock()
	binding := manager.binding
	manager.mu.Unlock()
	degraded, err := binding.MarkDegraded(reason, now)
	if err != nil {
		return result
	}
	saved, err := manager.store.Save(ctx, degraded, binding.Revision)
	if err != nil {
		return errors.Join(result, err)
	}
	manager.mu.Lock()
	if manager.binding.Revision == binding.Revision {
		manager.binding = saved
	}
	manager.mu.Unlock()
	return result
}

func (manager *PoolManager) Lease() (*PoolLease, error) {
	if manager == nil {
		return nil, ErrProviderUnavailable
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if !manager.binding.Enabled {
		return nil, ErrDisabledBinding
	}
	if manager.active == nil {
		return nil, ErrCredentialNotFound
	}
	if manager.binding.Health == HealthDegraded && manager.now().UTC().Sub(manager.binding.LastValidatedAt) > manager.stale {
		return nil, ErrProviderUnavailable
	}
	manager.active.leases++
	return &PoolLease{manager: manager, generation: manager.active}, nil
}

func (manager *PoolManager) Disable(ctx context.Context, now time.Time) error {
	if manager == nil {
		return ErrProviderUnavailable
	}
	manager.refreshMu.Lock()
	defer manager.refreshMu.Unlock()
	manager.mu.Lock()
	binding := manager.binding
	manager.mu.Unlock()
	disabled, err := binding.Disable(now)
	if err != nil {
		return err
	}
	if disabled.Revision == binding.Revision {
		return nil
	}
	saved, err := manager.store.Save(ctx, disabled, binding.Revision)
	if err != nil {
		return err
	}
	manager.mu.Lock()
	manager.binding = saved
	previous := manager.active
	manager.active = nil
	closePrevious := markDraining(previous)
	manager.mu.Unlock()
	if closePrevious != nil {
		return closePrevious.Close()
	}
	return nil
}

func (manager *PoolManager) Evidence() BindingEvidence {
	if manager == nil {
		return BindingEvidence{}
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.binding.Evidence()
}

type PoolLease struct {
	once       sync.Once
	manager    *PoolManager
	generation *poolGeneration
}

func (lease *PoolLease) Pool() RuntimePool {
	if lease == nil || lease.generation == nil {
		return nil
	}
	return lease.generation.pool
}

func (lease *PoolLease) Release() {
	if lease == nil {
		return
	}
	lease.once.Do(func() {
		manager := lease.manager
		manager.mu.Lock()
		generation := lease.generation
		if generation.leases > 0 {
			generation.leases--
		}
		var closing RuntimePool
		if generation.draining && generation.leases == 0 {
			closing = generation.pool
		}
		manager.mu.Unlock()
		if closing != nil {
			_ = closing.Close()
		}
	})
}

func markDraining(generation *poolGeneration) RuntimePool {
	if generation == nil {
		return nil
	}
	generation.draining = true
	if generation.leases == 0 {
		return generation.pool
	}
	return nil
}

func providerFailureReason(err error) string {
	switch {
	case errors.Is(err, ErrCredentialDenied):
		return "PROVIDER_ACCESS_DENIED"
	case errors.Is(err, ErrCredentialNotFound):
		return "PROVIDER_SECRET_NOT_FOUND"
	case errors.Is(err, ErrCredentialRateLimited):
		return "PROVIDER_RATE_LIMITED"
	default:
		return "PROVIDER_UNAVAILABLE"
	}
}
