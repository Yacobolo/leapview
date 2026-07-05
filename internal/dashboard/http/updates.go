package http

import (
	nethttp "net/http"

	lddatastar "github.com/Yacobolo/libredash/internal/dashboard/datastar"
	"github.com/Yacobolo/libredash/internal/dashboard/stream"
	"github.com/Yacobolo/libredash/pkg/pagestream"
)

func (h Handler) Updates(w nethttp.ResponseWriter, r *nethttp.Request) {
	metrics, ok := h.metricsForRequest(r)
	if !ok {
		nethttp.NotFound(w, r)
		return
	}
	signals, ok := h.readSignals(w, r)
	if !ok {
		return
	}
	dashboardID := lddatastar.DashboardID(r, signals, metrics.DefaultDashboardID())
	pageID := lddatastar.PageID(r, signals)
	clientID := lddatastar.ClientStreamID(r, signals, dashboardID, pageID)
	request := stream.SnapshotRequest{
		DashboardID:  dashboardID,
		PageID:       pageID,
		Filters:      signals.Filters,
		TableCommand: signals.TableCommand,
	}

	updates := pagestream.NewSignalStream(w, r)
	if err := updates.Patch(lddatastar.LoadingPatch(metrics.DataDir())); err != nil {
		return
	}
	snapshot := stream.Service{Metrics: metrics}.Snapshot(r.Context(), request)
	for _, patch := range lddatastar.SnapshotPatches(snapshot) {
		if err := updates.Patch(patch); err != nil {
			return
		}
	}
	_ = updates.Forward(r.Context(), h.Broker, clientID)
}
