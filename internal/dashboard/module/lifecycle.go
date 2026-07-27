package module

import (
	"context"
	"time"

	"github.com/Yacobolo/leapview/internal/dashboard/publication"
)

const publicationMonitorInterval = 500 * time.Millisecond

func (m *Module) Start(ctx context.Context) error {
	if m == nil || !m.PublicationsConfigured() || m.streams == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.lifecycleMu.Lock()
	if m.lifecycleCancel != nil {
		m.lifecycleMu.Unlock()
		return nil
	}
	monitorCtx, cancel := context.WithCancel(ctx)
	m.lifecycleCancel = cancel
	m.lifecycleWG.Add(1)
	m.lifecycleMu.Unlock()
	go m.monitorPublications(monitorCtx)
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.lifecycleMu.Lock()
	cancel := m.lifecycleCancel
	m.lifecycleCancel = nil
	m.lifecycleMu.Unlock()
	if m.coordinators != nil {
		m.coordinators.Close()
	}
	if cancel == nil {
		return nil
	}
	cancel()
	done := make(chan struct{})
	go func() {
		m.lifecycleWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Module) monitorPublications(ctx context.Context) {
	defer m.lifecycleWG.Done()
	reconcile := func() {
		rows, err := m.AllPublications(ctx)
		if err != nil {
			if ctx.Err() == nil && m.logger != nil {
				m.logger.WarnContext(ctx, "dashboard publication stream reconciliation failed", "error", err)
			}
			return
		}
		active := make(map[string]publication.StreamVersion, len(rows))
		for _, row := range rows {
			if row.Status() == publication.StatusActive {
				active[row.ID] = publication.StreamVersion{PublicID: row.PublicID, ServingStateID: row.ServingStateID}
			}
		}
		m.streams.Reconcile(ctx, active)
	}
	reconcile()
	ticker := time.NewTicker(publicationMonitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
		}
	}
}
