package app

import (
	"net/http"
	"strings"

	"github.com/Yacobolo/leapview/internal/ui"
	uisignals "github.com/Yacobolo/leapview/internal/ui/signals"
	uitransport "github.com/Yacobolo/leapview/internal/ui/transport"
	"github.com/Yacobolo/leapview/pkg/pagestream"
)

func (s *runtimeRouter) configurePageStream() {
	s.pageStreams = uitransport.NewPageStream(uitransport.PageStreamConfig{
		Trace: s.pageStreamTrace,
		Protect: func(privilege string, next http.Handler) http.Handler {
			return s.accessModule.ProtectNamed(privilege, next)
		},
		ProtectGlobal: func(privilege string, next http.Handler) http.Handler {
			return s.accessModule.ProtectGlobalNamed(privilege, next)
		},
		ProtectAnyWorkspace: func(privilege string, next http.Handler) http.Handler {
			return s.accessModule.ProtectAnyWorkspaceNamed(privilege, next)
		},
		Handlers: map[uisignals.RouteKind]http.Handler{
			uisignals.RouteDashboard: http.HandlerFunc(s.dashboardModule.HTTP().Updates),
			uisignals.RouteChat:      http.HandlerFunc(s.agentModule.HTTP().ChatUpdates),
			uisignals.RouteData:      http.HandlerFunc(s.workspaceModule.HTTP().DataExplorerUpdates),
			uisignals.RouteAdmin: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				adminHTTP := s.adminModule.HTTP()
				switch strings.TrimSpace(r.URL.Query().Get("section")) {
				case "queries":
					adminHTTP.QueryUpdates(w, r)
				case "storage":
					adminHTTP.StorageSignalUpdates(w, r)
				default:
					adminHTTP.BootstrapUpdates(w, r)
				}
			}),
			uisignals.RouteWorkspaceAsset:  http.HandlerFunc(s.workspaceAssetUpdates),
			uisignals.RouteConnectionAsset: http.HandlerFunc(s.workspaceAssetUpdates),
			uisignals.RouteLogin: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				uitransport.PatchAndWait(s.pageStreamTrace, w, r, ui.LoginBootstrapSignalsForOptions(s.accessModule.LoginPageOptions(r)))
			}),
			uisignals.RouteCatalog: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				uitransport.PatchAndWait(s.pageStreamTrace, w, r, ui.CatalogBootstrapSignalsForCatalogs(
					s.workspaceModule.CatalogsForVisibleWorkspaces(r), s.agentModule.ChromeOption(r),
				))
			}),
			uisignals.RouteWorkspace:   http.HandlerFunc(s.workspaceModule.HTTP().WorkspaceBootstrapUpdates),
			uisignals.RouteConnections: http.HandlerFunc(s.workspaceModule.HTTP().ConnectionsBootstrapUpdates),
		},
	})
}

func (s *runtimeRouter) workspaceAssetUpdates(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(r.URL.Query().Get("asset")) != "" {
		s.workspaceModule.HTTP().AssetUpdatesStream(w, r)
		return
	}
	uitransport.PatchAndWait(s.pageStreamTrace, w, r, pagestream.SignalPatch{"status": map[string]any{"loading": false, "error": ""}})
}
