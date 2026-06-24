package http

import (
	"context"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	nethttp "net/http"
	"time"

	"github.com/Yacobolo/libredash/internal/dashboard"
	lddatastar "github.com/Yacobolo/libredash/internal/dashboard/datastar"
	"github.com/Yacobolo/libredash/internal/dashboard/report"
	reportdef "github.com/Yacobolo/libredash/internal/dashboard/report"
	"github.com/Yacobolo/libredash/internal/dashboard/stream"
	reportui "github.com/Yacobolo/libredash/internal/dashboard/ui"
	"github.com/go-chi/chi/v5"
)

type Metrics interface {
	Catalog() dashboard.Catalog
	DataDir() string
	DefaultDashboardID() string
	DefaultFilters(dashboardID string) dashboard.Filters
	ModelIDForDashboard(dashboardID string) string
	NormalizeTableRequest(dashboardID string, request dashboard.TableRequest) dashboard.TableRequest
	Pages(dashboardID string) []dashboard.Page
	QueryDashboardPage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters) (dashboard.Patch, error)
	QueryTablePage(ctx context.Context, dashboardID, pageID string, filters dashboard.Filters, request dashboard.TableRequest) (dashboard.Table, error)
	Report(dashboardID string) (reportdef.Dashboard, *semanticmodel.Model, bool)
	RefreshMaterializations(ctx context.Context, modelID string) error
}

type Handler struct {
	Metrics        Metrics
	Broker         *stream.Broker
	TickerInterval time.Duration
	CSRFToken      func(r *nethttp.Request) string
}

func (h Handler) Dashboard(w nethttp.ResponseWriter, r *nethttp.Request) {
	dashboardID := chi.URLParam(r, "dashboard")
	pages := h.Metrics.Pages(dashboardID)
	if len(pages) == 0 {
		nethttp.NotFound(w, r)
		return
	}
	nethttp.Redirect(w, r, "/dashboards/"+dashboardID+"/pages/"+pages[0].ID, nethttp.StatusFound)
}

func (h Handler) Page(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.RenderPage(w, r, chi.URLParam(r, "dashboard"), chi.URLParam(r, "page"))
}

func (h Handler) RenderPage(w nethttp.ResponseWriter, r *nethttp.Request, dashboardID, pageID string) {
	clientID := lddatastar.EnsureClientID(w, r)
	reportDefinition, model, ok := h.Metrics.Report(dashboardID)
	if !ok {
		nethttp.NotFound(w, r)
		return
	}
	pages := h.Metrics.Pages(dashboardID)
	activePage, ok := report.ActivePage(pages, pageID)
	if !ok {
		nethttp.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(nethttp.StatusOK)
	initialFilters := reportDefinition.FiltersFromURLForPage(activePage.ID, r.URL.Query())
	csrfToken := ""
	if h.CSRFToken != nil {
		csrfToken = h.CSRFToken(r)
	}
	if err := reportui.Page(h.Metrics.DataDir(), clientID, csrfToken, h.Metrics.Catalog(), reportDefinition, model, pages, activePage, initialFilters).Render(w); err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
	}
}
