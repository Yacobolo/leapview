package command

import (
	"context"
	"net/http"

	"github.com/Yacobolo/libredash/internal/dashboard"
	lddatastar "github.com/Yacobolo/libredash/internal/dashboard/datastar"
	"github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/dashboard/stream"
)

type Metrics interface {
	report.Metrics
	DefaultDashboardID() string
	ModelIDForDashboard(dashboardID string) string
	NormalizeTableRequest(dashboardID string, request dashboard.TableRequest) dashboard.TableRequest
	QueryDashboardPage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters) (dashboard.Patch, error)
	RefreshMaterializations(ctx context.Context, modelID string) error
	DataDir() string
}

type Handler struct {
	Metrics Metrics
	Broker  *stream.Broker
}

func (h Handler) TableWindow(w http.ResponseWriter, r *http.Request) {
	signals, ok := readSignals(w, r)
	if !ok {
		return
	}
	dashboardID := lddatastar.DashboardID(r, signals, h.Metrics.DefaultDashboardID())
	pageID := lddatastar.PageID(r, signals)
	filters := report.NormalizeFilters(h.Metrics, dashboardID, pageID, signals.Filters)
	request := h.Metrics.NormalizeTableRequest(dashboardID, signals.TableCommand)
	clientID := lddatastar.ClientStreamID(r, signals, dashboardID, pageID)

	table := report.QueryTable(r.Context(), h.Metrics, dashboardID, pageID, filters, request)
	if !report.IsCanceledTable(table) {
		h.Broker.Publish(clientID, lddatastar.TablePatch(request.Table, table))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) ChartSelect(w http.ResponseWriter, r *http.Request) {
	h.reloadDashboard(w, r, func(dashboardID, pageID string, signals dashboard.Signals) dashboard.Filters {
		filters := report.NormalizeFilters(h.Metrics, dashboardID, pageID, signals.Filters).ToggleSelection(signals.VisualCommand)
		return report.NormalizeFilters(h.Metrics, dashboardID, pageID, filters)
	})
}

func (h Handler) ClearSelection(w http.ResponseWriter, r *http.Request) {
	h.reloadDashboard(w, r, func(dashboardID, pageID string, signals dashboard.Signals) dashboard.Filters {
		filters := report.NormalizeFilters(h.Metrics, dashboardID, pageID, signals.Filters)
		filters.VisualSelections = nil
		return filters
	})
}

func (h Handler) ResetFilters(w http.ResponseWriter, r *http.Request) {
	h.reloadDashboard(w, r, func(dashboardID, pageID string, _ dashboard.Signals) dashboard.Filters {
		return report.DefaultFilters(h.Metrics, dashboardID, pageID)
	})
}

func (h Handler) RefreshMaterializations(w http.ResponseWriter, r *http.Request) {
	signals, ok := readSignals(w, r)
	if !ok {
		return
	}
	dashboardID := lddatastar.DashboardID(r, signals, h.Metrics.DefaultDashboardID())
	pageID := lddatastar.PageID(r, signals)
	modelID := lddatastar.ModelID(r, signals, dashboardID, h.Metrics.ModelIDForDashboard)
	filters := report.NormalizeFilters(h.Metrics, dashboardID, pageID, signals.Filters)
	request := h.Metrics.NormalizeTableRequest(dashboardID, signals.TableCommand).Reset()
	clientID := lddatastar.ClientStreamID(r, signals, dashboardID, pageID)

	h.Broker.Publish(clientID, lddatastar.LoadingPatch(h.Metrics.DataDir()))
	if err := h.Metrics.RefreshMaterializations(r.Context(), modelID); err != nil {
		h.Broker.Publish(clientID, lddatastar.DashboardPatch(dashboard.EmptyPatch(filters, h.Metrics.DataDir(), err)))
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.publishDashboardReload(r, clientID, dashboardID, pageID, filters, request)
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) reloadDashboard(w http.ResponseWriter, r *http.Request, filters func(dashboardID, pageID string, signals dashboard.Signals) dashboard.Filters) {
	signals, ok := readSignals(w, r)
	if !ok {
		return
	}
	dashboardID := lddatastar.DashboardID(r, signals, h.Metrics.DefaultDashboardID())
	pageID := lddatastar.PageID(r, signals)
	request := h.Metrics.NormalizeTableRequest(dashboardID, signals.TableCommand).Reset()
	clientID := lddatastar.ClientStreamID(r, signals, dashboardID, pageID)

	h.Broker.Publish(clientID, lddatastar.LoadingPatch(h.Metrics.DataDir()))
	h.publishDashboardReload(r, clientID, dashboardID, pageID, filters(dashboardID, pageID, signals), request)
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) publishDashboardReload(r *http.Request, clientID, dashboardID, pageID string, filters dashboard.Filters, request dashboard.TableRequest) {
	patch, err := h.Metrics.QueryDashboardPage(r.Context(), dashboardID, pageID, filters)
	if err != nil {
		patch = dashboard.EmptyPatch(filters, h.Metrics.DataDir(), err)
	}
	h.Broker.Publish(clientID, lddatastar.DashboardPatch(patch))
	h.Broker.Publish(clientID, lddatastar.TablesPatch(report.Tables(r.Context(), h.Metrics, dashboardID, pageID, filters, request)))
}

func readSignals(w http.ResponseWriter, r *http.Request) (dashboard.Signals, bool) {
	signals := dashboard.Signals{}
	if err := lddatastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return dashboard.Signals{}, false
	}
	return signals, true
}
