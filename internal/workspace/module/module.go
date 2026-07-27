package module

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/Yacobolo/leapview/internal/access"
	"github.com/Yacobolo/leapview/internal/catalog"
	"github.com/Yacobolo/leapview/internal/queryruntime"
	"github.com/Yacobolo/leapview/internal/ui"
	"github.com/Yacobolo/leapview/internal/workspace"
	workspacehttp "github.com/Yacobolo/leapview/internal/workspace/http"
	"github.com/Yacobolo/leapview/pkg/pagestream"
)

type Module struct {
	handler            workspacehttp.Handler
	search             searchService
	currentCredential  func(*http.Request) (access.APICredential, bool)
	readModel          workspace.ReadModel
	assetCatalog       workspace.AssetCatalogReader
	rootMetrics        queryruntime.Metrics
	runtimeEnvironment string
	defaultWorkspaceID string
	chromeOptions      func(*http.Request) []ui.ChromeOption
}

type Principal struct {
	ID          string
	Email       string
	DisplayName string
	DevBypass   bool
}

type RefreshStateProvider interface {
	AssetRefreshState(context.Context, string, string, workspace.AssetView) (ui.AssetRefreshState, error)
}

type AssetRefreshInput struct {
	Request     *http.Request
	WorkspaceID string
	Asset       workspace.AssetView
	Assets      []workspace.AssetView
	Edges       []workspace.AssetEdgeView
}

type AssetRefreshRunner interface {
	RefreshAsset(context.Context, AssetRefreshInput) error
}

type AssetRefreshFunc func(context.Context, AssetRefreshInput) error

func (f AssetRefreshFunc) RefreshAsset(ctx context.Context, input AssetRefreshInput) error {
	return f(ctx, input)
}

type Config struct {
	Database            *sql.DB
	Directory           Directory
	ReadModel           ReadModel
	Securables          SecurableRegistrar
	WorkspaceID         func(string) string
	Environment         func(*http.Request) string
	AccessService       access.WorkspaceAccessService
	AssetCatalog        workspace.AssetCatalogReader
	MetricsForWorkspace func(string) (queryruntime.Metrics, bool)
	RootMetrics         queryruntime.Metrics
	CurrentPrincipal    func(*http.Request) (Principal, bool)
	AuthConfigured      bool
	RuntimeEnvironment  string
	DefaultWorkspaceID  string
	RefreshState        RefreshStateProvider
	RefreshRunner       AssetRefreshRunner
	Broker              *pagestream.Broker
	CSRFToken           func(*http.Request) string
	CurrentRoleLabel    func(*http.Request) string
	ChromeOptions       func(*http.Request) []ui.ChromeOption
	CurrentCredential   func(*http.Request) (access.APICredential, bool)
	AuthorizeObject     func(context.Context, string, access.Privilege, access.ObjectRef) (bool, error)
}

func Build(_ context.Context, config Config) (*Module, error) {
	directoryPort := config.Directory
	if directoryPort == nil && config.Database != nil {
		var err error
		directoryPort, err = BuildDirectory(config.Database, config.Securables)
		if err != nil {
			return nil, err
		}
	}
	var repository workspace.Repository
	if owned, ok := directoryPort.(*directory); ok {
		repository = owned.repository
	}
	readModel := config.ReadModel
	if readModel == nil {
		readModel = repository
	}
	m := &Module{
		readModel: readModel, currentCredential: config.CurrentCredential,
		rootMetrics: config.RootMetrics, runtimeEnvironment: config.RuntimeEnvironment,
		defaultWorkspaceID: config.DefaultWorkspaceID, chromeOptions: config.ChromeOptions,
	}
	m.assetCatalog = config.AssetCatalog
	if m.assetCatalog == nil && readModel != nil {
		m.assetCatalog = workspace.NewAssetCatalogService(readModel)
	}
	currentPrincipal := func(r *http.Request) (workspacehttp.Principal, bool) {
		if config.CurrentPrincipal == nil {
			return workspacehttp.Principal{}, false
		}
		principal, ok := config.CurrentPrincipal(r)
		return workspacehttp.Principal{
			ID: principal.ID, Email: principal.Email,
			DisplayName: principal.DisplayName, DevBypass: principal.DevBypass,
		}, ok
	}
	httpReadModel := workspacehttp.ReadModel{
		WorkspaceRepository: func() (workspace.ReadModel, error) { return readModel, nil },
		AccessService:       func() (access.WorkspaceAccessService, error) { return config.AccessService, nil },
		AssetCatalogReader:  m.AssetCatalogReader,
		MetricsForWorkspace: func(workspaceID string) (workspacehttp.Metrics, bool) {
			if config.MetricsForWorkspace == nil {
				return nil, false
			}
			metrics, ok := config.MetricsForWorkspace(workspaceID)
			if !ok || metrics == nil {
				return nil, ok
			}
			return MetricsAdapter{Metrics: metrics}, true
		},
		CatalogForWorkspace: func(workspaceID string) catalog.Catalog {
			if config.MetricsForWorkspace != nil {
				if metrics, ok := config.MetricsForWorkspace(workspaceID); ok && metrics != nil {
					return metrics.Catalog()
				}
			}
			if config.RootMetrics == nil {
				return catalog.Catalog{Workspace: catalog.Workspace{ID: workspaceID}}
			}
			return config.RootMetrics.Catalog()
		},
		RootCatalog: func() catalog.Catalog {
			if config.RootMetrics == nil {
				return catalog.Catalog{}
			}
			return config.RootMetrics.Catalog()
		},
		Environment:      config.Environment,
		CurrentPrincipal: currentPrincipal,
		AuthConfigured:   config.AuthConfigured,
	}
	var refreshRunner workspacehttp.AssetRefreshRunner
	if config.RefreshRunner != nil {
		refreshRunner = moduleRefreshRunner{upstream: config.RefreshRunner}
	}
	m.handler = workspacehttp.Handler{
		WorkspaceID: config.WorkspaceID, Environment: config.Environment, ReadModel: httpReadModel,
		RefreshState:  moduleRefreshState{module: m, upstream: config.RefreshState},
		RefreshRunner: refreshRunner, Broker: config.Broker,
		CSRFToken: config.CSRFToken, CurrentRoleLabel: config.CurrentRoleLabel, ChromeOptions: config.ChromeOptions,
	}
	m.search = buildSearch(config.Database, config.AuthorizeObject)
	return m, nil
}

func (m *Module) HTTP() workspacehttp.Handler { return m.handler }

func (m *Module) CatalogsForVisibleWorkspaces(r *http.Request) []catalog.Catalog {
	return m.handler.ReadModel.CatalogsForVisibleWorkspaces(r)
}

func (m *Module) WorkspaceAssetsAndEdgesForData(ctx context.Context, workspaceID, environment string) ([]workspace.AssetView, []workspace.AssetEdgeView, error) {
	return m.handler.ReadModel.WorkspaceAssetsAndEdgesForData(ctx, workspaceID, environment)
}

func (m *Module) WorkspaceResponse(r *http.Request, workspaceID string) workspace.WorkspaceView {
	return m.handler.ReadModel.WorkspaceResponse(r, workspaceID)
}

func (m *Module) WorkspaceViewContext(ctx context.Context, workspaceID string) workspace.WorkspaceView {
	return m.handler.ReadModel.WorkspaceViewContext(ctx, workspaceID)
}

func (m *Module) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	var options []ui.ChromeOption
	if m.chromeOptions != nil {
		options = m.chromeOptions(r)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := ui.CatalogPageForCatalogs(m.CatalogsForVisibleWorkspaces(r), options...).Render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
