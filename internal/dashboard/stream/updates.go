package stream

import (
	"context"
	"net/http"
	"time"

	"github.com/Yacobolo/libredash/internal/dashboard"
	lddatastar "github.com/Yacobolo/libredash/internal/dashboard/datastar"
	"github.com/Yacobolo/libredash/internal/dashboard/report"
	ds "github.com/starfederation/datastar-go/datastar"
)

type Metrics interface {
	report.Metrics
	DefaultDashboardID() string
	NormalizeTableRequest(dashboardID string, request dashboard.TableRequest) dashboard.TableRequest
	QueryDashboardPage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters) (dashboard.Patch, error)
	DataDir() string
}

type Handler struct {
	Metrics        Metrics
	Broker         *Broker
	TickerInterval time.Duration
}

func (h Handler) Updates(w http.ResponseWriter, r *http.Request) {
	signals := dashboard.Signals{}
	if err := lddatastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dashboardID := lddatastar.DashboardID(r, signals, h.Metrics.DefaultDashboardID())
	pageID := lddatastar.PageID(r, signals)
	filters := report.NormalizeFilters(h.Metrics, dashboardID, pageID, signals.Filters)
	clientID := lddatastar.ClientStreamID(r, signals, dashboardID, pageID)
	tableRequest := h.Metrics.NormalizeTableRequest(dashboardID, signals.TableCommand)

	sse := ds.NewSSE(w, r)
	updates, unsubscribe := h.Broker.Subscribe(clientID)
	defer unsubscribe()

	if err := sse.MarshalAndPatchSignals(lddatastar.LoadingPatch(h.Metrics.DataDir())); err != nil {
		return
	}
	if !h.queryAndPatch(w, r, sse, dashboardID, pageID, filters, tableRequest) {
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
			if err := sse.MarshalAndPatchSignals(patch); err != nil {
				return
			}
		case <-ticker.C:
			if !h.queryAndPatch(w, r, sse, dashboardID, pageID, filters, tableRequest) {
				return
			}
		}
	}
}

func (h Handler) queryAndPatch(_ http.ResponseWriter, r *http.Request, sse *ds.ServerSentEventGenerator, dashboardID, pageID string, filters dashboard.Filters, tableRequest dashboard.TableRequest) bool {
	patch, err := h.Metrics.QueryDashboardPage(r.Context(), dashboardID, pageID, filters)
	if err != nil {
		patch = dashboard.EmptyPatch(filters, h.Metrics.DataDir(), err)
	}
	if err := sse.MarshalAndPatchSignals(patch); err != nil {
		return false
	}
	tables := report.Tables(r.Context(), h.Metrics, dashboardID, pageID, filters, tableRequest)
	if err := sse.MarshalAndPatchSignals(lddatastar.TablesPatch(tables)); err != nil {
		return false
	}
	return true
}
