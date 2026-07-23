package app

import (
	"net/http"
	"sort"
	"strings"
	"time"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	adminmodule "github.com/Yacobolo/leapview/internal/admin/module"
	agentmodule "github.com/Yacobolo/leapview/internal/agent/module"
	apihttpmiddleware "github.com/Yacobolo/leapview/internal/api/httpmiddleware"
	apiprotocol "github.com/Yacobolo/leapview/internal/api/protocol"
	apitransport "github.com/Yacobolo/leapview/internal/api/transport"
	dashboardmodule "github.com/Yacobolo/leapview/internal/dashboard/module"
	"github.com/Yacobolo/leapview/internal/staticasset"
	uitransport "github.com/Yacobolo/leapview/internal/ui/transport"
	workspacemodule "github.com/Yacobolo/leapview/internal/workspace/module"
	"github.com/go-chi/chi/v5"
)

func (s *runtimeRouter) Routes() http.Handler {
	s.configureModules()
	mux := chi.NewRouter()
	if s.requestLogging {
		mux.Use(apihttpmiddleware.RequestLogger(s.logger))
	}
	mux.Use(s.telemetry.Middleware)
	mux.Use(apihttpmiddleware.PanicRecovery(s.logger))
	mux.Use(apihttpmiddleware.SecurityHeadersMiddleware(s.securityHeaders))
	mux.Use(apihttpmiddleware.AllowedHosts(s.allowedHosts))
	mux.Use(apihttpmiddleware.RequestBodyLimit(s.requestBodyLimit))
	mux.Get("/favicon.ico", favicon)
	mux.Get("/healthz", s.health.Healthz)
	mux.Get("/readyz", s.health.Readyz)
	mux.Get("/api/openapi.json", s.openAPIDescription)
	mux.Get("/api/docs", s.publicDocs)
	mux.Group(func(r chi.Router) {
		r.Use(s.rateLimits.PublicPage(func() { s.telemetry.PublicRateLimitObserved("page") }))
		s.dashboardModule.MountPublicDocuments(r)
	})
	mux.Group(func(r chi.Router) {
		r.Use(s.rateLimits.PublicCommand(func() { s.telemetry.PublicRateLimitObserved("command") }))
		s.dashboardModule.MountPublicCommands(r)
	})
	s.dashboardModule.MountPublicStream(mux.With(s.rateLimits.PublicStream(func() { s.telemetry.PublicRateLimitObserved("stream") })))
	if s.pageStreamTrace != nil {
		traceHandler := uitransport.TraceHandler{Store: s.pageStreamTrace}
		mux.Get("/__dev/pagestream/traces", traceHandler.Traces)
		mux.Get("/__dev/pagestream/signals", traceHandler.Signals)
	}
	mux.With(s.rateLimits.Auth()).Handle("/metrics", s.telemetry.MetricsHandler(s.metricsBearerToken, accessmodule.BearerToken))
	mux.With(s.csrf).Group(s.accessModule.MountLoginPage)
	mux.Group(func(r chi.Router) {
		r.Use(s.csrf)
		r.With(s.rateLimits.Updates()).Get("/updates", s.pageStreams.ServeHTTP)
		r.Get("/", s.accessModule.ProtectViewItem(s.workspaceModule.Home))
		s.workspaceModule.MountAuthenticated(r, workspacemodule.RouteGuard{
			Protect: s.accessModule.Protect, ProtectWithObjects: s.accessModule.ProtectWithObjects, AssetObjectRefs: s.workspaceModule.AssetObjectRefs,
		})
		s.agentModule.MountAuthenticated(r, agentmodule.RouteGuard{
			Protect: s.accessModule.Protect, ProtectGlobal: s.accessModule.ProtectGlobal,
		})
		r.Get("/chat", redirectLegacyChat)
		r.Get("/chat/updates", http.NotFound)
		r.Get("/chat/*", redirectLegacyChat)
		r.Post("/chat/turns", redirectLegacyChat)
		s.adminModule.MountAuthenticated(r, adminmodule.RouteGuard{
			Protect: s.accessModule.Protect, ProtectGlobal: s.accessModule.ProtectGlobal,
			ProtectAnyWorkspace: s.accessModule.ProtectAnyWorkspace,
		})
		s.dashboardModule.MountAuthenticated(r, dashboardmodule.RouteGuard{
			Protect: s.accessModule.Protect, ProtectWithObjects: s.accessModule.ProtectWithObjects,
		})
		s.accessModule.MountAuthenticatedBrowser(r)
	})
	mux.Group(func(r chi.Router) {
		r.Use(s.rateLimits.Auth())
		r.Use(s.csrf)
		s.accessModule.MountLocalLogin(r)
	})
	mux.Group(func(r chi.Router) {
		r.Use(s.rateLimits.Auth())
		s.accessModule.MountOAuthEndpoints(r)
	})
	s.accessModule.MountOAuthMetadata(mux)
	if s.persistenceConfigured {
		if s.auth != nil {
			mux.With(s.rateLimits.API()).Handle("/mcp", s.agentModule.MCPHandler())
		}
		if strings.TrimSpace(s.scimBearerToken) != "" {
			if handler, err := s.accessModule.SCIMHandler(s.scimBearerToken); err == nil {
				scimHandler := s.rateLimits.API()(http.StripPrefix("/scim", handler))
				mux.Handle("/scim/*", scimHandler)
			}
		}
		mux.Group(func(r chi.Router) {
			r.Use(s.rateLimits.API())
			r.Use(s.publicProtocolMiddleware)
			if s.managedDataTus != nil {
				tus := s.accessModule.ProtectIngestData(managedDataTusHandler(s.managedDataTus))
				r.Handle("/upload-protocols/tus", tus)
				r.Handle("/upload-protocols/tus/*", tus)
			}
			s.registerAPIGenRoutes(r)
		})
	}
	if s.dashboardAssets != nil {
		mux.Handle("/map-assets/*", s.dashboardAssets.Handler())
	}
	mux.Handle("/static/*", staticAssetCache(http.StripPrefix("/static/", http.FileServer(http.Dir("static")))))
	mux.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if isPublicAPIPath(r.URL.Path) {
			apiprotocol.PrepareRequest(w, r)
			apitransport.WriteProblem(w, r, http.StatusNotFound, "API_ROUTE_NOT_FOUND", "The requested API route does not exist", nil)
			return
		}
		http.NotFound(w, r)
	})
	registeredMethods := registeredRouteMethods(mux)
	mux.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		setAllowedMethods(w.Header(), mux, registeredMethods, r.URL.Path)
		if isPublicAPIPath(r.URL.Path) {
			if s.apiProtocol.Authenticate(w, r) {
				apitransport.WriteProblem(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "The requested method is not supported for this API route", nil)
			}
			return
		}
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	})

	return mux
}

func registeredRouteMethods(routes chi.Routes) []string {
	registered := make(map[string]struct{})
	_ = chi.Walk(routes, func(method, _ string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method != "*" {
			registered[method] = struct{}{}
		}
		return nil
	})
	methods := make([]string, 0, len(registered))
	for method := range registered {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func setAllowedMethods(header http.Header, routes chi.Routes, methods []string, path string) {
	for _, method := range methods {
		if routes.Match(chi.NewRouteContext(), method, path) {
			header.Add("Allow", method)
		}
	}
}

func isPublicAPIPath(path string) bool {
	return path == "/api/v1" || strings.HasPrefix(path, "/api/v1/") || path == "/upload-protocols" || strings.HasPrefix(path, "/upload-protocols/")
}

func redirectLegacyChat(w http.ResponseWriter, r *http.Request) {
	target := "/chats" + strings.TrimPrefix(r.URL.Path, "/chat")
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusPermanentRedirect)
}

func managedDataTusHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			// Authentication and headers have already completed. The upload
			// session TTL bounds abandoned bodies, while large chunks must not
			// inherit the general page/API read deadline.
			_ = http.NewResponseController(w).SetReadDeadline(time.Time{})
			next.ServeHTTP(w, r)
		case http.MethodOptions, http.MethodHead, http.MethodDelete:
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Allow", "OPTIONS, HEAD, PATCH, DELETE")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	})
}

func (s *runtimeRouter) protectGlobalAgent(privilege accessmodule.Privilege, next http.Handler) http.Handler {
	return s.accessModule.ProtectGlobal(privilege, next.ServeHTTP)
}

func (s *runtimeRouter) protectAnyWorkspace(privilege accessmodule.Privilege, next http.Handler) http.Handler {
	return s.accessModule.ProtectAnyWorkspace(privilege, next.ServeHTTP)
}

func (s *runtimeRouter) protect(privilege accessmodule.Privilege, next http.Handler) http.Handler {
	return s.accessModule.ProtectHandler(privilege, next)
}

func (s *runtimeRouter) protectGlobal(privilege accessmodule.Privilege, next http.Handler) http.Handler {
	return s.accessModule.ProtectGlobal(privilege, next.ServeHTTP)
}

func (s *runtimeRouter) protectWithObjects(privilege accessmodule.Privilege, objectResolver accessmodule.ObjectResolver, next http.Handler) http.Handler {
	return s.accessModule.ProtectHandlerWithObjects(privilege, objectResolver, next)
}

func (s *runtimeRouter) csrf(next http.Handler) http.Handler {
	return s.accessModule.CSRFMiddleware(next)
}

func staticAssetCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version := staticasset.Version()
		switch {
		case version != "dev" && r.URL.Query().Get("v") == version:
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case immutableStaticPath(r.URL.Path):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case fontStaticPath(r.URL.Path):
			w.Header().Set("Cache-Control", "public, max-age=86400")
		default:
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func immutableStaticPath(path string) bool {
	return strings.HasPrefix(path, "/static/chunks/")
}

func fontStaticPath(path string) bool {
	return strings.HasPrefix(path, "/static/files/") && strings.HasSuffix(path, ".woff2")
}

func favicon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="#0969da"/><path d="M8 22h16v3H8zm1-5h4v4H9zm5-7h4v11h-4zm5 4h4v7h-4z" fill="#fff"/></svg>`))
}
