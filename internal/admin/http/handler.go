package http

import (
	"fmt"
	nethttp "net/http"

	"github.com/Yacobolo/libredash/internal/dashboard"
	"github.com/Yacobolo/libredash/internal/ui"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	Catalog             func() dashboard.Catalog
	Data                func(*nethttp.Request) (ui.AdminData, error)
	CurrentRoleLabel    func(*nethttp.Request) string
	ChromeOption        func(*nethttp.Request) ui.ChromeOption
	EnsureClientID      func(nethttp.ResponseWriter, *nethttp.Request)
	QueryHistoryUpdates nethttp.HandlerFunc
	QueryHistoryCommand nethttp.HandlerFunc
	StorageUpdates      nethttp.HandlerFunc
	StorageSelectTable  nethttp.HandlerFunc
}

func (h Handler) General(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.renderPage(w, r, "general")
}

func (h Handler) Principals(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.renderPage(w, r, "principals")
}

func (h Handler) PrincipalDetail(w nethttp.ResponseWriter, r *nethttp.Request) {
	data, err := h.adminData(r)
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	principalID := chi.URLParam(r, "principal")
	for i := range data.Principals {
		if data.Principals[i].ID == principalID {
			data.SelectedPrincipal = &data.Principals[i]
			h.writePage(w, r, "principal-detail", data)
			return
		}
	}
	nethttp.NotFound(w, r)
}

func (h Handler) Groups(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.renderPage(w, r, "groups")
}

func (h Handler) GroupDetail(w nethttp.ResponseWriter, r *nethttp.Request) {
	data, err := h.adminData(r)
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	groupID := chi.URLParam(r, "group")
	for i := range data.Groups {
		if data.Groups[i].ID == groupID {
			data.SelectedGroup = &data.Groups[i]
			h.writePage(w, r, "group-detail", data)
			return
		}
	}
	nethttp.NotFound(w, r)
}

func (h Handler) Agent(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.renderPage(w, r, "agent")
}

func (h Handler) Storage(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.ensureClientID(w, r)
	h.renderPage(w, r, "storage")
}

func (h Handler) Queries(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.ensureClientID(w, r)
	h.renderPage(w, r, "queries")
}

func (h Handler) QueryUpdates(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.delegate(h.QueryHistoryUpdates, w, r)
}

func (h Handler) QueryCommand(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.delegate(h.QueryHistoryCommand, w, r)
}

func (h Handler) StorageSignalUpdates(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.delegate(h.StorageUpdates, w, r)
}

func (h Handler) StorageTableSelect(w nethttp.ResponseWriter, r *nethttp.Request) {
	h.delegate(h.StorageSelectTable, w, r)
}

func (h Handler) renderPage(w nethttp.ResponseWriter, r *nethttp.Request, active string) {
	data, err := h.adminData(r)
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	h.writePage(w, r, active, data)
}

func (h Handler) writePage(w nethttp.ResponseWriter, r *nethttp.Request, active string, data ui.AdminData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(nethttp.StatusOK)
	if err := ui.AdminPage(h.catalog(), active, h.roleLabel(r), data, h.chromeOption(r)).Render(w); err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
	}
}

func (h Handler) adminData(r *nethttp.Request) (ui.AdminData, error) {
	if h.Data == nil {
		return ui.AdminData{}, fmt.Errorf("admin data provider is not configured")
	}
	return h.Data(r)
}

func (h Handler) catalog() dashboard.Catalog {
	if h.Catalog == nil {
		return dashboard.Catalog{}
	}
	return h.Catalog()
}

func (h Handler) roleLabel(r *nethttp.Request) string {
	if h.CurrentRoleLabel == nil {
		return ""
	}
	return h.CurrentRoleLabel(r)
}

func (h Handler) chromeOption(r *nethttp.Request) ui.ChromeOption {
	if h.ChromeOption == nil {
		return nil
	}
	return h.ChromeOption(r)
}

func (h Handler) ensureClientID(w nethttp.ResponseWriter, r *nethttp.Request) {
	if h.EnsureClientID != nil {
		h.EnsureClientID(w, r)
	}
}

func (h Handler) delegate(next nethttp.HandlerFunc, w nethttp.ResponseWriter, r *nethttp.Request) {
	if next == nil {
		nethttp.NotFound(w, r)
		return
	}
	next(w, r)
}
