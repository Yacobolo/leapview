package app

import (
	"context"
	"fmt"
	"net/http"
	"os"

	analyticsduckdb "github.com/Yacobolo/libredash/internal/analytics/duckdb"
	"github.com/Yacobolo/libredash/internal/analytics/materialize"
	materializesqlite "github.com/Yacobolo/libredash/internal/analytics/materialize/sqlite"
	"github.com/Yacobolo/libredash/internal/deployment"
	deploymentfs "github.com/Yacobolo/libredash/internal/deployment/filesystem"
	"github.com/Yacobolo/libredash/internal/workspace"
	workspacehttp "github.com/Yacobolo/libredash/internal/workspace/http"
	"github.com/Yacobolo/libredash/internal/workspace/refresh"
)

func (s *Server) workspaceRefreshSupport() workspacehttp.Support {
	return workspacehttp.Support{
		Runs: func() (workspacehttp.RunRepository, error) {
			return s.materializationRunRepository()
		},
		Service: func(repo workspacehttp.RunRepository) (refresh.Service, error) {
			return s.workspaceRefreshService(repo)
		},
		Environment: func(r *http.Request) deployment.Environment {
			return s.requestDeploymentEnvironment(r)
		},
		PrincipalID: func(r *http.Request) string {
			principal, _ := currentPrincipal(s, r)
			return principal.ID
		},
		DispatchQueued: func() {
			s.dispatchQueuedMaterializationJobs(context.Background())
		},
		DataDir:      s.dataDirForWorkspace,
		DirectRunner: appRefreshRunner{metrics: s.metrics},
		ModelLookup:  refreshModelLookup(s.metrics),
		Broker:       s.broker,
		AssetCatalog: func(ctx context.Context, workspaceID string) ([]workspace.AssetView, []workspace.AssetEdgeView, bool) {
			catalog, ok, err := s.workspaceAssetCatalog(ctx, workspaceID, string(s.defaultDeploymentEnvironment()))
			if err != nil || !ok {
				return nil, nil, false
			}
			return assetCatalogViews(catalog), assetCatalogEdgeViews(catalog), true
		},
		WorkspaceView: func(r *http.Request, workspaceID string) workspace.WorkspaceView {
			return s.workspaceResponse(r, workspaceID)
		},
		WorkspaceViewContext: func(_ context.Context, workspaceID string) workspace.WorkspaceView {
			view := catalogWorkspaceView(s.catalogForWorkspace(workspaceID))
			view.ID = workspaceID
			return view
		},
		WorkspaceVersions: s.assetVersionsStateForSection,
	}
}

func (s *Server) workspaceRefreshService(runRepo refresh.RunRepository) (refresh.Service, error) {
	repo, err := s.deploymentRepository()
	if err != nil {
		return refresh.Service{}, err
	}
	if repo == nil {
		return refresh.Service{}, fmt.Errorf("deployment repository is required")
	}
	return refresh.Service{
		Deployments: repo,
		Runs:        runRepo,
		Artifacts:   appRefreshArtifactLoader{},
		Materializer: analyticsduckdb.WorkspaceRefreshMaterializer{
			DuckDBDir:       s.duckDBDir,
			DuckLakeCatalog: s.duckLakeCatalogPath,
			DuckLakeData:    s.duckLakeDataPath,
			DataDir:         s.dataDirForWorkspace,
		},
		Runtime:   appRefreshRuntimeHost{reloader: s.reloader},
		Retention: appRefreshRetention{server: s},
		Publisher: appRefreshPublisher{server: s},
	}, nil
}

type appRefreshArtifactLoader struct{}

func (appRefreshArtifactLoader) Load(_ context.Context, artifact deployment.Artifact) (refresh.LoadedArtifact, error) {
	root, err := os.MkdirTemp("", "libredash-refresh-artifact-*")
	if err != nil {
		return refresh.LoadedArtifact{}, err
	}
	defer os.RemoveAll(root)
	if err := deploymentfs.ExtractArtifact(artifact.Path, root); err != nil {
		return refresh.LoadedArtifact{}, err
	}
	compiled, _, err := deploymentfs.LoadCompiledWorkspaceArtifact(root)
	if err != nil {
		return refresh.LoadedArtifact{}, err
	}
	return refresh.LoadedArtifact{Definition: compiled.Definition, Graph: compiled.Graph}, nil
}

type appRefreshRuntimeHost struct {
	reloader runtimeReloader
}

func (h appRefreshRuntimeHost) PrepareDeployment(ctx context.Context, deploymentID string) (deployment.PreparedRuntime, error) {
	if h.reloader == nil {
		return nil, nil
	}
	return h.reloader.PrepareDeployment(ctx, deploymentID)
}

func (h appRefreshRuntimeHost) CommitPrepared(prepared deployment.PreparedRuntime) error {
	if h.reloader == nil || prepared == nil {
		return nil
	}
	return h.reloader.CommitPrepared(prepared)
}

func (h appRefreshRuntimeHost) Reload(ctx context.Context) error {
	if h.reloader == nil {
		return nil
	}
	return h.reloader.Reload(ctx)
}

type appRefreshRetention struct {
	server *Server
}

func (r appRefreshRetention) Run(ctx context.Context, dryRun bool) error {
	if r.server == nil {
		return nil
	}
	return r.server.reconcileStorageRetention(ctx, dryRun)
}

type appRefreshPublisher struct {
	server *Server
}

func (p appRefreshPublisher) PublishRefreshTarget(ctx context.Context, workspaceID, targetType, targetID string) {
	if p.server == nil {
		return
	}
	p.server.workspaceRefreshSupport().PublishWorkspaceAssetRefreshPatchesForTarget(ctx, workspaceID, targetType, targetID)
}

type appLegacyRefreshExecutor struct {
	repo    *materializesqlite.SQLRunRepository
	metrics QueryMetrics
	logger  interface {
		WarnContext(context.Context, string, ...any)
	}
}

func (e appLegacyRefreshExecutor) ExecuteLegacyJob(ctx context.Context, job materialize.JobRecord) error {
	orchestrator := materialize.NewGenericRefreshOrchestrator(e.repo, appRefreshRunner{metrics: e.metrics}, refreshModelLookup(e.metrics))
	_, err := orchestrator.ExecuteRun(ctx, job.WorkspaceID, job.RunID, materialize.RefreshPublisher{})
	if err != nil && e.logger != nil {
		e.logger.WarnContext(ctx, "materialization job failed", "workspace", job.WorkspaceID, "run", job.RunID, "error", err)
	}
	return err
}
