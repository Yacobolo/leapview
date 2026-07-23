package lifecycle

import (
	"context"
	"errors"
	"sync"
)

type Component struct {
	Start func(context.Context) error
	Stop  func(context.Context) error
}

type Group struct {
	components []Component

	mu      sync.Mutex
	cancel  context.CancelFunc
	started int
}

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
	g.mu.Lock()
	if g.cancel != nil {
		g.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	g.mu.Unlock()

	for index, component := range g.components {
		if component.Start == nil {
			g.mu.Lock()
			g.started = index + 1
			g.mu.Unlock()
			continue
		}
		if err := component.Start(runCtx); err != nil {
			cancel()
			stopErr := g.stopStarted(context.Background())
			return errors.Join(err, stopErr)
		}
		g.mu.Lock()
		g.started = index + 1
		g.mu.Unlock()
	}
	return nil
}

func (g *Group) Stop(ctx context.Context) error {
	if g == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	g.mu.Lock()
	cancel := g.cancel
	g.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return g.stopStarted(ctx)
}

func (g *Group) stopStarted(ctx context.Context) error {
	g.mu.Lock()
	started := g.started
	g.started = 0
	g.cancel = nil
	g.mu.Unlock()
	var errs []error
	for index := started - 1; index >= 0; index-- {
		if stop := g.components[index].Stop; stop != nil {
			errs = append(errs, stop(ctx))
		}
	}
	return errors.Join(errs...)
}
