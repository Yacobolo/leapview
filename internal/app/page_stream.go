package app

import (
	"net/http"
	"strings"

	uitransport "github.com/Yacobolo/leapview/internal/platform/web/transport"
	"github.com/Yacobolo/leapview/internal/workspace/ui"
	uisignals "github.com/Yacobolo/leapview/internal/workspace/ui/signals"
	"github.com/Yacobolo/leapview/pkg/pagestream"
)

func configurePageStream(routes *capabilityRoutes, runtime *runtimeServices, platform *platformServices, policy *httpPolicy) {
	runtime.pageStreams = uitransport.NewPageStream(uitransport.PageStreamConfig{
		Trace: runtime.pageStreamTrace,
		Authorize: func(route, section string, next http.Handler) (http.Handler, bool) {
			switch uisignals.RouteKind(route) {
			case uisignals.RouteLogin:
				return next, true
			case uisignals.RouteCatalog, uisignals.RouteDashboard, uisignals.RouteWorkspace, uisignals.RouteWorkspaceAsset, uisignals.RouteConnections, uisignals.RouteConnectionAsset, uisignals.RouteData:
				return routes.accessModule.ProtectNamed("VIEW_ITEM", next), true
			case uisignals.RouteChat:
				return routes.accessModule.ProtectNamed("VIEW_AGENT", next), true
			case uisignals.RouteAdmin:
				switch strings.TrimSpace(section) {
				case "queries":
					return routes.accessModule.ProtectGlobalNamed("VIEW_AUDIT", next), true
				case "publications":
					return routes.accessModule.ProtectAnyWorkspaceNamed("MANAGE_PUBLICATIONS", next), true
				default:
					return routes.accessModule.ProtectGlobalNamed("MANAGE_GRANTS", next), true
				}
			default:
				return nil, false
			}
		},
		Handlers: map[string]http.Handler{
			string(uisignals.RouteDashboard): http.HandlerFunc(routes.dashboardModule.HTTP().Updates),
			string(uisignals.RouteChat):      http.HandlerFunc(routes.agentModule.HTTP().ChatUpdates),
			string(uisignals.RouteData):      http.HandlerFunc(routes.workspaceModule.HTTP().DataExplorerUpdates),
			string(uisignals.RouteAdmin): http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				adminHTTP := routes.adminModule.HTTP()
				switch strings.TrimSpace(r.URL.Query().Get("section")) {
				case "queries":
					adminHTTP.QueryUpdates(w, r)
				case "storage":
					adminHTTP.StorageSignalUpdates(w, r)
				default:
					adminHTTP.BootstrapUpdates(w, r)
				}
			}),
			string(uisignals.RouteWorkspaceAsset): http.HandlerFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				workspaceAssetUpdates(routes, runtime, platform, policy, w, r)
			})),
			string(uisignals.RouteConnectionAsset): http.HandlerFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				workspaceAssetUpdates(routes, runtime, platform, policy, w, r)
			})),
			string(uisignals.RouteLogin): http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				uitransport.PatchAndWait(runtime.pageStreamTrace, w, r, routes.accessModule.LoginBootstrapSignals(r))
			}),
			string(uisignals.RouteCatalog): http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				uitransport.PatchAndWait(runtime.pageStreamTrace, w, r, ui.CatalogBootstrapSignalsForCatalogs(
					routes.workspaceModule.CatalogsForVisibleWorkspaces(r), routes.agentModule.ChromeOption(r),
				))
			}),
			string(uisignals.RouteWorkspace):   http.HandlerFunc(routes.workspaceModule.HTTP().WorkspaceBootstrapUpdates),
			string(uisignals.RouteConnections): http.HandlerFunc(routes.workspaceModule.HTTP().ConnectionsBootstrapUpdates),
		},
	})
}

func workspaceAssetUpdates(routes *capabilityRoutes, runtime *runtimeServices, platform *platformServices, policy *httpPolicy, w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(r.URL.Query().Get("asset")) != "" {
		routes.workspaceModule.HTTP().AssetUpdatesStream(w, r)
		return
	}
	uitransport.PatchAndWait(runtime.pageStreamTrace, w, r, pagestream.SignalPatch{"status": map[string]any{"loading": false, "error": ""}})
}
