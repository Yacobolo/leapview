package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	adminmodule "github.com/Yacobolo/leapview/internal/admin/module"
	agentmodule "github.com/Yacobolo/leapview/internal/agent/module"
	analyticsmodule "github.com/Yacobolo/leapview/internal/analytics/module"
	"github.com/Yacobolo/leapview/internal/api"
	apiapigenruntime "github.com/Yacobolo/leapview/internal/api/apigenruntime"
	apigenapi "github.com/Yacobolo/leapview/internal/api/gen"
	apihttpmiddleware "github.com/Yacobolo/leapview/internal/api/httpmiddleware"
	apiprotocol "github.com/Yacobolo/leapview/internal/api/protocol"
	"github.com/Yacobolo/leapview/internal/catalog"
	dashboardmodule "github.com/Yacobolo/leapview/internal/dashboard/module"
	deploymentmodule "github.com/Yacobolo/leapview/internal/deployment/module"
	manageddatamodule "github.com/Yacobolo/leapview/internal/manageddata/module"
	"github.com/Yacobolo/leapview/internal/observability"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
	jobsmodule "github.com/Yacobolo/leapview/internal/platform/jobs/module"
	platformlifecycle "github.com/Yacobolo/leapview/internal/platform/lifecycle"
	refreshmodule "github.com/Yacobolo/leapview/internal/refresh/module"
	releasemodule "github.com/Yacobolo/leapview/internal/release/module"
	runtimehostmodule "github.com/Yacobolo/leapview/internal/runtimehost/module"
	servingstatemodule "github.com/Yacobolo/leapview/internal/servingstate/module"
	"github.com/Yacobolo/leapview/internal/staticasset"
	"github.com/Yacobolo/leapview/internal/ui"
	uitransport "github.com/Yacobolo/leapview/internal/ui/transport"
	workloadmodule "github.com/Yacobolo/leapview/internal/workload/module"
	workspacemodule "github.com/Yacobolo/leapview/internal/workspace/module"
	"github.com/Yacobolo/leapview/pkg/pagestream"
)

type QueryMetrics = dashboardmodule.Metrics
type workspaceMetrics = dashboardmodule.WorkspaceMetrics

type runtimeRouter struct {
	accessModule          *accessmodule.Module
	workspaceModule       *workspacemodule.Module
	managedDataModule     *manageddatamodule.Module
	deploymentModule      *deploymentmodule.Module
	dashboardModule       *dashboardmodule.Module
	dashboardAssets       dashboardmodule.Assets
	agentModule           *agentmodule.Module
	releaseModule         *releasemodule.Module
	refreshModule         *refreshmodule.Module
	adminModule           *adminmodule.Module
	analyticsModule       *analyticsmodule.Module
	metrics               QueryMetrics
	workloads             workloadControl
	broker                *pagestream.Broker
	pageStreamTrace       *pagestream.TraceStore
	pageStreams           *uitransport.PageStream
	persistenceConfigured bool
	platformHealth        platformHealth
	storageRetention      *servingstatemodule.Retention
	queryAuditProvider    adminmodule.QueryAuditReaderProvider
	queryAuditEvents      http.HandlerFunc
	asyncJobs             jobs.Repository
	jobModule             *jobsmodule.Module
	auth                  *accessmodule.Auth
	defaultWorkspaceID    string
	defaultEnvironment    string
	scimBearerToken       string
	metricsBearerToken    string
	allowedHosts          []string
	rateLimits            apihttpmiddleware.RateLimitConfig
	securityHeaders       apihttpmiddleware.SecurityHeadersConfig
	requestBodyLimit      apihttpmiddleware.RequestBodyLimitConfig
	requestLogging        bool
	telemetry             *observability.Telemetry
	health                *observability.Health
	logger                *slog.Logger
	workers               *platformlifecycle.Group
	managedDataTus        http.Handler
	apiProtocol           *apiprotocol.Protocol
	apiGenHandler         *apiapigenruntime.Handler
	construction          *capabilityConstruction
}

// capabilityConstruction exists only while app.Build assembles capability
// modules. It is deliberately reachable through one pointer so the completed
// request runtime can discard every repository and adapter-building input in a
// single operation.
type capabilityConstruction struct {
	agentSettings         agentmodule.Settings
	adminDatabase         *sql.DB
	servingStateRepo      servingStateRepository
	managedDataValidation refreshmodule.CandidateValidationHook
	managedDataResolver   runtimehostmodule.ManagedDataResolver
	refreshPipelineClock  refreshmodule.Clock
	workspaceRepo         workspacemodule.Repository
	workspacePersistence  *workspacemodule.Persistence
	workspaceAssetCatalog workspacemodule.AssetCatalogReader
	accessRepo            accessmodule.Repository
	agent                 *agentmodule.Service
	agentConfig           agentmodule.ModelConfig
	reloader              runtimeReloader
	duckLakeCatalogPath   string
	duckLakeDataPath      string
	jobLeaseTimeout       time.Duration
	deploymentConfig      deploymentmodule.Config
	publicURL             string
}

func newRuntimeRouter(metrics QueryMetrics) *runtimeRouter {
	logger := slog.Default()
	var trace *pagestream.TraceStore
	if !staticasset.Production() {
		trace = pagestream.NewTraceStore(pagestream.TraceOptions{
			CapacityPerStream: 512,
			MaxStreams:        32,
			IncludePayloads:   true,
		})
	}
	server := &runtimeRouter{
		metrics:          metrics,
		broker:           pagestream.NewBroker(pagestream.WithTraceStore(trace)),
		pageStreamTrace:  trace,
		requestBodyLimit: apihttpmiddleware.DefaultRequestBodyLimitConfig(),
		telemetry:        observability.New(),
		logger:           logger,
		construction:     &capabilityConstruction{},
	}
	return server
}

type assemblyConfig struct {
	// Database is construction-only. assembleRuntime creates concrete adapters
	// and never retains the connection on the request router.
	Database              *sql.DB
	PlatformHealth        platformHealth
	AgentSettings         agentmodule.Settings
	AdminDatabase         *sql.DB
	ServingStateRepo      servingStateRepository
	StorageRetention      *servingstatemodule.Retention
	ManagedDataValidation refreshmodule.CandidateValidationHook
	ManagedDataResolver   runtimehostmodule.ManagedDataResolver
	WorkspaceRepo         workspacemodule.Repository
	WorkspacePersistence  *workspacemodule.Persistence
	AssetCatalog          workspacemodule.AssetCatalogReader
	ReleaseModule         *releasemodule.Module
	JobModule             *jobsmodule.Module
	AccessRepo            accessmodule.Repository
	AccessModule          *accessmodule.Module
	Agent                 *agentmodule.Service
	AgentConfig           agentmodule.ModelConfig
	Auth                  *accessmodule.Auth
	Reloader              runtimeReloader
	DuckDBDir             string
	DuckLakeCatalogPath   string
	DuckLakeDataPath      string
	DefaultWorkspaceID    string
	DefaultEnvironment    string
	SCIMBearerToken       string
	MetricsBearerToken    string
	AllowedHosts          []string
	RateLimits            apihttpmiddleware.RateLimitConfig
	SecurityHeaders       apihttpmiddleware.SecurityHeadersConfig
	RequestBodyLimit      apihttpmiddleware.RequestBodyLimitConfig
	RequestLogging        bool
	Logger                *slog.Logger
	Workload              workloadControl
	JobLeaseTimeout       time.Duration
	ManagedDataModule     *manageddatamodule.Module
	DeploymentConfig      deploymentmodule.Config
	ManagedDataTus        http.Handler
	MCPOAuth              MCPOAuthConfig
	PublicURL             string
	RefreshPipelineClock  refreshmodule.Clock
	AnalyticsModule       *analyticsmodule.Module
	DashboardAssets       dashboardmodule.Assets
	QueryAudit            *analyticsmodule.QueryAuditSurface
}

type MCPOAuthConfig struct {
	PublicURL string
	IssuerURL string
}

type platformHealth interface {
	Ping(context.Context) error
}

type workloadControl interface {
	workloadmodule.Admitter
	Stats() workloadmodule.Stats
	SetObserver(workloadmodule.Observer)
	Close()
}

func (s *runtimeRouter) AnalyticalFatal() <-chan struct{} {
	if s == nil || s.analyticsModule == nil {
		return nil
	}
	return s.analyticsModule.Fatal()
}

func (s *runtimeRouter) AnalyticalHealth() error {
	if s == nil || s.analyticsModule == nil {
		return nil
	}
	return s.analyticsModule.Healthy()
}

func (s *runtimeRouter) StopWorkloadAdmission() {
	if s != nil && s.workloads != nil {
		s.workloads.Close()
	}
}

// releaseConstructionInputs severs references that are needed only while
// capability modules are being built. Route handlers and lifecycle management
// retain module contracts, never repositories or adapter-construction inputs.
func (s *runtimeRouter) releaseConstructionInputs() {
	if s == nil {
		return
	}
	s.construction = nil
}

func assembleRuntimeChecked(ctx context.Context, metrics QueryMetrics, options assemblyConfig) (*runtimeRouter, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	telemetry := observability.New()
	if options.AnalyticsModule != nil {
		telemetry.Register(options.AnalyticsModule.Collector())
	}
	controller := options.Workload
	ownsController := false
	workloadTelemetry := workloadmodule.NewTelemetryObserver(telemetry)
	if controller == nil {
		var err error
		controller, err = workloadmodule.Build(ctx, workloadmodule.Config{
			Policy: workloadmodule.DefaultConfig(), Observer: workloadTelemetry,
		})
		if err != nil {
			return nil, fmt.Errorf("build workload module: %w", err)
		}
		ownsController = true
	} else {
		controller.SetObserver(workloadTelemetry)
	}
	fail := func(err error) (*runtimeRouter, error) {
		if ownsController && controller != nil {
			controller.Close()
		}
		return nil, err
	}
	if metrics != nil {
		metrics = dashboardmodule.WithAdmission(metrics, controller, options.DefaultWorkspaceID)
	}
	dataAccessRepo := options.AccessRepo
	workspaceRepo := options.WorkspaceRepo
	var dataAuthorization accessmodule.DataAuthorizationService = dataAccessRepo
	if options.AccessModule != nil {
		dataAuthorization = options.AccessModule.DataAuthorizationService()
	}
	if metrics != nil && dataAuthorization != nil && (options.AccessRepo != nil || options.Auth != nil || options.AccessModule != nil) {
		metrics = dashboardmodule.WithQueryAuthorization(metrics, dashboardmodule.QueryAuthorizationConfig{
			Repository:         dataAuthorization,
			DefaultWorkspaceID: options.DefaultWorkspaceID,
			PrincipalFromContext: func(ctx context.Context) (dashboardmodule.QueryPrincipal, bool) {
				principal, ok := accessmodule.PrincipalFromContext(ctx)
				return dashboardmodule.QueryPrincipal{ID: principal.ID, DevBypass: principal.DevBypass || options.Auth == nil}, ok
			},
			CredentialFromContext: accessmodule.APICredentialFromContext,
			TokenAllows:           accessmodule.TokenAllows,
		})
	}
	var queryAuditProvider adminmodule.QueryAuditReaderProvider
	var queryAuditRecorder dashboardmodule.QueryAuditRecorder
	var queryAuditEvents http.HandlerFunc
	if options.QueryAudit != nil {
		queryAuditProvider = adminmodule.QueryAuditReaderProvider(options.QueryAudit.Provider())
		queryAuditRecorder = options.QueryAudit.Recorder()
		queryAuditEvents = options.QueryAudit.Events(func(value string) string { return value })
	}
	if options.AnalyticsModule != nil {
		if options.AnalyticsModule.QueryAuditReader() != nil {
			queryAuditProvider = adminmodule.QueryAuditReaderProvider(options.AnalyticsModule.QueryAuditProvider())
		}
		if options.AnalyticsModule.QueryAuditRecorder() != nil {
			queryAuditRecorder = options.AnalyticsModule.QueryAuditRecorder()
		}
	}
	if metrics != nil && queryAuditRecorder != nil {
		metrics = dashboardmodule.WithQueryAudit(metrics, queryAuditRecorder, options.DefaultWorkspaceID, func(ctx context.Context) (string, bool) {
			principal, ok := accessmodule.PrincipalFromContext(ctx)
			return principal.ID, ok
		})
	}
	servingStateRepo := options.ServingStateRepo
	server := newRuntimeRouter(metrics)
	server.queryAuditEvents = queryAuditEvents
	if server.queryAuditEvents == nil {
		server.queryAuditEvents = analyticsmodule.NewQueryAuditEvents(nil, server.workspaceID)
	}
	if options.AnalyticsModule != nil && options.AnalyticsModule.QueryAuditReader() != nil {
		server.queryAuditEvents = options.AnalyticsModule.QueryAuditEvents(server.workspaceID)
	}
	server.telemetry = telemetry
	server.construction.refreshPipelineClock = options.RefreshPipelineClock
	server.queryAuditProvider = queryAuditProvider
	if server.construction.refreshPipelineClock == nil {
		server.construction.refreshPipelineClock = refreshmodule.NewRealClock()
	}
	server.workloads = controller
	server.persistenceConfigured = options.Database != nil
	server.platformHealth = options.PlatformHealth
	server.construction.agentSettings = options.AgentSettings
	server.construction.adminDatabase = options.AdminDatabase
	if options.Database != nil {
		server.jobModule = options.JobModule
		if server.jobModule == nil {
			var err error
			server.jobModule, err = jobsmodule.Build(ctx, jobsmodule.Config{
				Database: options.Database, Admission: server.workloads,
				LeaseTimeout: options.JobLeaseTimeout, Logger: options.Logger,
			})
			if err != nil {
				return fail(fmt.Errorf("build platform jobs module: %w", err))
			}
		}
		server.asyncJobs = server.jobModule
		if err := server.configureAPIProtocol(ctx, options.Database); err != nil {
			return fail(fmt.Errorf("build API protocol: %w", err))
		}
	}
	if server.apiProtocol == nil {
		if err := server.configureAPIProtocol(ctx, nil); err != nil {
			return fail(fmt.Errorf("build API protocol: %w", err))
		}
	}
	server.construction.servingStateRepo = servingStateRepo
	retentionStates, _ := servingStateRepo.(servingstatemodule.RetentionRepository)
	server.storageRetention = options.StorageRetention
	if server.storageRetention == nil {
		server.storageRetention = servingstatemodule.NewRetention(servingstatemodule.RetentionConfig{
			States: retentionStates, Snapshots: options.AnalyticsModule.RetentionSnapshots(),
			Admission: controller, Environment: options.DefaultEnvironment,
			CatalogPath: options.DuckLakeCatalogPath, DataPath: options.DuckLakeDataPath,
			ProtectedSnapshots: func() []int64 {
				if provider, ok := options.Reloader.(interface{ LeasedSnapshots() []int64 }); ok {
					return provider.LeasedSnapshots()
				}
				return nil
			},
		})
	}
	server.construction.managedDataValidation = options.ManagedDataValidation
	server.construction.managedDataResolver = options.ManagedDataResolver
	server.analyticsModule = options.AnalyticsModule
	server.dashboardAssets = options.DashboardAssets
	server.construction.workspaceRepo = workspaceRepo
	server.construction.workspacePersistence = options.WorkspacePersistence
	server.construction.workspaceAssetCatalog = options.AssetCatalog
	server.releaseModule = options.ReleaseModule
	server.construction.accessRepo = options.AccessRepo
	server.construction.agent = options.Agent
	server.construction.agentConfig = options.AgentConfig
	server.auth = options.Auth
	server.accessModule = options.AccessModule
	server.construction.reloader = options.Reloader
	server.construction.duckLakeCatalogPath = options.DuckLakeCatalogPath
	server.construction.duckLakeDataPath = options.DuckLakeDataPath
	server.defaultWorkspaceID = options.DefaultWorkspaceID
	server.defaultEnvironment = string(servingstatemodule.NormalizeEnvironment(servingstatemodule.Environment(options.DefaultEnvironment)))
	server.construction.publicURL = strings.TrimSuffix(strings.TrimSpace(options.PublicURL), "/")
	server.scimBearerToken = options.SCIMBearerToken
	server.metricsBearerToken = options.MetricsBearerToken
	server.allowedHosts = append([]string(nil), options.AllowedHosts...)
	server.rateLimits = options.RateLimits
	server.securityHeaders = options.SecurityHeaders
	server.requestBodyLimit = options.RequestBodyLimit
	if !server.requestBodyLimit.Enabled && server.requestBodyLimit.MaxBytes == 0 {
		server.requestBodyLimit = apihttpmiddleware.DefaultRequestBodyLimitConfig()
	}
	server.requestLogging = options.RequestLogging
	server.managedDataModule = options.ManagedDataModule
	server.construction.deploymentConfig = options.DeploymentConfig
	server.managedDataTus = options.ManagedDataTus
	server.construction.jobLeaseTimeout = options.JobLeaseTimeout
	if server.construction.jobLeaseTimeout <= 0 {
		server.construction.jobLeaseTimeout = 2 * time.Minute
	}
	if options.Logger != nil {
		server.logger = options.Logger
		if server.pageStreamTrace != nil {
			server.pageStreamTrace.SetLogger(options.Logger)
		}
	}
	if err := server.configureRefreshModule(ctx, options.Database); err != nil {
		return fail(err)
	}
	if err := server.configureModules(ctx, options.Database); err != nil {
		return fail(err)
	}
	if server.asyncJobs != nil {
		handlers := make([]jobs.Handler, 0, 4)
		if server.releaseModule != nil {
			handlers = append(handlers, server.releaseModule.JobHandlers(server.asyncJobs)...)
		}
		if server.deploymentModule != nil {
			handlers = append(handlers, server.deploymentModule.JobHandlers()...)
		}
		if server.managedDataModule != nil && server.managedDataModule.HasFinalizeJobs() {
			handlers = append(handlers, server.managedDataModule.JobHandlers(server.asyncJobs)...)
		}
		if server.agentModule != nil {
			handlers = append(handlers, server.agentModule.JobHandlers(server.asyncJobs)...)
		}
		if err := server.jobModule.RegisterHandlers(handlers); err != nil {
			return fail(fmt.Errorf("register async job handlers: %w", err))
		}
	}
	return server, nil
}

func (s *runtimeRouter) configureModules(ctx context.Context, database *sql.DB) error {
	if s == nil {
		return errors.New("runtime router is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if s.accessModule == nil {
		accessSurface := accessmodule.SurfaceConfig{
			Repository: s.accessRepository, Auth: s.auth,
			DefaultWorkspaceID: s.defaultWorkspaceID,
			WorkspaceIDs: func(ctx context.Context) ([]string, error) {
				repository, err := s.workspaceRepository()
				if err != nil || repository == nil {
					return nil, err
				}
				rows, err := repository.List(ctx)
				if err != nil {
					return nil, err
				}
				ids := make([]string, 0, len(rows))
				for _, row := range rows {
					ids = append(ids, string(row.ID))
				}
				return ids, nil
			},
			CurrentPrincipal: func(r *http.Request) (accessmodule.Principal, bool) {
				principal, ok := s.accessModule.CurrentPrincipal(r)
				return principal, ok
			},
			CurrentCredential: func(r *http.Request) (accessmodule.APICredential, bool) {
				if s.auth == nil {
					return accessmodule.APICredential{}, false
				}
				return s.auth.APICredential(r)
			},
			WorkspaceID: s.workspaceID,
		}
		var err error
		s.accessModule, err = accessmodule.Build(ctx, accessmodule.Config{Surface: &accessSurface})
		if err != nil {
			return fmt.Errorf("build access module: %w", err)
		}
	}
	if s.workspaceModule == nil {
		refreshSupport := s.workspaceRefreshSupport()
		var err error
		s.workspaceModule, err = workspacemodule.Build(ctx, workspacemodule.Config{
			Database:            database,
			Repository:          s.construction.workspaceRepo,
			Persistence:         s.construction.workspacePersistence,
			AccessService:       s.accessModule.WorkspaceAccessService(),
			AssetCatalog:        s.construction.workspaceAssetCatalog,
			WorkspaceID:         s.workspaceID,
			Environment:         func(r *http.Request) string { return string(s.requestServingEnvironment(r)) },
			MetricsForWorkspace: s.metricsForWorkspace,
			RootMetrics:         s.metrics,
			CurrentPrincipal: func(r *http.Request) (workspacemodule.Principal, bool) {
				principal, ok := s.accessModule.CurrentPrincipal(r)
				return workspacemodule.Principal{
					ID: principal.ID, Email: principal.Email,
					DisplayName: principal.DisplayName, DevBypass: principal.DevBypass,
				}, ok
			},
			AuthConfigured:     s.auth != nil,
			RuntimeEnvironment: s.defaultEnvironment,
			DefaultWorkspaceID: s.defaultWorkspaceID,
			RefreshState:       refreshSupport,
			RefreshRunner: workspacemodule.AssetRefreshFunc(func(ctx context.Context, input workspacemodule.AssetRefreshInput) error {
				return refreshSupport.RefreshAsset(ctx, input.Request, input.WorkspaceID, input.Asset, input.Assets, input.Edges)
			}),
			Broker:           s.broker,
			CSRFToken:        s.accessModule.CSRFToken,
			CurrentRoleLabel: s.accessModule.CurrentRoleLabel,
			ChromeOptions: func(r *http.Request) []ui.ChromeOption {
				return []ui.ChromeOption{s.agentModule.ChromeOption(r)}
			},
			CurrentCredential: func(r *http.Request) (accessmodule.APICredential, bool) {
				return accessmodule.APICredentialFromContext(r.Context())
			},
			AuthorizeObject: s.accessModule.AuthorizeObject,
		})
		if err != nil {
			return fmt.Errorf("build workspace module: %w", err)
		}
		s.construction.workspaceAssetCatalog = nil
	}
	if s.deploymentModule == nil {
		config := s.construction.deploymentConfig
		config.Logger = s.logger
		config.InstanceEnvironment = s.defaultEnvironment
		config.CurrentPrincipal = func(r *http.Request) (deploymentmodule.Principal, bool) {
			principal, ok := s.accessModule.CurrentPrincipal(r)
			return deploymentmodule.Principal{ID: principal.ID}, ok
		}
		config.Jobs = deploymentmodule.JobConfig{
			Reconcile: func(ctx context.Context) error {
				if s.refreshModule == nil {
					return nil
				}
				return s.refreshModule.Reconcile(ctx)
			},
			Events: s.asyncJobs,
			Logger: s.logger,
		}
		config.API = deploymentmodule.APIConfig{Releases: s.releaseModule.DeploymentLinkage(), Jobs: s.asyncJobs}
		config.PublicationAuthorization = deploymentmodule.PublicationAuthorizationConfig{
			States: s.construction.servingStateRepo, AuthorizeObject: s.accessModule.AuthorizeObject,
			Bypass: func(actor string) bool {
				return (s.auth == nil || s.auth.DevBypass()) && actor == accessmodule.LocalDeveloperPrincipal().ID
			},
		}
		var err error
		s.deploymentModule, err = deploymentmodule.Build(ctx, config)
		if err != nil {
			return fmt.Errorf("build deployment module: %w", err)
		}
	}
	if s.dashboardModule == nil {
		var err error
		s.dashboardModule, err = dashboardmodule.Build(ctx, dashboardmodule.Config{
			Database: database,
			HTTP: dashboardmodule.HTTPConfig{
				Metrics:             s.metrics,
				MetricsForWorkspace: s.metricsForWorkspace,
				Admission:           s.workloadController(), Broker: s.broker, Logger: s.logger,
				Telemetry: s.telemetry,
				CurrentPrincipalID: func(r *http.Request) string {
					principal, ok := accessmodule.PrincipalFromContext(r.Context())
					if !ok {
						return ""
					}
					return principal.ID
				},
				AuthorizeListObject: s.authorizeListObject,
				CSRFToken:           s.accessModule.CSRFToken,
				ChatChromeSignal:    s.agentModule.ChromeSignal,
				Environment:         func(r *http.Request) string { return string(s.requestServingEnvironment(r)) },
				DataRefreshedAt: func(ctx context.Context, workspaceID, environment, modelID string) string {
					if s.refreshModule == nil {
						return ""
					}
					version, ok, err := s.refreshModule.DataVersion(ctx, workspaceID, environment, modelID)
					if err != nil || !ok {
						return ""
					}
					return version.RefreshedAt.Format(time.RFC3339)
				},
				AgentBootstrap: func(r *http.Request, workspaceID string) ui.ChatViewState {
					return s.agentModule.HTTP().DashboardBootstrap(r, workspaceID)
				},
			},
			Semantic: dashboardmodule.SemanticConfig{
				Metrics:             s.metrics,
				MetricsForWorkspace: s.metricsForWorkspace,
				CurrentPrincipalID: func(r *http.Request) string {
					principal, ok := accessmodule.PrincipalFromContext(r.Context())
					if !ok {
						return ""
					}
					return principal.ID
				},
				AuthorizeListObject: func(ctx context.Context, principalID string, object accessmodule.ObjectRef) (bool, error) {
					return s.authorizeListObject(ctx, principalID, object)
				},
			},
			PublicTelemetry: dashboardmodule.PublicTelemetry{
				DocumentObserved: s.telemetry.PublicDocumentObserved,
				StreamStarted:    s.telemetry.PublicStreamStarted,
				CommandObserved:  s.telemetry.PublicCommandObserved,
			},
			Logger:    s.logger,
			Trace:     s.pageStreamTrace,
			PublicURL: s.construction.publicURL,
			CurrentActor: func(r *http.Request) string {
				principal, ok := accessmodule.PrincipalFromContext(r.Context())
				if !ok {
					return ""
				}
				return principal.ID
			},
			RuntimeMetrics: s.metrics, DefaultWorkspaceID: s.defaultWorkspaceID,
			ServingSnapshot: func(ctx context.Context, workspaceID string) (string, error) {
				if s.workspaceModule == nil {
					return "", nil
				}
				return s.workspaceModule.ActiveServingStateID(ctx, s.workspaceID(workspaceID))
			},
		})
		if err != nil {
			return fmt.Errorf("build dashboard module: %w", err)
		}
	}
	if s.agentModule == nil {
		var err error
		s.agentModule, err = agentmodule.Build(ctx, agentmodule.Config{
			Database: database, Metrics: s.metrics, Model: s.construction.agentConfig,
			Service: s.construction.agent, Jobs: s.asyncJobs, DefaultWorkspaceID: s.defaultWorkspaceID,
			RunWorkloadClass: string(workloadmodule.BackgroundClass), GlobalWorkspaceID: workloadmodule.GlobalWorkspace,
			Search: s.workspaceModule,
			Environment: func(r *http.Request) string {
				return string(s.requestServingEnvironment(r))
			},
			DashboardMetrics:         s.metricsForWorkspace,
			AuthorizeAnyObject:       s.accessModule.AuthorizeAnyObject,
			SkipContextAuthorization: s.auth == nil,
			RecordAudit:              s.accessModule.RecordAudit,
			EnableSystemPrompt:       s.persistenceConfigured,
			Logger:                   s.logger,
			MCPProtect:               s.accessModule.ProtectMCP,
			MCPScope: func(r *http.Request) (agentmodule.Scope, bool) {
				identity, ok := s.accessModule.MCPIdentity(r)
				if !ok {
					return agentmodule.Scope{}, false
				}
				scope := agentmodule.Scope{
					PrincipalID: identity.PrincipalID, DevAuthBypass: identity.DevBypass,
					Credential: agentmodule.CredentialScope{
						WorkspaceID: identity.Credential.Token.WorkspaceID,
						Restricted:  identity.Restricted,
					},
				}
				for _, privilege := range identity.Credential.Token.Privileges {
					scope.Credential.Privileges = append(scope.Credential.Privileges, string(privilege))
				}
				return scope, true
			},
			DispatchAPIGen: func(scope agentmodule.Scope, operationID string, request *http.Request) (*http.Response, bool) {
				principal := accessmodule.Principal{ID: scope.PrincipalID, DevBypass: scope.DevAuthBypass}
				if s.auth == nil {
					principal = accessmodule.LocalDeveloperPrincipal()
				}
				ctx := accessmodule.WithPrincipal(request.Context(), principal)
				if scope.Credential.Restricted || scope.Credential.WorkspaceID != "" || len(scope.Credential.Privileges) > 0 {
					ctx = accessmodule.WithAPICredential(ctx, accessmodule.AgentAPICredential(
						scope.PrincipalID, scope.Credential.WorkspaceID, scope.Credential.Privileges,
					))
				}
				request = request.WithContext(ctx)
				recorder := httptest.NewRecorder()
				if ok := apigenapi.DispatchAPIGenOperation(operationID, apiGenAdapter{server: s}, apiprotocol.TransportErrorResponder{Logger: s.logger}, recorder, request); !ok {
					return nil, false
				}
				return recorder.Result(), true
			},
			HTTP: agentmodule.HTTPConfig{
				Settings: s.construction.agentSettings, Broker: s.broker,
				CSRFToken:        s.accessModule.CSRFToken,
				CurrentRoleLabel: s.accessModule.CurrentRoleLabel,
				CurrentPrincipal: func(r *http.Request) (agentmodule.Principal, bool) {
					if s.auth == nil {
						return agentmodule.Principal{}, false
					}
					principal, ok := s.auth.Principal(r)
					return agentmodule.Principal{ID: principal.ID, DevAuthBypass: principal.DevBypass}, ok
				},
				CurrentCredential: func(r *http.Request) (accessmodule.APICredential, bool) {
					if s.auth == nil {
						return accessmodule.APICredential{}, false
					}
					return s.auth.APICredential(r)
				},
			},
		})
		if err != nil {
			return fmt.Errorf("build agent module: %w", err)
		}
	}
	if s.refreshModule == nil {
		if err := s.configureRefreshModule(ctx, nil); err != nil {
			return err
		}
	}
	if s.adminModule == nil {
		var accessReader adminmodule.AccessReader
		if reader := s.accessModule.AdminReader(); reader != nil {
			accessReader = reader
		}
		currentAdminPrincipal := func(r *http.Request) (adminmodule.Principal, bool) {
			principal, ok := s.accessModule.CurrentPrincipal(r)
			return adminmodule.Principal{
				ID: principal.ID, Email: principal.Email, DisplayName: principal.DisplayName, DevBypass: principal.DevBypass,
			}, ok
		}
		var err error
		s.adminModule, err = adminmodule.Build(ctx, adminmodule.Config{
			Catalog: func() catalog.Catalog {
				return s.metrics.Catalog()
			},
			Access: accessReader,
			AgentDetails: func(ctx context.Context) (api.AdminAgentResponse, error) {
				return s.agentModule.HTTP().AdminDetails(ctx)
			},
			QueryAuditReader: s.queryAuditProvider,
			CSRFToken:        s.accessModule.CSRFToken,
			CurrentPrincipal: currentAdminPrincipal,
			CurrentCredential: func(r *http.Request) (accessmodule.APICredential, bool) {
				if s.auth == nil {
					return accessmodule.APICredential{}, false
				}
				return s.auth.APICredential(r)
			},
			AuthorizeAnyWorkspace: s.accessModule.AuthorizeAnyWorkspace,
			Publications:          s.dashboardModule,
			DefaultWorkspaceID:    s.defaultWorkspaceID,
			AuthConfigured:        s.auth != nil,
			AccessConfigured:      accessReader != nil,
			Storage: adminmodule.StorageConfig{
				CatalogPath: s.construction.duckLakeCatalogPath, DataPath: s.construction.duckLakeDataPath,
				Environment: s.defaultEnvironment, ControlPlane: s.construction.adminDatabase,
				Analytics: s.analyticsModule.AdminResources(), Admitter: s.workloadController(),
			},
			CurrentRoleLabel: func(r *http.Request) string {
				principal, ok := currentAdminPrincipal(r)
				return adminmodule.RoleLabel(s.auth != nil, principal, ok)
			},
			ChromeOption: s.agentModule.ChromeOption,
			EnsureClientID: func(w http.ResponseWriter, r *http.Request) {
				_ = pagestream.EnsureClientID(w, r)
			},
			Broker: s.broker,
		})
		if err != nil {
			return fmt.Errorf("build admin module: %w", err)
		}
	}
	if s.managedDataModule == nil {
		var err error
		s.managedDataModule, err = manageddatamodule.Build(ctx, manageddatamodule.Config{
			Environment: s.defaultEnvironment, Jobs: s.asyncJobs,
			CurrentPrincipal: func(r *http.Request) (manageddatamodule.Principal, bool) {
				if s.auth == nil {
					return manageddatamodule.Principal{}, false
				}
				principal, ok := s.auth.Principal(r)
				return manageddatamodule.Principal{ID: principal.ID}, ok
			},
		})
		if err != nil {
			return fmt.Errorf("build managed data module: %w", err)
		}
	}
	objects, err := s.workspaceModule.SecurableObjects(ctx, s.defaultWorkspaceID)
	if err != nil {
		return fmt.Errorf("resolve workspace securables: %w", err)
	}
	if err := s.accessModule.RegisterSecurables(ctx, objects); err != nil {
		return fmt.Errorf("register workspace securables: %w", err)
	}
	apiGenAuthorizer, err := s.accessModule.APIGenAuthorizer(accessmodule.APIGenObjectResolvers{
		Dashboard:      dashboardmodule.DashboardObjectRefs,
		SemanticModel:  dashboardmodule.SemanticDatasetObjectRefs,
		WorkspaceAsset: workspacemodule.AssetObjectRefs,
	})
	if err != nil {
		return fmt.Errorf("build APIGen authorizer: %w", err)
	}
	s.apiGenHandler, err = apiapigenruntime.Build(
		apiGenAuthorizer,
		apiGenAdapter{server: s},
		apiprotocol.TransportErrorResponder{Logger: s.logger},
	)
	if err != nil {
		return fmt.Errorf("build APIGen transport: %w", err)
	}
	s.configurePageStream()
	s.health = observability.NewHealth(observability.HealthConfig{
		Platform: func(ctx context.Context) error {
			if s.platformHealth == nil {
				return errors.New("platform store is missing")
			}
			return s.platformHealth.Ping(ctx)
		},
		Analytics: func() error {
			if s.analyticsModule == nil {
				return nil
			}
			return s.analyticsModule.Healthy()
		},
		Checks: map[string]func(context.Context) error{
			"mapAssets": func(ctx context.Context) error {
				if s.dashboardAssets == nil {
					return nil
				}
				return s.dashboardAssets.Verify(ctx)
			},
		},
		ActiveWorkspaces: s.workspaceModule.ActiveRuntimeWorkspaces,
		RuntimeReady:     s.dashboardModule.RuntimeReady,
	})
	s.workers = platformlifecycle.New(
		platformlifecycle.Component{Start: s.refreshModule.Start, Stop: s.refreshModule.Stop},
		platformlifecycle.Component{
			Start: func(ctx context.Context) error { s.managedDataModule.Start(ctx); return nil },
			Stop:  s.managedDataModule.Stop,
		},
		platformlifecycle.Component{Start: s.dashboardModule.Start, Stop: s.dashboardModule.Stop},
		platformlifecycle.Component{Start: s.jobModule.Start, Stop: s.jobModule.Stop},
	)
	return nil
}

func (s *runtimeRouter) StartBackgroundJobs(ctx context.Context) error {
	if s == nil || s.workers == nil {
		return nil
	}
	return s.workers.Start(ctx)
}

func (s *runtimeRouter) StopBackgroundJobs(ctx context.Context) error {
	if s == nil || s.workers == nil {
		return nil
	}
	return s.workers.Stop(ctx)
}

func (s *runtimeRouter) workspaceRepository() (workspacemodule.Repository, error) {
	return s.construction.workspaceRepo, nil
}

func (s *runtimeRouter) accessRepository() (accessmodule.Repository, error) {
	return s.construction.accessRepo, nil
}

func (s *runtimeRouter) authorizeListObject(ctx context.Context, principalID string, object accessmodule.ObjectRef) (bool, error) {
	if s.auth == nil {
		return true, nil
	}
	if strings.TrimSpace(principalID) == "" {
		return false, nil
	}
	return s.accessModule.AuthorizeObject(ctx, principalID, accessmodule.PrivilegeViewItem, object)
}

func (s *runtimeRouter) metricsForWorkspace(workspaceID string) (QueryMetrics, bool) {
	if workspaceID == "" {
		return nil, false
	}
	if provider, ok := s.metrics.(workspaceMetrics); ok {
		return provider.MetricsForWorkspace(workspaceID)
	}
	if s.metrics == nil {
		return nil, false
	}
	if s.defaultWorkspaceID != "" && workspaceID == s.defaultWorkspaceID {
		return s.metrics, true
	}
	catalog := s.metrics.Catalog()
	if catalog.Workspace.ID == "" || catalog.Workspace.ID == workspaceID {
		return s.metrics, true
	}
	return nil, false
}
