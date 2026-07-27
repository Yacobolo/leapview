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

	startOnce    sync.Once
	startErr     error
	started      int
	shutdownOnce sync.Once
	shutdownErr  error
	cleanupOnce  sync.Once
	cleanupErr   error
	fatal        chan error
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
	a.startOnce.Do(func() {
		for index, component := range a.components {
			if component == nil {
				continue
			}
			if err := component.Start(ctx); err != nil {
				a.startErr = fmt.Errorf("start application component %d: %w", index, err)
				a.stopStarted(ctx)
				a.runCleanup(ctx)
				a.startErr = errors.Join(a.startErr, a.cleanupErr)
				return
			}
			a.started = index + 1
			a.forwardFatal(ctx, component)
		}
	})
	return a.startErr
}

func (a *Application) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	a.shutdownOnce.Do(func() {
		var errs []error
		for index := a.started - 1; index >= 0; index-- {
			if component := a.components[index]; component != nil {
				errs = append(errs, component.Stop(ctx))
			}
		}
		a.started = 0
		a.runCleanup(ctx)
		errs = append(errs, a.cleanupErr)
		a.shutdownErr = errors.Join(errs...)
	})
	return a.shutdownErr
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

func (a *Application) stopStarted(ctx context.Context) {
	for index := a.started - 1; index >= 0; index-- {
		if component := a.components[index]; component != nil {
			a.startErr = errors.Join(a.startErr, component.Stop(ctx))
		}
	}
	a.started = 0
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
