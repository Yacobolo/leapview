package lifecycle

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Component struct {
	Start func(context.Context) error
	Stop  func(context.Context) error
}

type Group struct {
	components []Component

	mu         sync.Mutex
	state      groupState
	done       chan struct{}
	cancel     context.CancelFunc
	started    []bool
	startErr   error
	stopErr    error
	stopReq    bool
	stopCtx    context.Context
	stopCancel context.CancelFunc
}

type groupState uint8

const (
	groupIdle groupState = iota
	groupStarting
	groupRunning
	groupStopping
)

func New(components ...Component) *Group {
	return &Group{components: append([]Component(nil), components...)}
}

func (g *Group) Start(ctx context.Context) error {
	if g == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		g.mu.Lock()
		switch g.state {
		case groupRunning:
			g.mu.Unlock()
			return nil
		case groupStarting:
			done := g.done
			g.mu.Unlock()
			if err := wait(ctx, done); err != nil {
				return err
			}
			g.mu.Lock()
			err := g.startErr
			g.mu.Unlock()
			return err
		case groupStopping:
			done := g.done
			g.mu.Unlock()
			if err := wait(ctx, done); err != nil {
				return err
			}
			continue
		default:
			runCtx, cancel := context.WithCancel(ctx)
			g.state = groupStarting
			g.done = make(chan struct{})
			g.cancel = cancel
			g.started = make([]bool, len(g.components))
			g.startErr, g.stopErr = nil, nil
			g.stopReq, g.stopCtx = false, nil
			done := g.done
			g.mu.Unlock()
			return g.startComponents(runCtx, done)
		}
	}
}

func (g *Group) Stop(ctx context.Context) error {
	if g == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	g.mu.Lock()
	switch g.state {
	case groupIdle:
		err := g.stopErr
		g.mu.Unlock()
		return err
	case groupStarting:
		g.stopReq = true
		if g.stopCtx == nil {
			g.stopCtx, g.stopCancel = lifecycleContext()
		}
		done, cancel := g.done, g.cancel
		g.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if err := wait(ctx, done); err != nil {
			return err
		}
		g.mu.Lock()
		err := g.stopErr
		g.mu.Unlock()
		return err
	case groupStopping:
		done := g.done
		g.mu.Unlock()
		if err := wait(ctx, done); err != nil {
			return err
		}
		g.mu.Lock()
		err := g.stopErr
		g.mu.Unlock()
		return err
	case groupRunning:
		g.state = groupStopping
		done := make(chan struct{})
		g.done = done
		cancel := g.cancel
		indexes := g.startedIndexesLocked()
		for _, index := range indexes {
			g.started[index] = false
		}
		g.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		stopCtx, stopCancel := lifecycleContext()
		go g.finishStop(stopCtx, stopCancel, done, indexes)
		if err := wait(ctx, done); err != nil {
			return err
		}
		g.mu.Lock()
		err := g.stopErr
		g.mu.Unlock()
		return err
	default:
		g.mu.Unlock()
		return nil
	}
}

func (g *Group) startComponents(runCtx context.Context, done chan struct{}) error {
	for index, component := range g.components {
		g.mu.Lock()
		stopReq := g.stopReq
		stopCtx := g.stopCtx
		g.mu.Unlock()
		if !stopReq {
			if err := runCtx.Err(); err != nil {
				return g.abortStartup(done, err)
			}
		}
		if stopReq {
			if stopCtx == nil {
				stopCtx = context.Background()
			}
			g.mu.Lock()
			indexes := g.startedIndexesLocked()
			for _, started := range indexes {
				g.started[started] = false
			}
			stopCleanupCancel := g.stopCancel
			g.state = groupStopping
			g.mu.Unlock()
			err := g.stopComponents(stopCtx, indexes)
			if stopCleanupCancel != nil {
				stopCleanupCancel()
			}
			g.mu.Lock()
			g.state, g.cancel, g.stopErr, g.stopCancel = groupIdle, nil, err, nil
			g.startErr = err
			close(done)
			g.mu.Unlock()
			return err
		}
		if component.Start != nil {
			if err := component.Start(runCtx); err != nil {
				return g.abortStartup(done, err)
			}
		}
		g.mu.Lock()
		g.started[index] = true
		g.mu.Unlock()
	}
	g.mu.Lock()
	if g.stopReq {
		indexes := g.startedIndexesLocked()
		for _, started := range indexes {
			g.started[started] = false
		}
		stopCtx := g.stopCtx
		stopCleanupCancel := g.stopCancel
		g.state = groupStopping
		g.mu.Unlock()
		if stopCtx == nil {
			stopCtx = context.Background()
		}
		err := g.stopComponents(stopCtx, indexes)
		if stopCleanupCancel != nil {
			stopCleanupCancel()
		}
		g.mu.Lock()
		g.state, g.cancel, g.stopErr, g.stopCancel = groupIdle, nil, err, nil
		g.startErr = err
		close(done)
		g.mu.Unlock()
		return err
	}
	if err := runCtx.Err(); err != nil {
		g.mu.Unlock()
		return g.abortStartup(done, err)
	}
	g.state = groupRunning
	g.mu.Unlock()
	close(done)
	return nil
}

func (g *Group) abortStartup(done chan struct{}, startupErr error) error {
	g.mu.Lock()
	indexes := g.startedIndexesLocked()
	for _, started := range indexes {
		g.started[started] = false
	}
	cancel := g.cancel
	g.state = groupStopping
	g.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	stopCtx, stopCancel := lifecycleContext()
	stopErr := g.stopComponents(stopCtx, indexes)
	stopCancel()
	err := errors.Join(startupErr, stopErr)
	g.mu.Lock()
	g.state, g.cancel, g.startErr, g.stopErr = groupIdle, nil, err, stopErr
	close(done)
	g.mu.Unlock()
	return err
}

func (g *Group) finishStop(ctx context.Context, cancel context.CancelFunc, done chan struct{}, indexes []int) {
	err := g.stopComponents(ctx, indexes)
	cancel()
	g.mu.Lock()
	g.state, g.cancel, g.stopErr = groupIdle, nil, err
	g.mu.Unlock()
	close(done)
}

func (g *Group) startedIndexesLocked() []int {
	indexes := make([]int, 0, len(g.started))
	for index, started := range g.started {
		if started {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func (g *Group) stopComponents(ctx context.Context, indexes []int) error {
	var errs []error
	for i := len(indexes) - 1; i >= 0; i-- {
		index := indexes[i]
		if stop := g.components[index].Stop; stop != nil {
			errs = append(errs, stop(ctx))
		}
	}
	return errors.Join(errs...)
}

func wait(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func lifecycleContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Minute)
}
