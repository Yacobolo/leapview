package cli

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	oidcauth "github.com/Yacobolo/leapview/internal/access/oidc"
	accesssqlite "github.com/Yacobolo/leapview/internal/access/sqlite"
	"github.com/Yacobolo/leapview/internal/agent"
	agentsqlite "github.com/Yacobolo/leapview/internal/agent/sqlite"
	analyticsduckdb "github.com/Yacobolo/leapview/internal/analytics/duckdb"
	materializesqlite "github.com/Yacobolo/leapview/internal/analytics/materialize/sqlite"
	"github.com/Yacobolo/leapview/internal/analytics/resultcache"
	"github.com/Yacobolo/leapview/internal/app"
	"github.com/Yacobolo/leapview/internal/config"
	"github.com/Yacobolo/leapview/internal/dataquery"
	projectdeployment "github.com/Yacobolo/leapview/internal/deployment"
	deploymentapiadapter "github.com/Yacobolo/leapview/internal/deployment/apiadapter"
	deploymenthttp "github.com/Yacobolo/leapview/internal/deployment/http"
	deploymentsqlite "github.com/Yacobolo/leapview/internal/deployment/sqlite"
	"github.com/Yacobolo/leapview/internal/instancelock"
	manageddataapiadapter "github.com/Yacobolo/leapview/internal/manageddata/apiadapter"
	manageddatahttp "github.com/Yacobolo/leapview/internal/manageddata/http"
	manageddataresolver "github.com/Yacobolo/leapview/internal/manageddata/resolver"
	"github.com/Yacobolo/leapview/internal/manageddata/s3multipart"
	manageddatasqlite "github.com/Yacobolo/leapview/internal/manageddata/sqlite"
	"github.com/Yacobolo/leapview/internal/platform"
	"github.com/Yacobolo/leapview/internal/runtimehost"
	"github.com/Yacobolo/leapview/internal/securefs"
	servingstate "github.com/Yacobolo/leapview/internal/servingstate"
	servingstatesqlite "github.com/Yacobolo/leapview/internal/servingstate/sqlite"
	storagemaintenance "github.com/Yacobolo/leapview/internal/storage/maintenance"
	"github.com/Yacobolo/leapview/internal/workload"
	"github.com/Yacobolo/leapview/internal/workspace"
	workspacesqlite "github.com/Yacobolo/leapview/internal/workspace/sqlite"
	"github.com/spf13/cobra"
)

const defaultHTTPServerShutdownTimeout = 15 * time.Second

func serveCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the LeapView HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.environment = serveEnvironmentFlagValue(cmd.Flags().Changed("environment"), opts.environment)
			return runServe(ctx, opts)
		},
	}
	cmd.Flags().StringVar(&opts.addr, "addr", "", "listen address; defaults to the configured address")
	cmd.Flags().StringVar(&opts.environment, "environment", "", "instance environment; overrides LEAPVIEW_ENVIRONMENT, then defaults to prod in production and dev otherwise")
	cmd.Flags().BoolVar(&opts.production, "production", false, "serve active serving state from the platform DB")
	return cmd
}

func runServe(ctx context.Context, opts *rootOptions) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	production := serveProductionMode(cfg, *opts)
	cfg.Production = production
	addr := opts.addr
	if addr == "" {
		addr = cfg.ListenAddr()
	}
	cfg.Addr = addr
	if err := cfg.Validate(config.ProfileServe); err != nil {
		return err
	}
	environment := serveEnvironment(production, opts.environment, cfg.Environment)
	instanceLock, err := instancelock.Acquire(cfg.HomeDir)
	if err != nil {
		return err
	}
	defer instanceLock.Release()
	server, cleanup, err := servingStateBackedServer(ctx, cfg, production, environment)
	if err != nil {
		return err
	}
	defer cleanup()
	serveCtx, stopServe := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopServe()
	server.StartBackgroundJobs(serveCtx)
	slog.Info("LeapView listening", "url", listenURL(addr), "environment", environment)
	err = runHTTPServer(serveCtx, productionHTTPServer(addr, server.Routes()))
	stopServe()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultHTTPServerShutdownTimeout)
	defer cancel()
	if stopErr := server.StopBackgroundJobs(shutdownCtx); err == nil && stopErr != nil {
		err = stopErr
	}
	return err
}

func serveProductionMode(cfg config.Config, opts rootOptions) bool {
	return opts.production || cfg.Production
}

func serveEnvironment(production bool, flagValue, configuredValue string) servingstate.Environment {
	if value := strings.TrimSpace(flagValue); value != "" {
		return servingstate.NormalizeEnvironment(servingstate.Environment(value))
	}
	if value := strings.TrimSpace(configuredValue); value != "" {
		return servingstate.NormalizeEnvironment(servingstate.Environment(value))
	}
	if production {
		return servingstate.Environment("prod")
	}
	return servingstate.DefaultEnvironment
}

func serveEnvironmentFlagValue(changed bool, value string) string {
	if !changed {
		return ""
	}
	return value
}

func listenURL(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	return "http://" + addr
}

func productionHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
}

func runHTTPServer(ctx context.Context, server *http.Server) error {
	if server == nil {
		return errors.New("http server is required")
	}
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()
	select {
	case err := <-errCh:
		return err
	case <-signalCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultHTTPServerShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errCh
	}
}

func servingStateBackedServer(ctx context.Context, cfg config.Config, production bool, environment servingstate.Environment) (*app.Server, func(), error) {
	cfg = withAnalyticalDefaults(cfg)
	cookieSecure, err := cfg.CookieSecure()
	if err != nil {
		return nil, nil, err
	}
	var allowedHosts []string
	if production {
		allowedHosts, err = cfg.ProductionAllowedHosts()
	} else {
		allowedHosts, err = cfg.AllowedHostList()
	}
	if err != nil {
		return nil, nil, err
	}
	duckLakeCatalogPath := cfg.DuckLakeCatalogPath()
	for _, dir := range []string{cfg.HomeDir, cfg.ArtifactDir(), cfg.DuckDBDirPath(), cfg.RuntimeDir(), cfg.DuckLakeDataDir(), filepath.Dir(duckLakeCatalogPath)} {
		if err := securefs.EnsurePrivateDir(dir); err != nil {
			return nil, nil, err
		}
	}
	store, err := platform.Open(ctx, cfg.DBPath())
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = store.Close() }
	if err := store.BindInstanceEnvironment(ctx, string(environment)); err != nil {
		cleanup()
		return nil, nil, err
	}
	accessRepo := accesssqlite.NewRepository(store.SQLDB())
	workspaceRepo := workspacesqlite.NewRepositoryWithSecurables(store.SQLDB(), accessRepo)
	if !production {
		if err := app.SeedLocalDeveloperPlatformAdmin(ctx, accessRepo); err != nil {
			cleanup()
			return nil, nil, err
		}
	}
	servingStateRepo := servingstatesqlite.NewRepository(store.SQLDB())
	managedDataRepo := manageddatasqlite.NewRepository(store.SQLDB())
	managedDataStorage, err := newManagedDataStorage(ctx, cfg)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	managedDataControl, err := newManagedDataControl(managedDataRepo, managedDataStorage, cfg)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	managedDataCollector, err := newManagedDataCollector(store.SQLDB(), managedDataStorage, cfg)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	managedDataRuntimeCollector, err := newManagedDataRuntimeCollector(managedDataStorage, cfg)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	managedDataResolver, err := manageddataresolver.New(managedDataRepo, servingStateRepo, managedDataStorage.materializer)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	var managedDataMultipart s3multipart.Coordinator
	var managedDataMultipartService *s3multipart.Service
	if managedDataStorage.s3 != nil {
		multipartService, multipartErr := s3multipart.New(managedDataRepo, managedDataStorage.s3, s3multipart.Config{Backend: "s3"})
		if multipartErr != nil {
			cleanup()
			return nil, nil, multipartErr
		}
		managedDataMultipart = multipartService
		managedDataMultipartService = multipartService
	}
	if err := materializesqlite.NewSQLRunRepository(store.SQLDB()).FailRunsForTerminalServingStates(ctx, string(environment), "refresh did not complete"); err != nil {
		cleanup()
		return nil, nil, err
	}
	if _, err := storagemaintenance.Run(ctx, servingStateRepo, storagemaintenance.Options{
		Environment: environment,
		RootDir:     cfg.HomeDir,
		CatalogPath: duckLakeCatalogPath,
		DataPath:    cfg.DuckLakeDataDir(),
		DryRun:      false,
	}); err != nil {
		cleanup()
		return nil, nil, err
	}
	agentRepo := agentsqlite.NewRepository(store.SQLDB())
	summaries, err := workspaceRepo.List(ctx)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	workspaceIDs := make([]servingstate.WorkspaceID, 0, len(summaries))
	for _, summary := range summaries {
		workspaceIDs = append(workspaceIDs, servingstate.WorkspaceID(summary.ID))
	}
	workloadPolicy := serveWorkloadConfig(cfg)
	workloadController, err := workload.New(workloadPolicy)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	queryCachePool, err := resultcache.New(resultcache.Limits{RuntimeEntries: cfg.QueryCacheRuntimeMaxEntries, RuntimeBytes: cfg.QueryCacheRuntimeMaxBytes, WorkspaceEntries: cfg.QueryCacheWorkspaceMaxEntries, WorkspaceBytes: cfg.QueryCacheWorkspaceMaxBytes, NodeEntries: cfg.QueryCacheNodeMaxEntries, NodeBytes: cfg.QueryCacheNodeMaxBytes})
	if err != nil {
		workloadController.Close()
		cleanup()
		return nil, nil, err
	}
	enginePool, err := analyticsduckdb.NewEnginePool(analyticsduckdb.EnginePoolConfig{MaxOpen: workloadPolicy.MaxRunning, NodeMemoryBytes: cfg.DuckDBNodeMemoryMaxBytes, NodeTempBytes: cfg.DuckDBNodeTempMaxBytes, NodeThreads: cfg.DuckDBNodeMaxThreads, TempRoot: cfg.DuckDBTempDirPath()})
	if err != nil {
		_ = queryCachePool.Close()
		workloadController.Close()
		cleanup()
		return nil, nil, err
	}
	resultLimits := dataquery.ResultLimits{MaxRows: cfg.QueryResultMaxRows, MaxBytes: cfg.QueryResultMaxBytes}
	cleanupAnalyticalResources := func() { _ = enginePool.Close(); _ = queryCachePool.Close(); workloadController.Close() }
	var registry *runtimehost.Registry
	registry = runtimehost.NewRegistryWithFactory(runtimehost.RegistryOptions{
		Repo:         servingStateRepo,
		WorkspaceIDs: workspaceIDs,
		Environment:  environment,
		ManagedData:  managedDataResolver,
		OnDrained: func(servingstate.ID, int64) {
			go func() {
				protected := []int64(nil)
				if registry != nil {
					protected = registry.LeasedSnapshots()
				}
				if _, err := storagemaintenance.Run(context.Background(), servingStateRepo, storagemaintenance.Options{
					Environment:                  environment,
					RootDir:                      cfg.HomeDir,
					CatalogPath:                  duckLakeCatalogPath,
					DataPath:                     cfg.DuckLakeDataDir(),
					AdditionalProtectedSnapshots: protected,
					DryRun:                       false,
				}); err != nil {
					slog.Default().Warn("storage retention cleanup failed after runtime drain", "error", err)
				}
			}()
		},
		Factory: servingStateRuntimeFactory{
			duckDBDir:        cfg.DuckDBDirPath(),
			runtimeDir:       cfg.RuntimeDir(),
			catalogPath:      duckLakeCatalogPath,
			duckLakeDataPath: cfg.DuckLakeDataDir(),
			enginePool:       enginePool,
			queryCachePool:   queryCachePool,
			resultLimits:     resultLimits,
		},
	})
	reloadLease, err := workloadController.Acquire(ctx, workload.Request{Class: workload.Control, Operation: "runtime.reload"})
	if err != nil {
		_ = registry.Close()
		cleanupAnalyticalResources()
		cleanup()
		return nil, nil, err
	}
	err = registry.Reload(reloadLease.Context())
	reloadLease.Release()
	if err != nil {
		_ = registry.Close()
		cleanupAnalyticalResources()
		cleanup()
		return nil, nil, err
	}
	deploymentRuntime, err := projectdeployment.NewRegistryRuntime(registry)
	if err != nil {
		_ = registry.Close()
		cleanupAnalyticalResources()
		cleanup()
		return nil, nil, err
	}
	deploymentService, err := projectdeployment.New(deploymentsqlite.NewRepository(store.SQLDB()), servingStateRepo, deploymentRuntime, managedDataResolver)
	if err != nil {
		_ = registry.Close()
		cleanupAnalyticalResources()
		cleanup()
		return nil, nil, err
	}
	deploymentAPI, err := deploymentapiadapter.New(deploymentService, managedDataRepo)
	if err != nil {
		_ = registry.Close()
		cleanupAnalyticalResources()
		cleanup()
		return nil, nil, err
	}
	managedDataAPI, err := manageddataapiadapter.New(managedDataRepo)
	if err != nil {
		_ = registry.Close()
		cleanupAnalyticalResources()
		cleanup()
		return nil, nil, err
	}
	cleanupWithRegistry := func() {
		_ = registry.Close()
		cleanupAnalyticalResources()
		cleanup()
	}
	runtimeMetrics := app.NewDynamicRuntimeMetrics("", func(workspaceID string) runtimehost.Provider {
		return registry.ProviderForWorkspace(servingstate.WorkspaceID(workspaceID))
	})
	assetCatalog := workspace.NewAssetCatalogService(workspaceRepo)
	authConfig := app.AuthConfig{DevBypass: true, DevAPIToken: cfg.DevAPIToken, CSRFKey: cfg.CSRFKey, CookieSecure: false}
	if production {
		oidcProviders := []oidcauth.Config{}
		if cfg.OIDCConfigured() {
			oidcProviders = append(oidcProviders, oidcauth.Config{
				ID:           cfg.OIDCProviderID,
				IssuerURL:    cfg.OIDCIssuerURL,
				ClientID:     cfg.OIDCClientID,
				ClientSecret: cfg.OIDCSecret,
				RedirectURL:  cfg.OIDCCallbackURL,
				Scopes:       cfg.OIDCScopesList(),
			})
		}
		authConfig = app.AuthConfig{
			DevBypass:       cfg.DevAuthBypass,
			DevAPIToken:     cfg.DevAPIToken,
			APITokenOnly:    cfg.APITokenOnlyAuth,
			LocalAuth:       cfg.LocalAuth,
			AzureClientID:   cfg.AzureClientID,
			AzureSecret:     cfg.AzureSecret,
			AzureCallback:   cfg.AzureCallbackURL,
			AzureTenant:     cfg.AzureTenant,
			CSRFKey:         cfg.CSRFKey,
			CookieSecure:    cookieSecure,
			BootstrapTenant: cfg.AzureTenant,
			OIDCProviders:   oidcProviders,
		}
	}
	auth := app.NewAuth(accessRepo, "", authConfig)
	rateLimits := app.ProductionRateLimitConfig()
	rateLimits.Enabled = production && cfg.RateLimitingEnabled()
	rateLimits.UseRealIP = cfg.RateLimitingUsesRealIP()
	server := app.NewWithOptions(runtimeMetrics, app.Options{
		Store:               store,
		ServingStateRepo:    servingStateRepo,
		WorkspaceRepo:       workspaceRepo,
		AssetCatalog:        assetCatalog,
		AccessRepo:          accessRepo,
		Agent:               agent.NewService(runtimeMetrics, agentRepo, agent.Config{APIKey: cfg.AgentAPIKey, BaseURL: cfg.AgentBaseURL, Model: cfg.AgentModel}),
		Auth:                auth,
		Reloader:            registry,
		ArtifactDir:         cfg.ArtifactDir(),
		DuckDBDir:           cfg.DuckDBDirPath(),
		DuckLakeCatalogPath: duckLakeCatalogPath,
		DuckLakeDataPath:    cfg.DuckLakeDataDir(),
		DuckDBEnginePool:    enginePool,
		QueryResultCache:    queryCachePool,
		QueryResultLimits:   resultLimits,
		DefaultEnvironment:  string(environment),
		RateLimits:          rateLimits,
		SecurityHeaders:     app.SecurityHeaders(production && cfg.HSTSEnabled(cookieSecure)),
		RequestLogging:      production && cfg.RequestLoggingEnabled(),
		Logger:              slog.Default(),
		SCIMBearerToken:     cfg.SCIMBearerToken,
		MetricsBearerToken:  cfg.MetricsBearerToken,
		MCPOAuth: app.MCPOAuthConfig{
			PublicURL: firstConfigured(cfg.PublicURL, listenURL(cfg.ListenAddr())),
			IssuerURL: cfg.MCPOAuthIssuerURL,
		},
		AllowedHosts:    allowedHosts,
		Workload:        workloadController,
		JobLeaseTimeout: cfg.RefreshJobLeaseTimeout,
		ManagedData: manageddatahttp.Options{
			Repository: managedDataAPI, Uploads: managedDataControl,
			Multipart: managedDataMultipart,
		},
		ManagedDataResolver: managedDataResolver,
		Deployment:          deploymenthttp.Options{Coordinator: deploymentAPI},
		ManagedDataTus:      managedDataStorage.tus,
		ManagedDataExpirer: managedDataMaintenance{
			uploads: managedDataControl, multipart: managedDataMultipartService,
			uploadTTL: cfg.ManagedDataUploadSessionTTL, collector: managedDataCollector, runtime: managedDataRuntimeCollector,
		},
		ManagedDataExpireInterval: cfg.ManagedDataGCInterval,
	})
	return server, cleanupWithRegistry, nil
}

func withAnalyticalDefaults(cfg config.Config) config.Config {
	if cfg.DuckDBNodeMemoryMaxBytes <= 0 {
		cfg.DuckDBNodeMemoryMaxBytes = 2684354560
	}
	if cfg.DuckDBNodeTempMaxBytes <= 0 {
		cfg.DuckDBNodeTempMaxBytes = 10737418240
	}
	if cfg.DuckDBNodeMaxThreads <= 0 {
		cfg.DuckDBNodeMaxThreads = 5
	}
	if cfg.QueryResultMaxRows <= 0 {
		cfg.QueryResultMaxRows = 10000
	}
	if cfg.QueryResultMaxBytes <= 0 {
		cfg.QueryResultMaxBytes = 32 << 20
	}
	if cfg.QueryCacheRuntimeMaxEntries <= 0 {
		cfg.QueryCacheRuntimeMaxEntries = 256
	}
	if cfg.QueryCacheRuntimeMaxBytes <= 0 {
		cfg.QueryCacheRuntimeMaxBytes = 64 << 20
	}
	if cfg.QueryCacheWorkspaceMaxEntries <= 0 {
		cfg.QueryCacheWorkspaceMaxEntries = 512
	}
	if cfg.QueryCacheWorkspaceMaxBytes <= 0 {
		cfg.QueryCacheWorkspaceMaxBytes = 128 << 20
	}
	if cfg.QueryCacheNodeMaxEntries <= 0 {
		cfg.QueryCacheNodeMaxEntries = 2048
	}
	if cfg.QueryCacheNodeMaxBytes <= 0 {
		cfg.QueryCacheNodeMaxBytes = 512 << 20
	}
	return cfg
}

func serveWorkloadConfig(cfg config.Config) workload.Config {
	if cfg.WorkloadInteractiveMaxRunning == 0 && cfg.WorkloadBackgroundMaxRunning == 0 && cfg.WorkloadRefreshMaxRunning == 0 && cfg.WorkloadControlMaxRunning == 0 && cfg.WorkloadMaintenanceMaxRunning == 0 {
		return workload.DefaultConfig()
	}
	return cfg.WorkloadConfig()
}

func firstConfigured(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
