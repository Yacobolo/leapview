package http

import (
	nethttp "net/http"
	"time"

	lddatastar "github.com/Yacobolo/libredash/internal/dashboard/datastar"
	"github.com/Yacobolo/libredash/internal/dashboard/stream"
)

func (h Handler) Updates(w nethttp.ResponseWriter, r *nethttp.Request) {
	signals, ok := h.readSignals(w, r)
	if !ok {
		return
	}
	dashboardID := lddatastar.DashboardID(r, signals, h.Metrics.DefaultDashboardID())
	pageID := lddatastar.PageID(r, signals)
	clientID := lddatastar.ClientStreamID(r, signals, dashboardID, pageID)
	request := stream.SnapshotRequest{
		DashboardID:  dashboardID,
		PageID:       pageID,
		Filters:      signals.Filters,
		TableCommand: signals.TableCommand,
	}

	writer := lddatastar.NewSignalWriter(w, r)
	updates, unsubscribe := h.Broker.Subscribe(clientID)
	defer unsubscribe()

	if err := writer.Patch(lddatastar.LoadingPatch(h.Metrics.DataDir())); err != nil {
		return
	}
	if !h.queryAndPatch(r, writer, request) {
		return
	}

	interval := h.TickerInterval
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case patch, ok := <-updates:
			if !ok {
				return
			}
			if err := writer.Patch(patch); err != nil {
				return
			}
		case <-ticker.C:
			if !h.queryAndPatch(r, writer, request) {
				return
			}
		}
	}
}

func (h Handler) queryAndPatch(r *nethttp.Request, writer lddatastar.SignalWriter, request stream.SnapshotRequest) bool {
	snapshot := stream.Service{Metrics: h.Metrics}.Snapshot(r.Context(), request)
	for _, patch := range lddatastar.SnapshotPatches(snapshot) {
		if err := writer.Patch(patch); err != nil {
			return false
		}
	}
	return true
}
