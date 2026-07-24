package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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

type applicationAssembly struct {
	routes   capabilityRoutes
	runtime  runtimeServices
	platform platformServices
	policy   httpPolicy
}

type capabilityRoutes struct {
	accessModule      *accessmodule.Module
	workspaceModule   *workspacemodule.Module
	managedDataModule *manageddatamodule.Module
	deploymentModule  *deploymentmodule.Module
	dashboardModule   *dashboardmodule.Module
	dashboardAssets   dashboardmodule.Assets
	agentModule       *agentmodule.Module
	releaseModule     *releasemodule.Module
	refreshModule     *refreshmodule.Module
	adminModule       *adminmodule.Module
}

type runtimeServices struct {
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
}

type platformServices struct {
	asyncJobs     jobs.Repository
	jobModule     *jobsmodule.Module
	auth          *accessmodule.Auth
	telemetry     *observability.Telemetry
	health        *observability.Health
	logger        *slog.Logger
	workers       *platformlifecycle.Group
	apiProtocol   *apiprotocol.Protocol
	apiGenHandler *apiapigenruntime.Handler
}

type httpPolicy struct {
	defaultWorkspaceID string
	defaultEnvironment string
	scimBearerToken    string
	metricsBearerToken string
	allowedHosts       []string
	rateLimits         apihttpmiddleware.RateLimitConfig
	securityHeaders    apihttpmiddleware.SecurityHeadersConfig
	requestBodyLimit   apihttpmiddleware.RequestBodyLimitConfig
	requestLogging     bool
	managedDataTus     http.Handler
}

type moduleAssemblyInputs struct {
	persistence persistenceInputs
	workflow    workflowInputs
	storage     storageInputs
}

type persistenceInputs struct {
	agentSettings         agentmodule.Settings
	adminDatabase         *sql.DB
	servingStateRepo      servingStateRepository
	workspaceReadModel    workspacemodule.ReadModel
	workspaceDirectory    workspacemodule.Directory
	workspaceAssetCatalog workspacemodule.AssetCatalogReader
	accessRepo            accessmodule.Repository
}

type workflowInputs struct {
	managedDataValidation refreshmodule.CandidateValidationHook
	managedDataResolver   runtimehostmodule.ManagedDataResolver
	refreshPipelineClock  refreshmodule.Clock
	agent                 *agentmodule.Service
	agentConfig           agentmodule.ModelConfig
	reloader              runtimeReloader
	deploymentConfig      deploymentmodule.Config
}

type storageInputs struct {
	duckLakeCatalogPath string
	duckLakeDataPath    string
	jobLeaseTimeout     time.Duration
	publicURL           string
}

func newHTTPAssembly(metrics QueryMetrics) *applicationAssembly {
	logger := slog.Default()
	var trace *pagestream.TraceStore
	if !staticasset.Production() {
		trace = pagestream.NewTraceStore(pagestream.TraceOptions{
			CapacityPerStream: 512,
			MaxStreams:        32,
			IncludePayloads:   true,
		})
	}
	server := &applicationAssembly{
		runtime: runtimeServices{
			metrics: metrics, broker: pagestream.NewBroker(pagestream.WithTraceStore(trace)),
			pageStreamTrace: trace,
		},
		platform: platformServices{telemetry: observability.New(), logger: logger},
		policy:   httpPolicy{requestBodyLimit: apihttpmiddleware.DefaultRequestBodyLimitConfig()},
	}
	return server
}

type assemblyInputs struct {
	data         dataAssemblyInputs
	capabilities capabilityAssemblyInputs
	workflow     workflowAssemblyInputs
	runtime      runtimeAssemblyInputs
	http         httpAssemblyInputs
}

type dataAssemblyInputs struct {
	Database           *sql.DB
	PlatformHealth     platformHealth
	AdminDatabase      *sql.DB
	ServingStateRepo   servingStateRepository
	StorageRetention   *servingstatemodule.Retention
	WorkspaceReadModel workspacemodule.ReadModel
	WorkspaceDirectory workspacemodule.Directory
	AssetCatalog       workspacemodule.AssetCatalogReader
	AccessRepo         accessmodule.Repository
}

type capabilityAssemblyInputs struct {
	ReleaseModule     *releasemodule.Module
	JobModule         *jobsmodule.Module
	AccessModule      *accessmodule.Module
	Agent             *agentmodule.Service
	ManagedDataModule *manageddatamodule.Module
	AnalyticsModule   *analyticsmodule.Module
	DashboardAssets   dashboardmodule.Assets
}

type workflowAssemblyInputs struct {
	AgentSettings         agentmodule.Settings
	ManagedDataValidation refreshmodule.CandidateValidationHook
	ManagedDataResolver   runtimehostmodule.ManagedDataResolver
	AgentConfig           agentmodule.ModelConfig
	Auth                  *accessmodule.Auth
	Reloader              runtimeReloader
	Workload              workloadControl
	DeploymentConfig      deploymentmodule.Config
	RefreshPipelineClock  refreshmodule.Clock
	QueryAudit            *analyticsmodule.QueryAuditSurface
}

type runtimeAssemblyInputs struct {
	DuckDBDir           string
	DuckLakeCatalogPath string
	DuckLakeDataPath    string
	DefaultWorkspaceID  string
	DefaultEnvironment  string
	SCIMBearerToken     string
	MetricsBearerToken  string
	AllowedHosts        []string
}

type httpAssemblyInputs struct {
	RateLimits       apihttpmiddleware.RateLimitConfig
	SecurityHeaders  apihttpmiddleware.SecurityHeadersConfig
	RequestBodyLimit apihttpmiddleware.RequestBodyLimitConfig
	RequestLogging   bool
	Logger           *slog.Logger
	JobLeaseTimeout  time.Duration
	ManagedDataTus   http.Handler
	MCPOAuth         MCPOAuthConfig
	PublicURL        string
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

func (s *applicationAssembly) AnalyticalFatal() <-chan struct{} {
	if s == nil || s.runtime.analyticsModule == nil {
		return nil
	}
	return s.runtime.analyticsModule.Fatal()
}

func (s *applicationAssembly) AnalyticalHealth() error {
	if s == nil || s.runtime.analyticsModule == nil {
		return nil
	}
	return s.runtime.analyticsModule.Healthy()
}

func (s *applicationAssembly) StopWorkloadAdmission() {
	if s != nil && s.runtime.workloads != nil {
		s.runtime.workloads.Close()
	}
}

func buildApplicationAssembly(ctx context.Context, metrics QueryMetrics, options assemblyInputs) (*applicationAssembly, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	telemetry := observability.New()
	if options.capabilities.AnalyticsModule != nil {
		telemetry.Register(options.capabilities.AnalyticsModule.Collector())
	}
	controller := options.workflow.Workload
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
	fail := func(err error) (*applicationAssembly, error) {
		if ownsController && controller != nil {
			controller.Close()
		}
		return nil, err
	}
	if metrics != nil {
		metrics = dashboardmodule.WithAdmission(metrics, controller, options.runtime.DefaultWorkspaceID)
	}
	dataAccessRepo := options.data.AccessRepo
	workspaceReadModel := options.data.WorkspaceReadModel
	var dataAuthorization accessmodule.DataAuthorizationService = dataAccessRepo
	if options.capabilities.AccessModule != nil {
		dataAuthorization = options.capabilities.AccessModule.DataAuthorizationService()
	}
	if metrics != nil && dataAuthorization != nil && (options.data.AccessRepo != nil || options.workflow.Auth != nil || options.capabilities.AccessModule != nil) {
		metrics = dashboardmodule.WithQueryAuthorization(metrics, dashboardmodule.QueryAuthorizationConfig{
			Repository:         dataAuthorization,
			DefaultWorkspaceID: options.runtime.DefaultWorkspaceID,
			PrincipalFromContext: func(ctx context.Context) (dashboardmodule.QueryPrincipal, bool) {
				principal, ok := accessmodule.PrincipalFromContext(ctx)
				return dashboardmodule.QueryPrincipal{ID: principal.ID, DevBypass: principal.DevBypass || options.workflow.Auth == nil}, ok
			},
			CredentialFromContext: accessmodule.APICredentialFromContext,
			TokenAllows:           accessmodule.TokenAllows,
		})
	}
	var queryAuditProvider adminmodule.QueryAuditReaderProvider
	var queryAuditRecorder dashboardmodule.QueryAuditRecorder
	var queryAuditEvents http.HandlerFunc
	if options.workflow.QueryAudit != nil {
		queryAuditProvider = adminmodule.QueryAuditReaderProvider(options.workflow.QueryAudit.Provider())
		queryAuditRecorder = options.workflow.QueryAudit.Recorder()
		queryAuditEvents = options.workflow.QueryAudit.Events(func(value string) string { return value })
	}
	if options.capabilities.AnalyticsModule != nil {
		if options.capabilities.AnalyticsModule.QueryAuditReader() != nil {
			queryAuditProvider = adminmodule.QueryAuditReaderProvider(options.capabilities.AnalyticsModule.QueryAuditProvider())
		}
		if options.capabilities.AnalyticsModule.QueryAuditRecorder() != nil {
			queryAuditRecorder = options.capabilities.AnalyticsModule.QueryAuditRecorder()
		}
	}
	if metrics != nil && queryAuditRecorder != nil {
		metrics = dashboardmodule.WithQueryAudit(metrics, queryAuditRecorder, options.runtime.DefaultWorkspaceID, func(ctx context.Context) (string, bool) {
			principal, ok := accessmodule.PrincipalFromContext(ctx)
			return principal.ID, ok
		})
	}
	servingStateRepo := options.data.ServingStateRepo
	server := newHTTPAssembly(metrics)
	inputs := moduleAssemblyInputs{}
	server.runtime.queryAuditEvents = queryAuditEvents
	if server.runtime.queryAuditEvents == nil {
		server.runtime.queryAuditEvents = analyticsmodule.NewQueryAuditEvents(nil, server.workspaceID)
	}
	if options.capabilities.AnalyticsModule != nil && options.capabilities.AnalyticsModule.QueryAuditReader() != nil {
		server.runtime.queryAuditEvents = options.capabilities.AnalyticsModule.QueryAuditEvents(server.workspaceID)
	}
	server.platform.telemetry = telemetry
	inputs.workflow.refreshPipelineClock = options.workflow.RefreshPipelineClock
	server.runtime.queryAuditProvider = queryAuditProvider
	if inputs.workflow.refreshPipelineClock == nil {
		inputs.workflow.refreshPipelineClock = refreshmodule.NewRealClock()
	}
	server.runtime.workloads = controller
	server.runtime.persistenceConfigured = options.data.Database != nil
	server.runtime.platformHealth = options.data.PlatformHealth
	inputs.persistence.agentSettings = options.workflow.AgentSettings
	inputs.persistence.adminDatabase = options.data.AdminDatabase
	if options.data.Database != nil {
		server.platform.jobModule = options.capabilities.JobModule
		if server.platform.jobModule == nil {
			var err error
			server.platform.jobModule, err = jobsmodule.Build(ctx, jobsmodule.Config{
				Database: options.data.Database, Admission: server.runtime.workloads,
				LeaseTimeout: options.http.JobLeaseTimeout, Logger: options.http.Logger,
			})
			if err != nil {
				return fail(fmt.Errorf("build platform jobs module: %w", err))
			}
		}
		server.platform.asyncJobs = server.platform.jobModule
		if err := server.configureAPIProtocol(ctx, options.data.Database); err != nil {
			return fail(fmt.Errorf("build API protocol: %w", err))
		}
	}
	if server.platform.apiProtocol == nil {
		if err := server.configureAPIProtocol(ctx, nil); err != nil {
			return fail(fmt.Errorf("build API protocol: %w", err))
		}
	}
	inputs.persistence.servingStateRepo = servingStateRepo
	retentionStates, _ := servingStateRepo.(servingstatemodule.RetentionRepository)
	server.runtime.storageRetention = options.data.StorageRetention
	if server.runtime.storageRetention == nil {
		server.runtime.storageRetention = servingstatemodule.NewRetention(servingstatemodule.RetentionConfig{
			States: retentionStates, Snapshots: options.capabilities.AnalyticsModule.RetentionSnapshots(),
			Admission: controller, Environment: options.runtime.DefaultEnvironment,
			CatalogPath: options.runtime.DuckLakeCatalogPath, DataPath: options.runtime.DuckLakeDataPath,
			ProtectedSnapshots: func() []int64 {
				if provider, ok := options.workflow.Reloader.(interface{ LeasedSnapshots() []int64 }); ok {
					return provider.LeasedSnapshots()
				}
				return nil
			},
		})
	}
	inputs.workflow.managedDataValidation = options.workflow.ManagedDataValidation
	inputs.workflow.managedDataResolver = options.workflow.ManagedDataResolver
	server.runtime.analyticsModule = options.capabilities.AnalyticsModule
	server.routes.dashboardAssets = options.capabilities.DashboardAssets
	inputs.persistence.workspaceReadModel = workspaceReadModel
	inputs.persistence.workspaceDirectory = options.data.WorkspaceDirectory
	inputs.persistence.workspaceAssetCatalog = options.data.AssetCatalog
	server.routes.releaseModule = options.capabilities.ReleaseModule
	inputs.persistence.accessRepo = options.data.AccessRepo
	inputs.workflow.agent = options.capabilities.Agent
	inputs.workflow.agentConfig = options.workflow.AgentConfig
	server.platform.auth = options.workflow.Auth
	server.routes.accessModule = options.capabilities.AccessModule
	inputs.workflow.reloader = options.workflow.Reloader
	inputs.storage.duckLakeCatalogPath = options.runtime.DuckLakeCatalogPath
	inputs.storage.duckLakeDataPath = options.runtime.DuckLakeDataPath
	server.policy.defaultWorkspaceID = options.runtime.DefaultWorkspaceID
	server.policy.defaultEnvironment = string(servingstatemodule.NormalizeEnvironment(servingstatemodule.Environment(options.runtime.DefaultEnvironment)))
	inputs.storage.publicURL = strings.TrimSuffix(strings.TrimSpace(options.http.PublicURL), "/")
	server.policy.scimBearerToken = options.runtime.SCIMBearerToken
	server.policy.metricsBearerToken = options.runtime.MetricsBearerToken
	server.policy.allowedHosts = append([]string(nil), options.runtime.AllowedHosts...)
	server.policy.rateLimits = options.http.RateLimits
	server.policy.securityHeaders = options.http.SecurityHeaders
	server.policy.requestBodyLimit = options.http.RequestBodyLimit
	if !server.policy.requestBodyLimit.Enabled && server.policy.requestBodyLimit.MaxBytes == 0 {
		server.policy.requestBodyLimit = apihttpmiddleware.DefaultRequestBodyLimitConfig()
	}
	server.policy.requestLogging = options.http.RequestLogging
	server.routes.managedDataModule = options.capabilities.ManagedDataModule
	inputs.workflow.deploymentConfig = options.workflow.DeploymentConfig
	server.policy.managedDataTus = options.http.ManagedDataTus
	inputs.storage.jobLeaseTimeout = options.http.JobLeaseTimeout
	if inputs.storage.jobLeaseTimeout <= 0 {
		inputs.storage.jobLeaseTimeout = 2 * time.Minute
	}
	if options.http.Logger != nil {
		server.platform.logger = options.http.Logger
		if server.runtime.pageStreamTrace != nil {
			server.runtime.pageStreamTrace.SetLogger(options.http.Logger)
		}
	}
	if err := server.configureRefreshModule(ctx, options.data.Database, inputs); err != nil {
		return fail(err)
	}
	if err := server.configureModules(ctx, options.data.Database, inputs); err != nil {
		return fail(err)
	}
	if server.platform.asyncJobs != nil {
		handlers := make([]jobs.Handler, 0, 4)
		if server.routes.releaseModule != nil {
			handlers = append(handlers, server.routes.releaseModule.JobHandlers(server.platform.asyncJobs)...)
		}
		if server.routes.deploymentModule != nil {
			handlers = append(handlers, server.routes.deploymentModule.JobHandlers()...)
		}
		if server.routes.managedDataModule != nil && server.routes.managedDataModule.HasFinalizeJobs() {
			handlers = append(handlers, server.routes.managedDataModule.JobHandlers(server.platform.asyncJobs)...)
		}
		if server.routes.agentModule != nil {
			handlers = append(handlers, server.routes.agentModule.JobHandlers(server.platform.asyncJobs)...)
		}
		if err := server.platform.jobModule.RegisterHandlers(handlers); err != nil {
			return fail(fmt.Errorf("register async job handlers: %w", err))
		}
	}
	return server, nil
}

func (s *applicationAssembly) configureModules(ctx context.Context, database *sql.DB, inputs moduleAssemblyInputs) error {
	if s == nil {
		return errors.New("runtime router is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var apiDispatcher *apiGenDispatcher
	if s.routes.accessModule == nil {
		var err error
		s.routes.accessModule, err = accessmodule.Build(ctx, accessmodule.Config{
			Database: database, ExistingAuth: s.platform.auth, WorkspaceID: s.policy.defaultWorkspaceID,
			WorkspaceIDs: func(ctx context.Context) ([]string, error) {
				if inputs.persistence.workspaceDirectory != nil {
					return inputs.persistence.workspaceDirectory.WorkspaceIDs(ctx)
				}
				repository, err := s.workspaceReadModel(inputs)
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
		})
		if err != nil {
			return fmt.Errorf("build access module: %w", err)
		}
	}
	if s.routes.workspaceModule == nil {
		refreshSupport := s.workspaceRefreshSupport()
		var err error
		s.routes.workspaceModule, err = workspacemodule.Build(ctx, workspacemodule.Config{
			Database:            database,
			Directory:           inputs.persistence.workspaceDirectory,
			ReadModel:           inputs.persistence.workspaceReadModel,
			AccessService:       s.routes.accessModule.WorkspaceAccessService(),
			AssetCatalog:        inputs.persistence.workspaceAssetCatalog,
			WorkspaceID:         s.workspaceID,
			Environment:         func(r *http.Request) string { return string(s.requestServingEnvironment(r)) },
			MetricsForWorkspace: s.metricsForWorkspace,
			RootMetrics:         s.runtime.metrics,
			CurrentPrincipal: func(r *http.Request) (workspacemodule.Principal, bool) {
				principal, ok := s.routes.accessModule.CurrentPrincipal(r)
				return workspacemodule.Principal{
					ID: principal.ID, Email: principal.Email,
					DisplayName: principal.DisplayName, DevBypass: principal.DevBypass,
				}, ok
			},
			AuthConfigured:     s.platform.auth != nil,
			RuntimeEnvironment: s.policy.defaultEnvironment,
			DefaultWorkspaceID: s.policy.defaultWorkspaceID,
			RefreshState:       refreshSupport,
			RefreshRunner: workspacemodule.AssetRefreshFunc(func(ctx context.Context, input workspacemodule.AssetRefreshInput) error {
				return refreshSupport.RefreshAsset(ctx, input.Request, input.WorkspaceID, input.Asset, input.Assets, input.Edges)
			}),
			Broker:           s.runtime.broker,
			CSRFToken:        s.routes.accessModule.CSRFToken,
			CurrentRoleLabel: s.routes.accessModule.CurrentRoleLabel,
			ChromeOptions: func(r *http.Request) []ui.ChromeOption {
				return []ui.ChromeOption{s.routes.agentModule.ChromeOption(r)}
			},
			CurrentCredential: func(r *http.Request) (accessmodule.APICredential, bool) {
				return accessmodule.APICredentialFromContext(r.Context())
			},
			AuthorizeObject: s.routes.accessModule.AuthorizeObject,
		})
		if err != nil {
			return fmt.Errorf("build workspace module: %w", err)
		}
		inputs.persistence.workspaceAssetCatalog = nil
	}
	if s.routes.deploymentModule == nil {
		config := inputs.workflow.deploymentConfig
		config.Logger = s.platform.logger
		config.InstanceEnvironment = s.policy.defaultEnvironment
		config.CurrentPrincipal = func(r *http.Request) (deploymentmodule.Principal, bool) {
			principal, ok := s.routes.accessModule.CurrentPrincipal(r)
			return deploymentmodule.Principal{ID: principal.ID}, ok
		}
		config.Jobs = deploymentmodule.JobConfig{
			Reconcile: func(ctx context.Context) error {
				if s.routes.refreshModule == nil {
					return nil
				}
				return s.routes.refreshModule.Reconcile(ctx)
			},
			Events: s.platform.asyncJobs,
			Logger: s.platform.logger,
		}
		config.API = deploymentmodule.APIConfig{Releases: s.routes.releaseModule.DeploymentLinkage(), Jobs: s.platform.asyncJobs}
		config.PublicationAuthorization = deploymentmodule.PublicationAuthorizationConfig{
			States: inputs.persistence.servingStateRepo, AuthorizeObject: s.routes.accessModule.AuthorizeObject,
			Bypass: func(actor string) bool {
				return (s.platform.auth == nil || s.platform.auth.DevBypass()) && actor == accessmodule.LocalDeveloperPrincipal().ID
			},
		}
		var err error
		s.routes.deploymentModule, err = deploymentmodule.Build(ctx, config)
		if err != nil {
			return fmt.Errorf("build deployment module: %w", err)
		}
	}
	if s.routes.dashboardModule == nil {
		var err error
		s.routes.dashboardModule, err = dashboardmodule.Build(ctx, dashboardmodule.Config{
			Database: database,
			HTTP: dashboardmodule.HTTPConfig{
				Metrics:             s.runtime.metrics,
				MetricsForWorkspace: s.metricsForWorkspace,
				Admission:           s.workloadController(), Broker: s.runtime.broker, Logger: s.platform.logger,
				Telemetry: s.platform.telemetry,
				CurrentPrincipalID: func(r *http.Request) string {
					principal, ok := accessmodule.PrincipalFromContext(r.Context())
					if !ok {
						return ""
					}
					return principal.ID
				},
				AuthorizeListObject: s.authorizeListObject,
				CSRFToken:           s.routes.accessModule.CSRFToken,
				ChatChromeSignal:    s.routes.agentModule.ChromeSignal,
				Environment:         func(r *http.Request) string { return string(s.requestServingEnvironment(r)) },
				DataRefreshedAt: func(ctx context.Context, workspaceID, environment, modelID string) string {
					if s.routes.refreshModule == nil {
						return ""
					}
					version, ok, err := s.routes.refreshModule.DataVersion(ctx, workspaceID, environment, modelID)
					if err != nil || !ok {
						return ""
					}
					return version.RefreshedAt.Format(time.RFC3339)
				},
				AgentBootstrap: func(r *http.Request, workspaceID string) ui.ChatViewState {
					return s.routes.agentModule.HTTP().DashboardBootstrap(r, workspaceID)
				},
			},
			Semantic: dashboardmodule.SemanticConfig{
				Metrics:             s.runtime.metrics,
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
				DocumentObserved: s.platform.telemetry.PublicDocumentObserved,
				StreamStarted:    s.platform.telemetry.PublicStreamStarted,
				CommandObserved:  s.platform.telemetry.PublicCommandObserved,
			},
			Logger:    s.platform.logger,
			Trace:     s.runtime.pageStreamTrace,
			PublicURL: inputs.storage.publicURL,
			CurrentActor: func(r *http.Request) string {
				principal, ok := accessmodule.PrincipalFromContext(r.Context())
				if !ok {
					return ""
				}
				return principal.ID
			},
			RuntimeMetrics: s.runtime.metrics, DefaultWorkspaceID: s.policy.defaultWorkspaceID,
			ServingSnapshot: func(ctx context.Context, workspaceID string) (string, error) {
				if s.routes.workspaceModule == nil {
					return "", nil
				}
				return s.routes.workspaceModule.ActiveServingStateID(ctx, s.workspaceID(workspaceID))
			},
		})
		if err != nil {
			return fmt.Errorf("build dashboard module: %w", err)
		}
	}
	if s.routes.agentModule == nil {
		var err error
		s.routes.agentModule, err = agentmodule.Build(ctx, agentmodule.Config{
			Database: database, Model: inputs.workflow.agentConfig,
			Service: inputs.workflow.agent, Jobs: s.platform.asyncJobs, DefaultWorkspaceID: s.policy.defaultWorkspaceID,
			RunWorkloadClass: string(workloadmodule.BackgroundClass), GlobalWorkspaceID: workloadmodule.GlobalWorkspace,
			Search: s.routes.workspaceModule,
			Environment: func(r *http.Request) string {
				return string(s.requestServingEnvironment(r))
			},
			DashboardMetrics:         s.metricsForWorkspace,
			AuthorizeAnyObject:       s.routes.accessModule.AuthorizeAnyObject,
			SkipContextAuthorization: s.platform.auth == nil,
			RecordAudit:              s.routes.accessModule.RecordAudit,
			EnableSystemPrompt:       s.runtime.persistenceConfigured,
			Logger:                   s.platform.logger,
			MCPProtect:               s.routes.accessModule.ProtectMCP,
			MCPScope: func(r *http.Request) (agentmodule.Scope, bool) {
				identity, ok := s.routes.accessModule.MCPIdentity(r)
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
			DispatchAPIGen: func(scope agentmodule.Scope, operationID string, writer http.ResponseWriter, request *http.Request) bool {
				principal := accessmodule.Principal{ID: scope.PrincipalID, DevBypass: scope.DevAuthBypass}
				if s.platform.auth == nil {
					principal = accessmodule.LocalDeveloperPrincipal()
				}
				ctx := accessmodule.WithPrincipal(request.Context(), principal)
				if scope.Credential.Restricted || scope.Credential.WorkspaceID != "" || len(scope.Credential.Privileges) > 0 {
					ctx = accessmodule.WithAPICredential(ctx, accessmodule.AgentAPICredential(
						scope.PrincipalID, scope.Credential.WorkspaceID, scope.Credential.Privileges,
					))
				}
				request = request.WithContext(ctx)
				if apiDispatcher == nil {
					return false
				}
				return apigenapi.DispatchAPIGenOperation(operationID, apiDispatcher, apiprotocol.TransportErrorResponder{Logger: s.platform.logger}, writer, request)
			},
			HTTP: agentmodule.HTTPConfig{
				Settings: inputs.persistence.agentSettings, Broker: s.runtime.broker,
				CSRFToken:        s.routes.accessModule.CSRFToken,
				CurrentRoleLabel: s.routes.accessModule.CurrentRoleLabel,
				CurrentPrincipal: func(r *http.Request) (agentmodule.Principal, bool) {
					if s.platform.auth == nil {
						return agentmodule.Principal{}, false
					}
					principal, ok := s.platform.auth.Principal(r)
					return agentmodule.Principal{ID: principal.ID, DevAuthBypass: principal.DevBypass}, ok
				},
				CurrentCredential: func(r *http.Request) (accessmodule.APICredential, bool) {
					if s.platform.auth == nil {
						return accessmodule.APICredential{}, false
					}
					return s.platform.auth.APICredential(r)
				},
			},
		})
		if err != nil {
			return fmt.Errorf("build agent module: %w", err)
		}
	}
	if s.routes.refreshModule == nil {
		if err := s.configureRefreshModule(ctx, nil, inputs); err != nil {
			return err
		}
	}
	if s.routes.adminModule == nil {
		var accessReader adminmodule.AccessReader
		if reader := s.routes.accessModule.AdminReader(); reader != nil {
			accessReader = reader
		}
		currentAdminPrincipal := func(r *http.Request) (adminmodule.Principal, bool) {
			principal, ok := s.routes.accessModule.CurrentPrincipal(r)
			return adminmodule.Principal{
				ID: principal.ID, Email: principal.Email, DisplayName: principal.DisplayName, DevBypass: principal.DevBypass,
			}, ok
		}
		var err error
		s.routes.adminModule, err = adminmodule.Build(ctx, adminmodule.Config{
			Catalog: func() catalog.Catalog {
				return s.runtime.metrics.Catalog()
			},
			Access: accessReader,
			AgentDetails: func(ctx context.Context) (api.AdminAgentResponse, error) {
				return s.routes.agentModule.HTTP().AdminDetails(ctx)
			},
			QueryAuditReader: s.runtime.queryAuditProvider,
			CSRFToken:        s.routes.accessModule.CSRFToken,
			CurrentPrincipal: currentAdminPrincipal,
			CurrentCredential: func(r *http.Request) (accessmodule.APICredential, bool) {
				if s.platform.auth == nil {
					return accessmodule.APICredential{}, false
				}
				return s.platform.auth.APICredential(r)
			},
			AuthorizeAnyWorkspace: s.routes.accessModule.AuthorizeAnyWorkspace,
			Publications:          s.routes.dashboardModule,
			DefaultWorkspaceID:    s.policy.defaultWorkspaceID,
			AuthConfigured:        s.platform.auth != nil,
			AccessConfigured:      accessReader != nil,
			Storage: adminmodule.StorageConfig{
				CatalogPath: inputs.storage.duckLakeCatalogPath, DataPath: inputs.storage.duckLakeDataPath,
				Environment: s.policy.defaultEnvironment, ControlPlane: inputs.persistence.adminDatabase,
				Analytics: s.runtime.analyticsModule.AdminResources(), Admitter: s.workloadController(),
			},
			CurrentRoleLabel: func(r *http.Request) string {
				principal, ok := currentAdminPrincipal(r)
				return adminmodule.RoleLabel(s.platform.auth != nil, principal, ok)
			},
			ChromeOption: s.routes.agentModule.ChromeOption,
			EnsureClientID: func(w http.ResponseWriter, r *http.Request) {
				_ = pagestream.EnsureClientID(w, r)
			},
			Broker: s.runtime.broker,
		})
		if err != nil {
			return fmt.Errorf("build admin module: %w", err)
		}
	}
	if s.routes.managedDataModule == nil {
		var err error
		s.routes.managedDataModule, err = manageddatamodule.Build(ctx, manageddatamodule.Config{
			Disabled:    true,
			Environment: s.policy.defaultEnvironment, Jobs: s.platform.asyncJobs,
			CurrentPrincipal: func(r *http.Request) (manageddatamodule.Principal, bool) {
				if s.platform.auth == nil {
					return manageddatamodule.Principal{}, false
				}
				principal, ok := s.platform.auth.Principal(r)
				return manageddatamodule.Principal{ID: principal.ID}, ok
			},
		})
		if err != nil {
			return fmt.Errorf("build managed data module: %w", err)
		}
	}
	objects, err := s.routes.workspaceModule.SecurableObjects(ctx, s.policy.defaultWorkspaceID)
	if err != nil {
		return fmt.Errorf("resolve workspace securables: %w", err)
	}
	if err := s.routes.accessModule.RegisterSecurables(ctx, objects); err != nil {
		return fmt.Errorf("register workspace securables: %w", err)
	}
	apiDispatcher = &apiGenDispatcher{
		accessModule: s.routes.accessModule, agentModule: s.routes.agentModule,
		dashboardModule: s.routes.dashboardModule, deploymentModule: s.routes.deploymentModule,
		managedDataModule: s.routes.managedDataModule, refreshModule: s.routes.refreshModule,
		releaseModule: s.routes.releaseModule, workspaceModule: s.routes.workspaceModule,
		defaultEnvironment: s.policy.defaultEnvironment, managedDataTus: s.policy.managedDataTus,
		queryAuditEvents: s.runtime.queryAuditEvents,
	}
	apiGenAuthorizer, err := s.routes.accessModule.APIGenAuthorizer(accessmodule.APIGenObjectResolvers{
		Dashboard:      dashboardmodule.DashboardObjectRefs,
		SemanticModel:  dashboardmodule.SemanticDatasetObjectRefs,
		WorkspaceAsset: workspacemodule.AssetObjectRefs,
	})
	if err != nil {
		return fmt.Errorf("build APIGen authorizer: %w", err)
	}
	s.platform.apiGenHandler, err = apiapigenruntime.Build(
		apiGenAuthorizer,
		apiDispatcher,
		apiprotocol.TransportErrorResponder{Logger: s.platform.logger},
	)
	if err != nil {
		return fmt.Errorf("build APIGen transport: %w", err)
	}
	s.configurePageStream()
	s.platform.health = observability.NewHealth(observability.HealthConfig{
		Platform: func(ctx context.Context) error {
			if s.runtime.platformHealth == nil {
				return errors.New("platform store is missing")
			}
			return s.runtime.platformHealth.Ping(ctx)
		},
		Analytics: func() error {
			if s.runtime.analyticsModule == nil {
				return nil
			}
			return s.runtime.analyticsModule.Healthy()
		},
		Checks: map[string]func(context.Context) error{
			"mapAssets": func(ctx context.Context) error {
				if s.routes.dashboardAssets == nil {
					return nil
				}
				return s.routes.dashboardAssets.Verify(ctx)
			},
		},
		ActiveWorkspaces: s.routes.workspaceModule.ActiveRuntimeWorkspaces,
		RuntimeReady:     s.routes.dashboardModule.RuntimeReady,
	})
	s.platform.workers = platformlifecycle.New(
		platformlifecycle.Component{Start: s.routes.refreshModule.Start, Stop: s.routes.refreshModule.Stop},
		platformlifecycle.Component{
			Start: func(ctx context.Context) error { s.routes.managedDataModule.Start(ctx); return nil },
			Stop:  s.routes.managedDataModule.Stop,
		},
		platformlifecycle.Component{Start: s.routes.dashboardModule.Start, Stop: s.routes.dashboardModule.Stop},
		platformlifecycle.Component{Start: s.platform.jobModule.Start, Stop: s.platform.jobModule.Stop},
	)
	return nil
}

func (s *applicationAssembly) StartBackgroundJobs(ctx context.Context) error {
	if s == nil || s.platform.workers == nil {
		return nil
	}
	return s.platform.workers.Start(ctx)
}

func (s *applicationAssembly) StopBackgroundJobs(ctx context.Context) error {
	if s == nil || s.platform.workers == nil {
		return nil
	}
	return s.platform.workers.Stop(ctx)
}

func (s *applicationAssembly) workspaceReadModel(inputs moduleAssemblyInputs) (workspacemodule.ReadModel, error) {
	return inputs.persistence.workspaceReadModel, nil
}

func (s *applicationAssembly) authorizeListObject(ctx context.Context, principalID string, object accessmodule.ObjectRef) (bool, error) {
	if s.platform.auth == nil {
		return true, nil
	}
	if strings.TrimSpace(principalID) == "" {
		return false, nil
	}
	return s.routes.accessModule.AuthorizeObject(ctx, principalID, accessmodule.PrivilegeViewItem, object)
}

func (s *applicationAssembly) metricsForWorkspace(workspaceID string) (QueryMetrics, bool) {
	if workspaceID == "" {
		return nil, false
	}
	if provider, ok := s.runtime.metrics.(workspaceMetrics); ok {
		return provider.MetricsForWorkspace(workspaceID)
	}
	if s.runtime.metrics == nil {
		return nil, false
	}
	if s.policy.defaultWorkspaceID != "" && workspaceID == s.policy.defaultWorkspaceID {
		return s.runtime.metrics, true
	}
	catalog := s.runtime.metrics.Catalog()
	if catalog.Workspace.ID == "" || catalog.Workspace.ID == workspaceID {
		return s.runtime.metrics, true
	}
	return nil, false
}
