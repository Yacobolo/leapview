package duckdb

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Yacobolo/leapview/internal/securefs"
	"github.com/Yacobolo/leapview/internal/workload"
)

var (
	ErrUnadmitted        = errors.New("DuckDB engine access requires workload admission")
	ErrConflictingNested = errors.New("conflicting nested DuckDB engine acquisition")
	ErrCapacityInvariant = errors.New("DuckDB engine capacity exhausted after workload admission")
	ErrEnginePoolClosed  = errors.New("DuckDB engine pool is closed")
)

type Engine interface{ Close() error }
type EngineBudget struct {
	MemoryBytes, TempBytes int64
	Threads                int
	TempDir                string
}
type Descriptor struct {
	WorkspaceID, RuntimeID string
	RequireAdmission       bool
	Open                   func(context.Context, EngineBudget) (Engine, error)
}
type EnginePoolConfig struct {
	MaxOpen                        int
	NodeMemoryBytes, NodeTempBytes int64
	NodeThreads                    int
	TempRoot                       string
}

func (c EnginePoolConfig) Validate() error {
	if c.MaxOpen <= 0 {
		return fmt.Errorf("DuckDB maximum open engines must be positive")
	}
	if c.NodeMemoryBytes < int64(c.MaxOpen) || c.NodeTempBytes < int64(c.MaxOpen) || c.NodeThreads < c.MaxOpen {
		return fmt.Errorf("DuckDB node budgets must provide positive capacity for every open engine")
	}
	if c.TempRoot == "" {
		return fmt.Errorf("DuckDB temporary root is required")
	}
	return nil
}

type EnginePool struct {
	mu                               sync.Mutex
	config                           EnginePoolConfig
	budget                           EngineBudget
	closed                           bool
	handles                          map[string]*Handle
	engines                          map[string]*engineState
	idle                             *list.List
	open, active                     int
	opens, reuse, evictions          uint64
	initializationFailures, cleanups uint64
	cleanupFailures                  uint64
	exhaustions                      map[string]uint64
	acquisitionDuration              durationHistogram
	resultRows, resultBytes          valueHistogram
}
type Handle struct {
	pool       *EnginePool
	descriptor Descriptor
	key        string
	closed     bool
}
type engineState struct {
	handle      *Handle
	engine      Engine
	refs        int
	idleElement *list.Element
	tempDir     string
	opening     bool
	ready       chan struct{}
	openErr     error
}
type engineContext struct {
	handle *Handle
	engine Engine
}
type engineContextKey struct{}
type EngineLease interface {
	Engine() Engine
	Context() context.Context
	Release()
}
type engineLease struct {
	pool  *EnginePool
	state *engineState
	ctx   context.Context
	once  sync.Once
}
type nestedEngineLease struct {
	engine Engine
	ctx    context.Context
}

type WorkspaceEngineStats struct{ Open, Active, Idle int }
type EnginePoolStats struct {
	Open, Active, Idle, Maximum                    int
	Opens, Reuses, Evictions                       uint64
	InitializationFailures, Cleanups, CleanupFails uint64
	Workspaces                                     map[string]WorkspaceEngineStats
	Exhaustions                                    map[string]uint64
	AcquisitionDuration                            HistogramSnapshot
	ResultRows, ResultBytes                        HistogramSnapshot
}

type HistogramSnapshot struct {
	Count   uint64
	Sum     float64
	Buckets map[float64]uint64
}

type durationHistogram struct {
	count   uint64
	sum     float64
	buckets [8]uint64
}
type valueHistogram struct {
	count   uint64
	sum     float64
	buckets [8]uint64
}

var durationBounds = [...]float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5}
var rowBounds = [...]float64{1, 10, 100, 1000, 10000, 50000, 100000, 1000000}
var byteBounds = [...]float64{1024, 16 << 10, 256 << 10, 1 << 20, 8 << 20, 32 << 20, 128 << 20, 512 << 20}

func NewEnginePool(config EnginePoolConfig) (*EnginePool, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if err := securefs.EnsurePrivateDir(config.TempRoot); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(config.TempRoot)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if err := os.RemoveAll(filepath.Join(config.TempRoot, entry.Name())); err != nil {
				return nil, fmt.Errorf("remove stale DuckDB temporary directory: %w", err)
			}
		}
	}
	return &EnginePool{config: config, budget: EngineBudget{MemoryBytes: config.NodeMemoryBytes / int64(config.MaxOpen), TempBytes: config.NodeTempBytes / int64(config.MaxOpen), Threads: config.NodeThreads / config.MaxOpen}, handles: map[string]*Handle{}, engines: map[string]*engineState{}, idle: list.New(), exhaustions: map[string]uint64{}}, nil
}

func (p *EnginePool) Prepare(ctx context.Context, descriptor Descriptor) (*Handle, error) {
	handle, err := p.register(descriptor)
	if err != nil {
		return nil, err
	}
	lease, err := handle.Acquire(ctx)
	if err != nil {
		_ = handle.Close()
		return nil, err
	}
	lease.Release()
	return handle, nil
}

func (p *EnginePool) register(descriptor Descriptor) (*Handle, error) {
	if p == nil {
		return nil, fmt.Errorf("DuckDB engine pool is required")
	}
	if descriptor.WorkspaceID == "" || descriptor.RuntimeID == "" || descriptor.Open == nil {
		return nil, fmt.Errorf("DuckDB engine descriptor is incomplete")
	}
	key := descriptor.WorkspaceID + "\x00" + descriptor.RuntimeID
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, ErrEnginePoolClosed
	}
	if _, ok := p.handles[key]; ok {
		return nil, fmt.Errorf("DuckDB engine descriptor already registered")
	}
	h := &Handle{pool: p, descriptor: descriptor, key: key}
	p.handles[key] = h
	return h, nil
}

func (h *Handle) Acquire(ctx context.Context) (EngineLease, error) {
	started := time.Now()
	lease, err := h.acquire(ctx)
	if h != nil && h.pool != nil {
		h.pool.observeAcquisition(time.Since(started))
	}
	return lease, err
}

func (h *Handle) acquire(ctx context.Context) (EngineLease, error) {
	if h == nil || h.pool == nil {
		return nil, ErrEnginePoolClosed
	}
	if _, _, ok := workload.Current(ctx); h.descriptor.RequireAdmission && !ok {
		return nil, ErrUnadmitted
	}
	if active, _ := ctx.Value(engineContextKey{}).(*engineContext); active != nil {
		if active.handle == h {
			return &nestedEngineLease{engine: active.engine, ctx: ctx}, nil
		}
		return nil, ErrConflictingNested
	}
	p := h.pool
	p.mu.Lock()
	if p.closed || h.closed {
		p.mu.Unlock()
		return nil, ErrEnginePoolClosed
	}
	if state := p.engines[h.key]; state != nil {
		if state.opening {
			ready := state.ready
			p.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-ready:
				if state.openErr != nil {
					return nil, state.openErr
				}
				return h.acquire(ctx)
			}
		}
		if state.idleElement != nil {
			p.idle.Remove(state.idleElement)
			state.idleElement = nil
		}
		if state.refs == 0 {
			p.active++
		}
		state.refs++
		p.reuse++
		engine := state.engine
		p.mu.Unlock()
		return newEngineLease(ctx, p, state, h, engine), nil
	}
	var evicted *engineState
	if p.open >= p.config.MaxOpen {
		element := p.idle.Back()
		if element == nil {
			p.mu.Unlock()
			return nil, ErrCapacityInvariant
		}
		evicted = element.Value.(*engineState)
		p.idle.Remove(element)
		delete(p.engines, evicted.handle.key)
		p.open--
		p.evictions++
	}
	p.open++
	tempDir := filepath.Join(p.config.TempRoot, safeEngineDir(h.descriptor.WorkspaceID, h.descriptor.RuntimeID))
	state := &engineState{handle: h, opening: true, ready: make(chan struct{}), tempDir: tempDir}
	p.engines[h.key] = state
	budget := p.budget
	budget.TempDir = tempDir
	p.mu.Unlock()
	if evicted != nil {
		if err := p.closeState(evicted); err != nil {
			p.mu.Lock()
			delete(p.engines, h.key)
			p.open--
			state.openErr = err
			state.opening = false
			close(state.ready)
			p.mu.Unlock()
			return nil, err
		}
	}
	if err := securefs.EnsurePrivateDir(tempDir); err != nil {
		p.mu.Lock()
		delete(p.engines, h.key)
		p.open--
		state.openErr = err
		state.opening = false
		p.initializationFailures++
		close(state.ready)
		p.mu.Unlock()
		return nil, err
	}
	engine, err := h.descriptor.Open(ctx, budget)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		p.mu.Lock()
		delete(p.engines, h.key)
		p.open--
		state.openErr = err
		state.opening = false
		p.initializationFailures++
		close(state.ready)
		p.mu.Unlock()
		return nil, err
	}
	p.mu.Lock()
	state.engine = engine
	if p.closed || h.closed {
		delete(p.engines, h.key)
		p.open--
		state.openErr = ErrEnginePoolClosed
		state.opening = false
		close(state.ready)
		p.mu.Unlock()
		return nil, errors.Join(p.closeState(state), ErrEnginePoolClosed)
	}
	state.refs = 1
	state.opening = false
	p.active++
	p.opens++
	close(state.ready)
	p.mu.Unlock()
	return newEngineLease(ctx, p, state, h, engine), nil
}

func newEngineLease(parent context.Context, p *EnginePool, state *engineState, h *Handle, engine Engine) *engineLease {
	ctx := context.WithValue(parent, engineContextKey{}, &engineContext{handle: h, engine: engine})
	return &engineLease{pool: p, state: state, ctx: ctx}
}
func (l *engineLease) Engine() Engine           { return l.state.engine }
func (l *engineLease) Context() context.Context { return l.ctx }
func (l *engineLease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		p := l.pool
		p.mu.Lock()
		state := l.state
		if state.refs > 0 {
			state.refs--
			if state.refs == 0 {
				p.active--
			}
		}
		if state.refs == 0 {
			if state.handle.closed || p.closed {
				delete(p.engines, state.handle.key)
				p.open--
				p.mu.Unlock()
				_ = p.closeState(state)
				return
			}
			state.idleElement = p.idle.PushFront(state)
		}
		p.mu.Unlock()
	})
}
func (l *nestedEngineLease) Engine() Engine           { return l.engine }
func (l *nestedEngineLease) Context() context.Context { return l.ctx }
func (l *nestedEngineLease) Release()                 {}

func (h *Handle) Close() error {
	if h == nil || h.pool == nil {
		return nil
	}
	p := h.pool
	p.mu.Lock()
	if h.closed {
		p.mu.Unlock()
		return nil
	}
	h.closed = true
	delete(p.handles, h.key)
	state := p.engines[h.key]
	if state == nil || state.refs > 0 || state.opening {
		p.mu.Unlock()
		return nil
	}
	if state.idleElement != nil {
		p.idle.Remove(state.idleElement)
	}
	delete(p.engines, h.key)
	p.open--
	p.mu.Unlock()
	return p.closeState(state)
}
func closeEngineState(state *engineState) error {
	return errors.Join(state.engine.Close(), os.RemoveAll(state.tempDir))
}

func (p *EnginePool) closeState(state *engineState) error {
	err := closeEngineState(state)
	p.mu.Lock()
	p.cleanups++
	if err != nil {
		p.cleanupFailures++
	}
	p.mu.Unlock()
	return err
}

func (p *EnginePool) Stats() EnginePoolStats {
	if p == nil {
		return EnginePoolStats{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	result := EnginePoolStats{Open: p.open, Active: p.active, Idle: p.open - p.active, Maximum: p.config.MaxOpen, Opens: p.opens, Reuses: p.reuse, Evictions: p.evictions, InitializationFailures: p.initializationFailures, Cleanups: p.cleanups, CleanupFails: p.cleanupFailures, Workspaces: map[string]WorkspaceEngineStats{}, Exhaustions: map[string]uint64{}, AcquisitionDuration: durationSnapshot(p.acquisitionDuration), ResultRows: valueSnapshot(p.resultRows, rowBounds), ResultBytes: valueSnapshot(p.resultBytes, byteBounds)}
	for _, state := range p.engines {
		stats := result.Workspaces[state.handle.descriptor.WorkspaceID]
		stats.Open++
		if state.refs > 0 {
			stats.Active++
		} else {
			stats.Idle++
		}
		result.Workspaces[state.handle.descriptor.WorkspaceID] = stats
	}
	for reason, value := range p.exhaustions {
		result.Exhaustions[reason] = value
	}
	return result
}

func durationSnapshot(value durationHistogram) HistogramSnapshot {
	buckets := make(map[float64]uint64, len(durationBounds))
	for index, bound := range durationBounds {
		buckets[bound] = value.buckets[index]
	}
	return HistogramSnapshot{Count: value.count, Sum: value.sum, Buckets: buckets}
}

func valueSnapshot(value valueHistogram, bounds [8]float64) HistogramSnapshot {
	buckets := make(map[float64]uint64, len(bounds))
	for index, bound := range bounds {
		buckets[bound] = value.buckets[index]
	}
	return HistogramSnapshot{Count: value.count, Sum: value.sum, Buckets: buckets}
}

func (p *EnginePool) observeAcquisition(elapsed time.Duration) {
	seconds := elapsed.Seconds()
	p.mu.Lock()
	p.acquisitionDuration.count++
	p.acquisitionDuration.sum += seconds
	for index, bound := range durationBounds {
		if seconds <= bound {
			p.acquisitionDuration.buckets[index]++
		}
	}
	p.mu.Unlock()
}

func (p *EnginePool) observeResult(rows int, bytes int64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.resultRows.observe(float64(rows), rowBounds)
	p.resultBytes.observe(float64(bytes), byteBounds)
	p.mu.Unlock()
}

func (h *valueHistogram) observe(value float64, bounds [8]float64) {
	h.count++
	h.sum += value
	for index, bound := range bounds {
		if value <= bound {
			h.buckets[index]++
		}
	}
}

func (p *EnginePool) recordExhaustion(reason string) {
	if p == nil || reason == "" {
		return
	}
	p.mu.Lock()
	p.exhaustions[reason]++
	p.mu.Unlock()
}

func (p *EnginePool) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	var idle []*engineState
	for _, h := range p.handles {
		h.closed = true
	}
	for key, state := range p.engines {
		if state.refs == 0 && !state.opening {
			idle = append(idle, state)
			delete(p.engines, key)
			p.open--
		}
	}
	p.idle.Init()
	p.mu.Unlock()
	var err error
	for _, state := range idle {
		err = errors.Join(err, p.closeState(state))
	}
	return err
}

func safeEngineDir(workspace, runtimeID string) string {
	value := fmt.Sprintf("%x", []byte(workspace+"\x00"+runtimeID))
	if len(value) > 64 {
		value = value[:64]
	}
	return time.Now().UTC().Format("20060102T150405.000000000-") + value
}
