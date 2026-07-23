package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	agentmodule "github.com/Yacobolo/leapview/internal/agent/module"
	analyticsmodule "github.com/Yacobolo/leapview/internal/analytics/module"
	apihttpmiddleware "github.com/Yacobolo/leapview/internal/api/httpmiddleware"
	"github.com/Yacobolo/leapview/internal/config"
	dashboardmodule "github.com/Yacobolo/leapview/internal/dashboard/module"
	deploymentmodule "github.com/Yacobolo/leapview/internal/deployment/module"
	manageddatamodule "github.com/Yacobolo/leapview/internal/manageddata/module"
	"github.com/Yacobolo/leapview/internal/platform"
	jobsmodule "github.com/Yacobolo/leapview/internal/platform/jobs/module"
	"github.com/Yacobolo/leapview/internal/platform/transaction"
	refreshmodule "github.com/Yacobolo/leapview/internal/refresh/module"
	releasemodule "github.com/Yacobolo/leapview/internal/release/module"
	runtimehostmodule "github.com/Yacobolo/leapview/internal/runtimehost/module"
	"github.com/Yacobolo/leapview/internal/securefs"
	servingstatemodule "github.com/Yacobolo/leapview/internal/servingstate/module"
	workloadmodule "github.com/Yacobolo/leapview/internal/workload/module"
	workspacemodule "github.com/Yacobolo/leapview/internal/workspace/module"
)

// assemble constructs the complete process exactly once. CLI and other process
// entrypoints provide configuration but never construct capability adapters.
func assemble(ctx context.Context, cfg config.Config) (http.Handler, Lifecycle, cleanupFunc, error) {
	production := cfg.Production
	environment := servingstatemodule.NormalizeEnvironment(servingstatemodule.Environment(cfg.Environment))
	if strings.TrimSpace(cfg.Environment) == "" {
		if production {
			environment = servingstatemodule.Environment("prod")
		} else {
			environment = servingstatemodule.DefaultEnvironment
		}
	}
	runtime, cleanup, err := buildRuntime(ctx, cfg, production, environment)
	if err != nil {
		return nil, nil, nil, err
	}
	handler := runtime.Routes()
	lifecycle := newRuntimeLifecycle(runtime.workers, runtime.analyticsModule, runtime.workloads)
	runtime.releaseConstructionInputs()
	return handler, lifecycle, func(context.Context) error {
		cleanup()
		return nil
	}, nil
}

func buildRuntime(ctx context.Context, cfg config.Config, production bool, environment servingstatemodule.Environment) (*runtimeRouter, func(), error) {
	dashboardAssets, err := dashboardmodule.BuildAssets(ctx, cfg.MapAssetDir)
	if err != nil {
		return nil, nil, err
	}
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
	var analyticsModule *analyticsmodule.Module
	var workloadController *workloadmodule.Module
	cleanup := func() {
		if workloadController != nil {
			workloadController.Close()
		}
		if analyticsModule != nil {
			_ = analyticsModule.Close()
		}
		_ = store.Close()
	}
	if err := store.BindInstanceEnvironment(ctx, string(environment)); err != nil {
		cleanup()
		return nil, nil, err
	}
	workloadConfig := cfg.WorkloadConfig()
	analyticsModule, err = analyticsmodule.Build(ctx, analyticsmodule.Config{
		Database: store.SQLDB(), RootDir: cfg.DuckDBDirPath(),
		CatalogPath: duckLakeCatalogPath, DataPath: cfg.DuckLakeDataDir(),
		MaxConnections: workloadConfig.MaxRunning, MemoryMaxBytes: cfg.DuckDBNodeMemoryMaxBytes,
		TempMaxBytes: cfg.DuckDBNodeTempMaxBytes, MaxThreads: cfg.DuckDBNodeMaxThreads,
		TempDir:             cfg.DuckDBTempDirPath(),
		RuntimeCacheEntries: cfg.QueryCacheRuntimeMaxEntries, RuntimeCacheBytes: cfg.QueryCacheRuntimeMaxBytes,
		WorkspaceCacheEntries: cfg.QueryCacheWorkspaceMaxEntries, WorkspaceCacheBytes: cfg.QueryCacheWorkspaceMaxBytes,
		NodeCacheEntries: cfg.QueryCacheNodeMaxEntries, NodeCacheBytes: cfg.QueryCacheNodeMaxBytes,
	})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	duckDBEnvironment := analyticsModule.Environment()
	resultCachePool := analyticsModule.Cache()
	var workspacePersistence *workspacemodule.Persistence
	accessModule, err := accessmodule.Build(ctx, accessmodule.Config{
		Database: store.SQLDB(), Auth: accessAuthConfig(cfg, production, cookieSecure),
		WorkspaceID: platform.DefaultWorkspaceID,
		PublicURL:   firstConfigured(cfg.PublicURL, configuredListenURL(cfg.ListenAddr())), MCPIssuerURL: cfg.MCPOAuthIssuerURL,
		WorkspaceIDs: func(ctx context.Context) ([]string, error) {
			if workspacePersistence == nil {
				return nil, nil
			}
			return workspacePersistence.WorkspaceIDs(ctx)
		},
	})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	accessSecurables := accessModule.Securables()
	workspacePersistence, err = workspacemodule.BuildPersistence(store.SQLDB(), accessSecurables)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	if !production {
		if err := accessModule.SeedLocalDeveloperPlatformAdmin(ctx); err != nil {
			cleanup()
			return nil, nil, err
		}
	}
	servingStateRepo, err := servingstatemodule.Build(ctx, servingstatemodule.Config{Database: store.SQLDB()})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	workloadController, err = workloadmodule.Build(ctx, workloadmodule.Config{Policy: workloadConfig})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	jobModule, err := jobsmodule.Build(ctx, jobsmodule.Config{
		Database: store.SQLDB(), Admission: workloadController,
		LeaseTimeout: cfg.RefreshJobLeaseTimeout, Logger: slog.Default(),
	})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	managedDataModule, err := manageddatamodule.Build(ctx, manageddatamodule.Config{
		Database: store.SQLDB(), Product: cfg, ServingStates: servingStateRepo,
		Environment: string(environment),
		CurrentPrincipal: func(r *http.Request) (manageddatamodule.Principal, bool) {
			auth := accessModule.Auth()
			if auth == nil {
				return manageddatamodule.Principal{}, false
			}
			principal, ok := auth.Principal(r)
			return manageddatamodule.Principal{ID: principal.ID}, ok
		},
		Jobs: jobModule,
		Worker: manageddatamodule.MaintenanceWorkerConfig{
			Interval: cfg.ManagedDataGCInterval,
			Acquire: func(ctx context.Context) (manageddatamodule.MaintenanceLease, error) {
				return workloadController.Acquire(ctx, workloadmodule.MaintenanceRequest("managed_data.collect"))
			},
			Logger: slog.Default(),
		},
	})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	releaseModule, err := releasemodule.Build(ctx, releasemodule.Config{
		Database: store.SQLDB(),
		States:   servingStateRepo, Workspaces: workspacePersistence,
		ManagedDataPins: managedDataModule.BindingValidator(), ManagedDataHook: managedDataModule.BindingValidator(),
		ArtifactDirectory: cfg.ArtifactDir(), Environment: environment,
		API: releasemodule.APIConfig{
			CurrentPrincipal: func(r *http.Request) (releasemodule.Principal, bool) {
				auth := accessModule.Auth()
				if auth == nil {
					principal := accessmodule.LocalDeveloperPrincipal()
					return releasemodule.Principal{ID: principal.ID}, true
				}
				principal, ok := auth.Principal(r)
				return releasemodule.Principal{ID: principal.ID}, ok
			},
			Jobs: jobModule,
		},
	})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	managedDataResolution := managedDataModule.RuntimeResolver()
	if managedDataResolution == nil {
		cleanup()
		return nil, nil, errors.New("managed-data runtime resolver is required")
	}
	managedDataResolver := runtimehostmodule.NewManagedDataResolver(managedDataResolution)
	if err := refreshmodule.Recover(ctx, store.SQLDB(), string(environment)); err != nil {
		cleanup()
		return nil, nil, err
	}
	retention := servingstatemodule.NewRetention(servingstatemodule.RetentionConfig{
		States: servingStateRepo, Snapshots: duckDBEnvironment,
		Admission: workloadController, Environment: string(environment),
		CatalogPath: duckLakeCatalogPath, DataPath: cfg.DuckLakeDataDir(),
	})
	if err := retention.Run(ctx, false); err != nil {
		cleanup()
		return nil, nil, err
	}
	workspaceIDValues, err := workspacePersistence.WorkspaceIDs(ctx)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	workspaceIDs := make([]servingstatemodule.WorkspaceID, 0, len(workspaceIDValues))
	for _, workspaceID := range workspaceIDValues {
		workspaceIDs = append(workspaceIDs, servingstatemodule.WorkspaceID(workspaceID))
	}
	runtimeHostModule, err := runtimehostmodule.Build(ctx, runtimehostmodule.Config{
		States:       servingStateRepo,
		WorkspaceIDs: workspaceIDs,
		Environment:  environment,
		ManagedData:  managedDataResolver,
		OnDrained: func(_ servingstatemodule.ID, _ int64, protected []int64) {
			go func() {
				if err := retention.RunWithProtected(context.Background(), false, protected); err != nil {
					slog.Default().Warn("storage retention cleanup failed after runtime drain", "error", err)
				}
			}()
		},
		Factory: runtimehostmodule.NewFactory(runtimehostmodule.FactoryConfig{
			DuckDBDir: cfg.DuckDBDirPath(), RuntimeDir: cfg.RuntimeDir(),
			DashboardRuntime: dashboardmodule.NewRuntimeFactory(dashboardmodule.RuntimeFactoryConfig{
				Database: duckDBEnvironment, Cache: resultCachePool,
				MaxRows: cfg.QueryResultMaxRows, MaxBytes: cfg.QueryResultMaxBytes,
			}),
		}),
	})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	deploymentRuntime, err := deploymentmodule.NewRuntime(runtimeHostModule)
	if err != nil {
		_ = runtimeHostModule.Close()
		cleanup()
		return nil, nil, err
	}
	deploymentConfig := deploymentmodule.Config{
		Database: store.SQLDB(), States: servingStateRepo, Runtime: deploymentRuntime,
		ManagedData: managedDataResolver, DeploymentMetadata: managedDataModule.DeploymentMetadata(),
		ActivationHooks: deploymentmodule.ActivationHooks{
			ApplyAccessSnapshot: accessmodule.ApplySnapshot,
			ReconcilePublications: func(ctx context.Context, tx transaction.Transaction, input deploymentmodule.PublicationActivationInput) error {
				return dashboardmodule.ReconcilePublications(ctx, tx, dashboardmodule.PublicationActivationInput{
					ProjectID: input.ProjectID, WorkspaceID: input.WorkspaceID,
					ServingStateID: input.ServingStateID, ActorID: input.ActorID,
					Publications: input.Publications,
				})
			},
		},
	}
	cleanupWithRegistry := func() {
		_ = runtimeHostModule.Close()
		cleanup()
	}
	runtimeMetrics := dashboardmodule.NewDynamicRuntimeMetrics("", func(workspaceID string) runtimehostmodule.Provider {
		return runtimeHostModule.ProviderForWorkspace(servingstatemodule.WorkspaceID(workspaceID))
	})
	auth := accessModule.Auth()
	rateLimits := apihttpmiddleware.ProductionRateLimitConfig()
	rateLimits.Enabled = production && cfg.RateLimitingEnabled()
	rateLimits.UseRealIP = cfg.RateLimitingUsesRealIP()
	server, err := assembleRuntimeChecked(ctx, runtimeMetrics, assemblyConfig{
		Database:              store.SQLDB(),
		PlatformHealth:        store,
		AgentSettings:         store,
		AdminDatabase:         store.SQLDB(),
		AnalyticsModule:       analyticsModule,
		DashboardAssets:       dashboardAssets,
		ServingStateRepo:      servingStateRepo,
		StorageRetention:      retention,
		WorkspacePersistence:  workspacePersistence,
		ReleaseModule:         releaseModule,
		JobModule:             jobModule,
		AccessModule:          accessModule,
		AgentConfig:           agentmodule.ModelConfig{APIKey: cfg.AgentAPIKey, BaseURL: cfg.AgentBaseURL, Model: cfg.AgentModel},
		Auth:                  auth,
		Reloader:              runtimeHostModule,
		DuckLakeCatalogPath:   duckLakeCatalogPath,
		DuckLakeDataPath:      cfg.DuckLakeDataDir(),
		DefaultEnvironment:    string(environment),
		PublicURL:             firstConfigured(cfg.PublicURL, configuredListenURL(cfg.ListenAddr())),
		RateLimits:            rateLimits,
		SecurityHeaders:       apihttpmiddleware.SecurityHeaders(production && cfg.HSTSEnabled(cookieSecure)),
		RequestLogging:        production && cfg.RequestLoggingEnabled(),
		Logger:                slog.Default(),
		SCIMBearerToken:       cfg.SCIMBearerToken,
		MetricsBearerToken:    cfg.MetricsBearerToken,
		AllowedHosts:          allowedHosts,
		Workload:              workloadController,
		JobLeaseTimeout:       cfg.RefreshJobLeaseTimeout,
		ManagedDataModule:     managedDataModule,
		ManagedDataValidation: managedDataModule.BindingValidator(),
		ManagedDataResolver:   managedDataResolver,
		DeploymentConfig:      deploymentConfig,
		ManagedDataTus:        managedDataModule.TusHandler(),
	})
	if err != nil {
		cleanupWithRegistry()
		return nil, nil, err
	}
	return server, cleanupWithRegistry, nil
}

func firstConfigured(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func configuredListenURL(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	return "http://" + addr
}

func accessAuthConfig(cfg config.Config, production, cookieSecure bool) accessmodule.AuthConfig {
	if !production {
		return accessmodule.AuthConfig{DevBypass: true, DevAPIToken: cfg.DevAPIToken, CSRFKey: cfg.CSRFKey}
	}
	providers := []accessmodule.OIDCProviderConfig{}
	if cfg.OIDCConfigured() {
		providers = append(providers, accessmodule.OIDCProviderConfig{
			ID: cfg.OIDCProviderID, IssuerURL: cfg.OIDCIssuerURL, ClientID: cfg.OIDCClientID,
			ClientSecret: cfg.OIDCSecret, RedirectURL: cfg.OIDCCallbackURL, Scopes: cfg.OIDCScopesList(),
		})
	}
	return accessmodule.AuthConfig{
		DevBypass: cfg.DevAuthBypass, DevAPIToken: cfg.DevAPIToken, APITokenOnly: cfg.APITokenOnlyAuth,
		LocalAuth: cfg.LocalAuth, AzureClientID: cfg.AzureClientID, AzureSecret: cfg.AzureSecret,
		AzureCallback: cfg.AzureCallbackURL, AzureTenant: cfg.AzureTenant, CSRFKey: cfg.CSRFKey,
		CookieSecure: cookieSecure, BootstrapTenant: cfg.AzureTenant, OIDCProviders: providers,
	}
}
