package app

import (
	"context"
	"time"

	"github.com/Yacobolo/leapview/internal/dashboard/publication"
)

const publicationMonitorInterval = 500 * time.Millisecond

func (s *Server) startPublicationMonitor(ctx context.Context) {
	s.jobDispatchWG.Add(1)
	go func() {
		defer s.jobDispatchWG.Done()
		reconcile := func() {
			rows, err := s.publicationRepo.ListAll(ctx)
			if err != nil {
				if ctx.Err() == nil {
					s.logger.WarnContext(ctx, "dashboard publication stream reconciliation failed", "error", err)
				}
				return
			}
			active := make(map[string]publication.StreamVersion, len(rows))
			for _, row := range rows {
				if row.Status() != publication.StatusActive {
					continue
				}
				active[row.ID] = publication.StreamVersion{PublicID: row.PublicID, ServingStateID: row.ServingStateID}
			}
			s.publicationStreams.Reconcile(ctx, active)
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
	}()
}
