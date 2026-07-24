package transport

import (
	"net/http"
	"strings"

	uisignals "github.com/Yacobolo/leapview/internal/ui/signals"
	"github.com/Yacobolo/leapview/pkg/pagestream"
)

type Protect func(privilege string, next http.Handler) http.Handler

type PageStreamConfig struct {
	Trace               *pagestream.TraceStore
	Handlers            map[uisignals.RouteKind]http.Handler
	Protect             Protect
	ProtectGlobal       Protect
	ProtectAnyWorkspace Protect
}

type PageStream struct {
	config PageStreamConfig
}

func NewPageStream(config PageStreamConfig) *PageStream {
	return &PageStream{config: config}
}

func (s *PageStream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := Route(r)
	if route == "" {
		http.Error(w, "updates route is required", http.StatusBadRequest)
		return
	}
	privilege, ok := StreamPrivilege(route, r.URL.Query().Get("section"))
	if !ok {
		http.Error(w, "unknown updates route", http.StatusBadRequest)
		return
	}
	handler := s.config.Handlers[uisignals.RouteKind(route)]
	if handler == nil {
		http.Error(w, "unknown updates route", http.StatusBadRequest)
		return
	}
	if privilege == "" {
		handler.ServeHTTP(w, r)
		return
	}
	protect := s.config.Protect
	if uisignals.RouteKind(route) == uisignals.RouteAdmin {
		if strings.TrimSpace(r.URL.Query().Get("section")) == "publications" {
			protect = s.config.ProtectAnyWorkspace
		} else {
			protect = s.config.ProtectGlobal
		}
	}
	if protect == nil {
		http.Error(w, "updates authorization is not configured", http.StatusInternalServerError)
		return
	}
	protect(privilege, handler).ServeHTTP(w, r)
}

func Route(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.URL.Query().Get("route"))
}

func StreamPrivilege(route, section string) (string, bool) {
	switch uisignals.RouteKind(route) {
	case uisignals.RouteLogin:
		return "", true
	case uisignals.RouteCatalog, uisignals.RouteDashboard, uisignals.RouteWorkspace, uisignals.RouteWorkspaceAsset, uisignals.RouteConnections, uisignals.RouteConnectionAsset, uisignals.RouteData:
		return "VIEW_ITEM", true
	case uisignals.RouteChat:
		return "VIEW_AGENT", true
	case uisignals.RouteAdmin:
		if strings.TrimSpace(section) == "queries" {
			return "VIEW_AUDIT", true
		}
		if strings.TrimSpace(section) == "publications" {
			return "MANAGE_PUBLICATIONS", true
		}
		return "MANAGE_GRANTS", true
	default:
		return "", false
	}
}

func PatchAndWait(trace *pagestream.TraceStore, w http.ResponseWriter, r *http.Request, patch pagestream.SignalPatch) {
	clientID := pagestream.EnsureClientID(w, r)
	route := Route(r)
	streamID := route + ":" + clientID
	updates := pagestream.NewSignalStream(w, r, pagestream.WithStreamTrace(trace, streamID, route+".bootstrap"))
	if err := updates.Patch(patch); err != nil {
		return
	}
	updates.Wait(r.Context())
}
