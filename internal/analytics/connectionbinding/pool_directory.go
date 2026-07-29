package connectionbinding

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type PoolManagerBuilder func(TargetBinding) (*PoolManager, error)

type PoolDirectoryConfig struct {
	Build          PoolManagerBuilder
	RefreshTimeout time.Duration
	MaxConcurrent  int
}

// PoolDirectory owns the live pool-manager generation for each target binding.
// A binding revision change atomically retires the old manager so new work
// cannot lease a generation prepared from stale endpoint or credential metadata.
type PoolDirectory struct {
	mu             sync.Mutex
	build          PoolManagerBuilder
	refreshTimeout time.Duration
	refreshGate    chan struct{}
	pools          map[string]*boundedAdministrationPool
	closed         bool
}

func NewPoolDirectory(config PoolDirectoryConfig) (*PoolDirectory, error) {
	if config.Build == nil || config.RefreshTimeout <= 0 || config.MaxConcurrent <= 0 {
		return nil, fmt.Errorf(
			"%w: pool builder, refresh timeout, and positive concurrency limit are required",
			ErrInvalidBinding,
		)
	}
	return &PoolDirectory{
		build: config.Build, refreshTimeout: config.RefreshTimeout,
		refreshGate: make(chan struct{}, config.MaxConcurrent),
		pools:       map[string]*boundedAdministrationPool{},
	}, nil
}

func (directory *PoolDirectory) Pool(binding TargetBinding) (AdministrationPool, error) {
	if directory == nil {
		return nil, ErrProviderUnavailable
	}
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	directory.mu.Lock()
	defer directory.mu.Unlock()
	if directory.closed {
		return nil, ErrProviderUnavailable
	}
	if current := directory.pools[binding.ID]; current != nil {
		evidence := current.manager.Evidence()
		if evidence.BindingRevision == binding.Revision {
			return current, nil
		}
		if err := current.manager.Retire(); err != nil {
			return nil, err
		}
		delete(directory.pools, binding.ID)
	}
	manager, err := directory.build(binding)
	if err != nil {
		return nil, err
	}
	if manager == nil {
		return nil, ErrProviderUnavailable
	}
	pool := &boundedAdministrationPool{
		manager: manager, timeout: directory.refreshTimeout, gate: directory.refreshGate,
	}
	directory.pools[binding.ID] = pool
	return pool, nil
}

func (directory *PoolDirectory) Close() error {
	if directory == nil {
		return nil
	}
	directory.mu.Lock()
	if directory.closed {
		directory.mu.Unlock()
		return nil
	}
	directory.closed = true
	pools := make([]*boundedAdministrationPool, 0, len(directory.pools))
	for _, pool := range directory.pools {
		pools = append(pools, pool)
	}
	clear(directory.pools)
	directory.mu.Unlock()

	var errs []error
	for _, pool := range pools {
		errs = append(errs, pool.manager.Retire())
	}
	return errors.Join(errs...)
}

type boundedAdministrationPool struct {
	manager *PoolManager
	timeout time.Duration
	gate    chan struct{}
}

func (pool *boundedAdministrationPool) Refresh(ctx context.Context, request RefreshRequest) error {
	if pool == nil || pool.manager == nil {
		return ErrProviderUnavailable
	}
	bounded, cancel := context.WithTimeout(ctx, pool.timeout)
	defer cancel()
	select {
	case pool.gate <- struct{}{}:
		defer func() { <-pool.gate }()
	case <-bounded.Done():
		return bounded.Err()
	}
	return pool.manager.Refresh(bounded, request)
}

func (pool *boundedAdministrationPool) Disable(ctx context.Context, now time.Time) error {
	if pool == nil || pool.manager == nil {
		return ErrProviderUnavailable
	}
	return pool.manager.Disable(ctx, now)
}

func (pool *boundedAdministrationPool) HealthStatus() BindingHealthStatus {
	if pool == nil || pool.manager == nil {
		return BindingHealthStatus{}
	}
	return pool.manager.HealthStatus()
}
