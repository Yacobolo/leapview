package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Yacobolo/leapview/internal/workload"
)

type Handler interface {
	Kind() string
	Handle(context.Context, Job) error
}

type HandlerFunc struct {
	JobKind string
	Run     func(context.Context, Job) error
}

func (h HandlerFunc) Kind() string { return h.JobKind }

func (h HandlerFunc) Handle(ctx context.Context, job Job) error {
	if h.Run == nil {
		return fmt.Errorf("job handler %q is not configured", h.JobKind)
	}
	return h.Run(ctx, job)
}

type RunnerConfig struct {
	Repository   Repository
	Workload     workload.Admitter
	Handlers     []Handler
	LeaseTimeout time.Duration
	PollInterval time.Duration
	Logger       *slog.Logger
}

// Runner owns generic polling, admission, claims, lease renewal, and terminal
// persistence. Capability handlers own payload decoding and business behavior.
type Runner struct {
	repository   Repository
	workload     workload.Admitter
	handlers     map[string]Handler
	leaseTimeout time.Duration
	pollInterval time.Duration
	logger       *slog.Logger
}

func NewRunner(config RunnerConfig) (*Runner, error) {
	if config.Repository == nil || config.Workload == nil {
		return nil, errors.New("job repository and workload controller are required")
	}
	leaseTimeout := config.LeaseTimeout
	if leaseTimeout <= 0 {
		leaseTimeout = 2 * time.Minute
	}
	pollInterval := config.PollInterval
	if pollInterval <= 0 {
		pollInterval = 250 * time.Millisecond
	}
	handlers := make(map[string]Handler, len(config.Handlers))
	for _, handler := range config.Handlers {
		if handler == nil || handler.Kind() == "" {
			return nil, errors.New("job handler kind is required")
		}
		if _, exists := handlers[handler.Kind()]; exists {
			return nil, fmt.Errorf("duplicate job handler %q", handler.Kind())
		}
		handlers[handler.Kind()] = handler
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{repository: config.Repository, workload: config.Workload, handlers: handlers, leaseTimeout: leaseTimeout, pollInterval: pollInterval, logger: logger}, nil
}

func (r *Runner) Run(ctx context.Context) {
	owner := fmt.Sprintf("leapview-jobs-%d", time.Now().UnixNano())
	var pumps sync.WaitGroup
	for _, class := range []workload.Class{workload.Control, workload.Background} {
		class := class
		pumps.Add(1)
		go func() {
			defer pumps.Done()
			r.runPump(ctx, owner, class)
		}()
	}
	pumps.Wait()
}

func (r *Runner) runPump(ctx context.Context, owner string, class workload.Class) {
	poll := time.NewTicker(r.pollInterval)
	defer poll.Stop()
	for {
		candidates, err := r.repository.Candidates(ctx, string(class), 16)
		if err != nil {
			r.logger.WarnContext(ctx, "list async job candidates failed", "class", class, "error", err)
		} else {
			var batch sync.WaitGroup
			for _, candidate := range candidates {
				candidate := candidate
				batch.Add(1)
				go func() {
					defer batch.Done()
					r.dispatchCandidate(ctx, owner, class, candidate)
				}()
			}
			batch.Wait()
		}
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
		}
	}
}

func (r *Runner) dispatchCandidate(ctx context.Context, owner string, class workload.Class, candidate Job) {
	lease, err := r.workload.Acquire(ctx, workload.Request{Class: class, WorkspaceID: candidate.WorkspaceID, Operation: candidate.Kind})
	if err != nil {
		return
	}
	defer lease.Release()
	job, ok, err := r.repository.ClaimByID(lease.Context(), candidate.ID, string(class), owner, r.leaseTimeout)
	if err != nil || !ok {
		return
	}
	r.executeClaimed(lease.Context(), job)
}

func (r *Runner) executeClaimed(parent context.Context, job Job) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	done := make(chan struct{})
	go func() {
		interval := r.leaseTimeout / 2
		if interval <= 0 {
			interval = time.Millisecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := r.repository.Renew(context.WithoutCancel(ctx), job.ID, job.Fence(), r.leaseTimeout); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	handler, ok := r.handlers[job.Kind]
	var err error
	if !ok {
		err = fmt.Errorf("unsupported async job kind %q", job.Kind)
	} else {
		err = handler.Handle(ctx, job)
	}
	close(done)
	if ctx.Err() != nil {
		return
	}
	if err == nil {
		_ = r.repository.Complete(context.WithoutCancel(ctx), job.ID, job.Fence())
		return
	}
	problem, _ := json.Marshal(map[string]any{"code": "ASYNC_JOB_FAILED", "detail": err.Error()})
	_ = r.repository.Fail(context.WithoutCancel(ctx), job.ID, job.Fence(), problem)
	r.logger.ErrorContext(ctx, "async job failed", "kind", job.Kind, "resource", job.ResourceID, "error", err)
}
