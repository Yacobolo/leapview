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

// Application is the complete process-facing surface. Construction details
// stay behind the private lifecycle owner instead of leaking a service graph.
type Application struct {
	handler   http.Handler
	lifecycle *applicationLifecycleOwner
}

type applicationLifecycleOwner struct {
	components  []Lifecycle
	cleanup     []cleanupFunc
	state       applicationLifecycleState
	cleanupOnce sync.Once
	cleanupErr  error
	fatal       chan error
}

func newApplication(handler http.Handler, components []Lifecycle, cleanup ...cleanupFunc) *Application {
	return &Application{
		handler: handler,
		lifecycle: &applicationLifecycleOwner{
			components: append([]Lifecycle(nil), components...),
			cleanup:    append([]cleanupFunc(nil), cleanup...), fatal: make(chan error, 1),
		},
	}
}

func (a *Application) Handler() http.Handler {
	if a == nil || a.handler == nil {
		return http.NotFoundHandler()
	}
	return a.handler
}

func (a *Application) Start(ctx context.Context) error {
	if a == nil || a.lifecycle == nil {
		return errors.New("application is not initialized")
	}
	return a.lifecycle.start(ctx)
}

func (a *Application) Shutdown(ctx context.Context) error {
	if a == nil || a.lifecycle == nil {
		return nil
	}
	return a.lifecycle.shutdown(ctx)
}

func (a *Application) Fatal() <-chan error {
	if a == nil || a.lifecycle == nil {
		return nil
	}
	return a.lifecycle.fatalChannel()
}

func (o *applicationLifecycleOwner) start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	o.state.mu.Lock()
	if o.state.shutdownRequested {
		o.state.mu.Unlock()
		return context.Canceled
	}
	if done := o.state.startDone; done != nil {
		o.state.mu.Unlock()
		select {
		case <-done:
			o.state.mu.Lock()
			err := o.state.startErr
			o.state.mu.Unlock()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done := make(chan struct{})
	runCtx, cancel := context.WithCancel(ctx)
	o.state.startDone = done
	o.state.startCancel = cancel
	o.state.mu.Unlock()

	err := o.startComponents(runCtx)
	if err != nil {
		cancel()
	}
	o.state.mu.Lock()
	if err != nil {
		o.state.startCancel = nil
	}
	o.state.startErr = err
	close(done)
	o.state.mu.Unlock()
	return err
}

func (o *applicationLifecycleOwner) shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	o.state.mu.Lock()
	o.state.shutdownRequested = true
	startCancel := o.state.startCancel
	startDone := o.state.startDone
	o.state.mu.Unlock()
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

	o.state.mu.Lock()
	if done := o.state.shutdownDone; done != nil {
		o.state.mu.Unlock()
		select {
		case <-done:
			o.state.mu.Lock()
			err := o.state.shutdownErr
			o.state.mu.Unlock()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done := make(chan struct{})
	o.state.shutdownDone = done
	o.state.mu.Unlock()

	stopErr := o.stopStarted(ctx)
	o.runCleanup(ctx)
	err := errors.Join(stopErr, o.cleanupErr)
	o.state.mu.Lock()
	o.state.shutdownErr = err
	close(done)
	o.state.mu.Unlock()
	return err
}

func (o *applicationLifecycleOwner) startComponents(ctx context.Context) error {
	for index, component := range o.components {
		if err := ctx.Err(); err != nil {
			return o.rollbackStartup(ctx, err)
		}
		if component != nil {
			if err := component.Start(ctx); err != nil {
				return o.rollbackStartup(ctx, fmt.Errorf("start application component %d: %w", index, err))
			}
		}
		o.state.mu.Lock()
		o.state.started = index + 1
		shutdownRequested := o.state.shutdownRequested
		o.state.mu.Unlock()
		if component != nil {
			o.forwardFatal(ctx, component)
		}
		if shutdownRequested || ctx.Err() != nil {
			return o.rollbackStartup(ctx, context.Canceled)
		}
	}
	return nil
}

func (o *applicationLifecycleOwner) rollbackStartup(ctx context.Context, cause error) error {
	rollbackCtx := context.WithoutCancel(ctx)
	stopErr := o.stopStarted(rollbackCtx)
	o.runCleanup(rollbackCtx)
	return errors.Join(cause, stopErr, o.cleanupErr)
}

func (o *applicationLifecycleOwner) runCleanup(ctx context.Context) {
	o.cleanupOnce.Do(func() {
		var errs []error
		for index := len(o.cleanup) - 1; index >= 0; index-- {
			if cleanup := o.cleanup[index]; cleanup != nil {
				errs = append(errs, cleanup(ctx))
			}
		}
		o.cleanupErr = errors.Join(errs...)
	})
}

func (o *applicationLifecycleOwner) fatalChannel() <-chan error {
	return o.fatal
}

func (o *applicationLifecycleOwner) stopStarted(ctx context.Context) error {
	o.state.mu.Lock()
	started := o.state.started
	o.state.started = 0
	o.state.mu.Unlock()
	var errs []error
	for index := started - 1; index >= 0; index-- {
		if component := o.components[index]; component != nil {
			errs = append(errs, component.Stop(ctx))
		}
	}
	return errors.Join(errs...)
}

func (o *applicationLifecycleOwner) forwardFatal(ctx context.Context, component Lifecycle) {
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
			case o.fatal <- err:
			default:
			}
		}
	}()
}
