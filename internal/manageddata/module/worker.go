package module

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Yacobolo/leapview/internal/manageddata/control"
)

type MaintenanceLease interface {
	Context() context.Context
	Release()
}

type MaintenanceWorkerConfig struct {
	Interval time.Duration
	Acquire  func(context.Context) (MaintenanceLease, error)
	Logger   *slog.Logger
}

type uploadExpirer interface {
	ExpireUploads(context.Context) (control.ExpireResult, error)
}

type maintenanceWorker struct {
	expirer  uploadExpirer
	interval time.Duration
	acquire  func(context.Context) (MaintenanceLease, error)
	logger   *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func newMaintenanceWorker(expirer uploadExpirer, config MaintenanceWorkerConfig) *maintenanceWorker {
	if expirer == nil || config.Acquire == nil {
		return nil
	}
	interval := config.Interval
	if interval <= 0 {
		interval = time.Hour
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &maintenanceWorker{expirer: expirer, interval: interval, acquire: config.Acquire, logger: logger}
}

func (w *maintenanceWorker) Start(ctx context.Context) {
	if w == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	w.mu.Lock()
	if w.cancel != nil {
		w.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	w.cancel, w.done = cancel, done
	w.mu.Unlock()

	go func() {
		defer close(done)
		w.runPass(runCtx)
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				w.runPass(runCtx)
			}
		}
	}()
}

func (w *maintenanceWorker) Stop(ctx context.Context) error {
	if w == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	if cancel == nil {
		w.mu.Unlock()
		return nil
	}
	cancel()
	w.mu.Unlock()

	select {
	case <-done:
		w.mu.Lock()
		if w.done == done {
			w.cancel, w.done = nil, nil
		}
		w.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *maintenanceWorker) runPass(ctx context.Context) {
	lease, err := w.acquire(ctx)
	if err != nil {
		w.logger.DebugContext(ctx, "managed-data maintenance skipped", "error", err)
		return
	}
	defer lease.Release()
	result, err := w.expirer.ExpireUploads(lease.Context())
	if err != nil {
		w.logger.WarnContext(ctx, "managed-data upload expiration failed", "error", err)
		return
	}
	if result.Expired > 0 {
		w.logger.InfoContext(ctx, "expired managed-data upload sessions", "count", result.Expired)
	}
}
