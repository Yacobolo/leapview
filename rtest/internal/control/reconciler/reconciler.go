package reconciler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flidai/leapview/rtest/internal/control"
	"github.com/flidai/leapview/rtest/internal/protocol"
)

type Store interface {
	ScheduledJobs(context.Context) ([]control.Job, error)
	Job(context.Context, string) (control.Job, error)
	SyncJob(context.Context, string, protocol.Job) (control.Job, error)
}

type Scheduler interface {
	ManagedJobs(context.Context) ([]protocol.Job, error)
	Remove(context.Context, string) error
}

type Config struct {
	Store            Store
	Scheduler        Scheduler
	ServiceRetention time.Duration
	Now              func() time.Time
}

type Reconciler struct {
	config Config
}

func New(config Config) *Reconciler {
	if config.ServiceRetention <= 0 {
		config.ServiceRetention = time.Hour
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Reconciler{config: config}
}

func (r *Reconciler) RunOnce(ctx context.Context) error {
	scheduled, err := r.config.Store.ScheduledJobs(ctx)
	if err != nil {
		return fmt.Errorf("list scheduled jobs: %w", err)
	}
	managed, err := r.config.Scheduler.ManagedJobs(ctx)
	if err != nil {
		return fmt.Errorf("list managed jobs: %w", err)
	}
	remoteByID := make(map[string]protocol.Job, len(managed))
	for _, job := range managed {
		remoteByID[job.ID] = job
	}
	now := r.config.Now().UTC()
	for _, job := range scheduled {
		remote, ok := remoteByID[job.ID]
		if !ok {
			finished, exitCode := now, 1
			remote = protocol.Job{ID: job.ID, Status: protocol.StatusLost, FinishedAt: &finished, ExitCode: &exitCode, ErrorMessage: "managed Swarm service is missing"}
		}
		if _, err := r.config.Store.SyncJob(ctx, job.ID, remote); err != nil {
			return fmt.Errorf("synchronize job %s: %w", job.ID, err)
		}
	}

	cutoff := now.Add(-r.config.ServiceRetention)
	for _, remote := range managed {
		stored, err := r.config.Store.Job(ctx, remote.ID)
		if err != nil && !errors.Is(err, control.ErrNotFound) {
			return fmt.Errorf("read managed job %s: %w", remote.ID, err)
		}
		removeAfter := remote.CreatedAt
		if err == nil {
			if !stored.Status.Terminal() {
				continue
			}
			if stored.FinishedAt != nil {
				removeAfter = *stored.FinishedAt
			} else if remote.FinishedAt != nil {
				removeAfter = *remote.FinishedAt
			}
		} else if remote.FinishedAt != nil {
			removeAfter = *remote.FinishedAt
		}
		if removeAfter.IsZero() || removeAfter.After(cutoff) {
			continue
		}
		if err := r.config.Scheduler.Remove(ctx, remote.ID); err != nil {
			return fmt.Errorf("remove managed job %s: %w", remote.ID, err)
		}
	}
	return nil
}
