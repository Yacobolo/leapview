package app

import (
	"net/http"
	"strings"

	"github.com/Yacobolo/libredash/internal/access"
	uisignals "github.com/Yacobolo/libredash/internal/ui/signals"
	"github.com/Yacobolo/libredash/pkg/pagestream"
)

type updateRouteSignals struct {
	Runtime uisignals.RouteRuntimeSignal `json:"runtime"`
}

func (s *Server) updates(w http.ResponseWriter, r *http.Request) {
	route := updateRoute(r)
	if route == "" {
		http.Error(w, "updates route is required", http.StatusBadRequest)
		return
	}
	permission, ok := updatesPermission(route, r.URL.Query().Get("section"))
	if !ok {
		http.Error(w, "unknown updates route", http.StatusBadRequest)
		return
	}
	if permission == "" {
		s.serveUpdates(w, r, route)
		return
	}
	s.protect(permission, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.serveUpdates(w, r, route)
	})).ServeHTTP(w, r)
}

func updateRoute(r *http.Request) string {
	route := strings.TrimSpace(r.URL.Query().Get("route"))
	if route != "" {
		return route
	}
	if strings.TrimSpace(r.URL.Query().Get("dashboard")) != "" {
		return string(uisignals.RouteDashboard)
	}
	var signals updateRouteSignals
	if err := pagestream.ReadSignals(r, &signals); err == nil {
		if signals.Runtime.RouteKey != "" {
			return string(signals.Runtime.RouteKey)
		}
		if signals.Runtime.Kind != "" {
			return string(signals.Runtime.Kind)
		}
	}
	return ""
}

func updatesPermission(route, section string) (string, bool) {
	switch uisignals.RouteKind(route) {
	case uisignals.RouteLogin:
		return "", true
	case uisignals.RouteCatalog, uisignals.RouteDashboard, uisignals.RouteWorkspace, uisignals.RouteWorkspaceAsset, uisignals.RouteConnections, uisignals.RouteConnectionAsset, uisignals.RouteData:
		return access.PermissionDashboardView, true
	case uisignals.RouteChat:
		return access.PermissionAgentRead, true
	case uisignals.RouteAdmin:
		if strings.TrimSpace(section) == "queries" {
			return access.PermissionAuditRead, true
		}
		return access.PermissionRBACRead, true
	default:
		return "", false
	}
}

func (s *Server) serveUpdates(w http.ResponseWriter, r *http.Request, route string) {
	switch uisignals.RouteKind(route) {
	case uisignals.RouteDashboard:
		s.dashboardHTTP().Updates(w, r)
	case uisignals.RouteChat:
		if s.agent == nil || !s.agent.Enabled() {
			s.noopUpdates(w, r)
			return
		}
		s.chatUpdates(w, r)
	case uisignals.RouteData:
		s.dataExplorerUpdates(w, r)
	case uisignals.RouteAdmin:
		switch strings.TrimSpace(r.URL.Query().Get("section")) {
		case "queries":
			s.adminQueryHistoryUpdates(w, r)
		case "storage":
			s.adminStorageUpdates(w, r)
		default:
			s.noopUpdates(w, r)
		}
	case uisignals.RouteWorkspaceAsset, uisignals.RouteConnectionAsset:
		if strings.TrimSpace(r.URL.Query().Get("asset")) != "" {
			s.workspaceAssetUpdates(w, r)
			return
		}
		s.noopUpdates(w, r)
	case uisignals.RouteLogin, uisignals.RouteCatalog, uisignals.RouteWorkspace, uisignals.RouteConnections:
		s.noopUpdates(w, r)
	default:
		http.Error(w, "unknown updates route", http.StatusBadRequest)
	}
}

func (s *Server) noopUpdates(w http.ResponseWriter, r *http.Request) {
	_ = pagestream.EnsureClientID(w, r)
	pagestream.ServeStream(w, r, pagestream.StreamSpec{
		InitialPatches: []pagestream.Patch{
			{"status": map[string]any{"loading": false, "error": ""}},
		},
	})
}
