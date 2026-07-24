package app

import (
	"net/http"
	"strings"

	"github.com/Yacobolo/leapview/internal/ui"
	uisignals "github.com/Yacobolo/leapview/internal/ui/signals"
	uitransport "github.com/Yacobolo/leapview/internal/ui/transport"
	"github.com/Yacobolo/leapview/pkg/pagestream"
)

func (s *applicationAssembly) configurePageStream() {
	s.runtime.pageStreams = uitransport.NewPageStream(uitransport.PageStreamConfig{
		Trace: s.runtime.pageStreamTrace,
		Protect: func(privilege string, next http.Handler) http.Handler {
			return s.routes.accessModule.ProtectNamed(privilege, next)
		},
		ProtectGlobal: func(privilege string, next http.Handler) http.Handler {
			return s.routes.accessModule.ProtectGlobalNamed(privilege, next)
		},
		ProtectAnyWorkspace: func(privilege string, next http.Handler) http.Handler {
			return s.routes.accessModule.ProtectAnyWorkspaceNamed(privilege, next)
		},
		Handlers: map[uisignals.RouteKind]http.Handler{
			uisignals.RouteDashboard: http.HandlerFunc(s.routes.dashboardModule.HTTP().Updates),
			uisignals.RouteChat:      http.HandlerFunc(s.routes.agentModule.HTTP().ChatUpdates),
			uisignals.RouteData:      http.HandlerFunc(s.routes.workspaceModule.HTTP().DataExplorerUpdates),
			uisignals.RouteAdmin: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				adminHTTP := s.routes.adminModule.HTTP()
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
				uitransport.PatchAndWait(s.runtime.pageStreamTrace, w, r, ui.LoginBootstrapSignalsForOptions(s.routes.accessModule.LoginPageOptions(r)))
			}),
			uisignals.RouteCatalog: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				uitransport.PatchAndWait(s.runtime.pageStreamTrace, w, r, ui.CatalogBootstrapSignalsForCatalogs(
					s.routes.workspaceModule.CatalogsForVisibleWorkspaces(r), s.routes.agentModule.ChromeOption(r),
				))
			}),
			uisignals.RouteWorkspace:   http.HandlerFunc(s.routes.workspaceModule.HTTP().WorkspaceBootstrapUpdates),
			uisignals.RouteConnections: http.HandlerFunc(s.routes.workspaceModule.HTTP().ConnectionsBootstrapUpdates),
		},
	})
}

func (s *applicationAssembly) workspaceAssetUpdates(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(r.URL.Query().Get("asset")) != "" {
		s.routes.workspaceModule.HTTP().AssetUpdatesStream(w, r)
		return
	}
	uitransport.PatchAndWait(s.runtime.pageStreamTrace, w, r, pagestream.SignalPatch{"status": map[string]any{"loading": false, "error": ""}})
}
