package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// Lifecycle is the narrow process-owned contract retained by Application.
// Capability modules that own workers may implement it without exposing their
// repositories or internal dependency graphs.
type Lifecycle interface {
	Start(context.Context) error
	Stop(context.Context) error
}

type fatalSource interface {
	Fatal() <-chan error
}

type cleanupFunc func(context.Context) error

// Application is the complete process-facing surface. It retains only the
// final handler, lifecycle components, fatal probes, and cleanup closures.
type Application struct {
	handler    http.Handler
	components []Lifecycle
	cleanup    []cleanupFunc

	lifecycle   applicationLifecycleState
	cleanupOnce sync.Once
	cleanupErr  error
	fatal       chan error
}

func newApplication(handler http.Handler, components []Lifecycle, cleanup ...cleanupFunc) *Application {
	return &Application{
		handler: handler, components: append([]Lifecycle(nil), components...),
		cleanup: append([]cleanupFunc(nil), cleanup...), fatal: make(chan error, 1),
	}
}

func (a *Application) Handler() http.Handler {
	if a == nil || a.handler == nil {
		return http.NotFoundHandler()
	}
	return a.handler
}

func (a *Application) Start(ctx context.Context) error {
	if a == nil {
		return errors.New("application is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	a.lifecycle.mu.Lock()
	if a.lifecycle.shutdownRequested {
		a.lifecycle.mu.Unlock()
		return context.Canceled
	}
	if done := a.lifecycle.startDone; done != nil {
		a.lifecycle.mu.Unlock()
		select {
		case <-done:
			a.lifecycle.mu.Lock()
			err := a.lifecycle.startErr
			a.lifecycle.mu.Unlock()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done := make(chan struct{})
	runCtx, cancel := context.WithCancel(ctx)
	a.lifecycle.startDone = done
	a.lifecycle.startCancel = cancel
	a.lifecycle.mu.Unlock()

	err := a.startComponents(runCtx)
	if err != nil {
		cancel()
	}
	a.lifecycle.mu.Lock()
	if err != nil {
		a.lifecycle.startCancel = nil
	}
	a.lifecycle.startErr = err
	close(done)
	a.lifecycle.mu.Unlock()
	return err
}

func (a *Application) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	a.lifecycle.mu.Lock()
	a.lifecycle.shutdownRequested = true
	startCancel := a.lifecycle.startCancel
	startDone := a.lifecycle.startDone
	a.lifecycle.mu.Unlock()
	if startCancel != nil {
		startCancel()
	}
	if startDone != nil {
		select {
		case <-startDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	a.lifecycle.mu.Lock()
	if done := a.lifecycle.shutdownDone; done != nil {
		a.lifecycle.mu.Unlock()
		select {
		case <-done:
			a.lifecycle.mu.Lock()
			err := a.lifecycle.shutdownErr
			a.lifecycle.mu.Unlock()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done := make(chan struct{})
	a.lifecycle.shutdownDone = done
	a.lifecycle.mu.Unlock()

	stopErr := a.stopStarted(ctx)
	a.runCleanup(ctx)
	err := errors.Join(stopErr, a.cleanupErr)
	a.lifecycle.mu.Lock()
	a.lifecycle.shutdownErr = err
	close(done)
	a.lifecycle.mu.Unlock()
	return err
}

func (a *Application) startComponents(ctx context.Context) error {
	for index, component := range a.components {
		if err := ctx.Err(); err != nil {
			return a.rollbackStartup(ctx, err)
		}
		if component != nil {
			if err := component.Start(ctx); err != nil {
				return a.rollbackStartup(ctx, fmt.Errorf("start application component %d: %w", index, err))
			}
		}
		a.lifecycle.mu.Lock()
		a.lifecycle.started = index + 1
		shutdownRequested := a.lifecycle.shutdownRequested
		a.lifecycle.mu.Unlock()
		if component != nil {
			a.forwardFatal(ctx, component)
		}
		if shutdownRequested || ctx.Err() != nil {
			return a.rollbackStartup(ctx, context.Canceled)
		}
	}
	return nil
}

func (a *Application) rollbackStartup(ctx context.Context, cause error) error {
	rollbackCtx := context.WithoutCancel(ctx)
	stopErr := a.stopStarted(rollbackCtx)
	a.runCleanup(rollbackCtx)
	return errors.Join(cause, stopErr, a.cleanupErr)
}

func (a *Application) runCleanup(ctx context.Context) {
	a.cleanupOnce.Do(func() {
		var errs []error
		for index := len(a.cleanup) - 1; index >= 0; index-- {
			if cleanup := a.cleanup[index]; cleanup != nil {
				errs = append(errs, cleanup(ctx))
			}
		}
		a.cleanupErr = errors.Join(errs...)
	})
}

func (a *Application) Fatal() <-chan error {
	if a == nil {
		return nil
	}
	return a.fatal
}

func (a *Application) stopStarted(ctx context.Context) error {
	a.lifecycle.mu.Lock()
	started := a.lifecycle.started
	a.lifecycle.started = 0
	a.lifecycle.mu.Unlock()
	var errs []error
	for index := started - 1; index >= 0; index-- {
		if component := a.components[index]; component != nil {
			errs = append(errs, component.Stop(ctx))
		}
	}
	return errors.Join(errs...)
}

func (a *Application) forwardFatal(ctx context.Context, component Lifecycle) {
	source, ok := component.(fatalSource)
	if !ok || source.Fatal() == nil {
		return
	}
	go func() {
		select {
		case <-ctx.Done():
		case err, open := <-source.Fatal():
			if !open || err == nil {
				return
			}
			select {
			case a.fatal <- err:
			default:
			}
		}
	}()
}
