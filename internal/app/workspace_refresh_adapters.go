package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	refreshmodule "github.com/Yacobolo/leapview/internal/refresh/module"
	servingstatemodule "github.com/Yacobolo/leapview/internal/servingstate/module"
	workspacemodule "github.com/Yacobolo/leapview/internal/workspace/module"
)

func (s *applicationAssembly) workspaceRefreshSupport() refreshmodule.WorkspaceSupport {
	support := refreshmodule.WorkspaceSupport{
		Runs: func() (refreshmodule.RunReader, error) {
			if s.routes.refreshModule == nil {
				return nil, fmt.Errorf("refresh module is required")
			}
			return s.routes.refreshModule, nil
		},
		QueuePipeline: func(ctx context.Context, input refreshmodule.QueuePipelineInput) (refreshmodule.QueueAssetResult, error) {
			if s.routes.refreshModule == nil {
				return refreshmodule.QueueAssetResult{}, fmt.Errorf("refresh module is required")
			}
			return s.routes.refreshModule.QueuePipelineRefresh(ctx, input)
		},
		Environment: func(r *http.Request) servingstatemodule.Environment {
			return s.requestServingEnvironment(r)
		},
		PrincipalID: func(r *http.Request) string {
			principal, _ := s.routes.accessModule.CurrentPrincipal(r)
			return principal.ID
		},
		DispatchQueued: func() {
			if s.routes.refreshModule != nil {
				s.routes.refreshModule.Dispatch(context.Background())
			}
		},
		Broker: s.runtime.broker,
		AssetCatalog: func(ctx context.Context, workspaceID string) ([]workspacemodule.AssetView, []workspacemodule.AssetEdgeView, bool) {
			assets, edges, err := s.routes.workspaceModule.WorkspaceAssetsAndEdgesForData(ctx, workspaceID, string(s.defaultServingEnvironment()))
			if err != nil || (len(assets) == 0 && len(edges) == 0) {
				return nil, nil, false
			}
			return assets, edges, true
		},
		WorkspaceView: func(r *http.Request, workspaceID string) workspacemodule.WorkspaceView {
			return s.routes.workspaceModule.WorkspaceResponse(r, workspaceID)
		},
		WorkspaceViewContext: func(ctx context.Context, workspaceID string) workspacemodule.WorkspaceView {
			return s.routes.workspaceModule.WorkspaceViewContext(ctx, workspaceID)
		},
		Presentation: workspacemodule.RefreshPresentation{},
	}
	if s.runtime.persistenceConfigured {
		support.DataVersions = s.routes.refreshModule
	}
	return support
}

func (s *applicationAssembly) workspaceRefreshService(inputs moduleAssemblyInputs) (refreshmodule.Service, error) {
	repo, err := s.servingStateRepository(inputs)
	if err != nil {
		return refreshmodule.Service{}, err
	}
	if repo == nil {
		return refreshmodule.Service{}, fmt.Errorf("serving state repository is required")
	}
	hooks := []refreshmodule.CandidateValidationHook{}
	if inputs.workflow.managedDataValidation != nil {
		hooks = append(hooks, inputs.workflow.managedDataValidation)
	}
	return refreshmodule.Service{
		ServingStates: repo,
		Runtime:       inputs.workflow.reloader,
		Publisher: refreshmodule.Publisher{
			Workspace: s.workspaceRefreshSupport,
			SemanticModelVersion: func(ctx context.Context, workspaceID, environment, modelID string) {
				refreshedAt := ""
				if s.routes.refreshModule != nil {
					if version, ok, err := s.routes.refreshModule.DataVersion(ctx, workspaceID, environment, modelID); err == nil && ok {
						refreshedAt = version.RefreshedAt.Format(time.RFC3339)
					}
				}
				if s.routes.dashboardModule != nil {
					s.routes.dashboardModule.PublishSemanticModelRefresh(workspaceID, environment, modelID, refreshedAt)
				}
			},
		},
		CandidateValidationHooks: hooks,
	}, nil
}
