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
			if s.refreshModule == nil {
				return nil, fmt.Errorf("refresh module is required")
			}
			return s.refreshModule, nil
		},
		QueuePipeline: func(ctx context.Context, input refreshmodule.QueuePipelineInput) (refreshmodule.QueueAssetResult, error) {
			if s.refreshModule == nil {
				return refreshmodule.QueueAssetResult{}, fmt.Errorf("refresh module is required")
			}
			return s.refreshModule.QueuePipelineRefresh(ctx, input)
		},
		Environment: func(r *http.Request) servingstatemodule.Environment {
			return s.requestServingEnvironment(r)
		},
		PrincipalID: func(r *http.Request) string {
			principal, _ := s.accessModule.CurrentPrincipal(r)
			return principal.ID
		},
		DispatchQueued: func() {
			if s.refreshModule != nil {
				s.refreshModule.Dispatch(context.Background())
			}
		},
		Broker: s.broker,
		AssetCatalog: func(ctx context.Context, workspaceID string) ([]workspacemodule.AssetView, []workspacemodule.AssetEdgeView, bool) {
			assets, edges, err := s.workspaceModule.WorkspaceAssetsAndEdgesForData(ctx, workspaceID, string(s.defaultServingEnvironment()))
			if err != nil || (len(assets) == 0 && len(edges) == 0) {
				return nil, nil, false
			}
			return assets, edges, true
		},
		WorkspaceView: func(r *http.Request, workspaceID string) workspacemodule.WorkspaceView {
			return s.workspaceModule.WorkspaceResponse(r, workspaceID)
		},
		WorkspaceViewContext: func(ctx context.Context, workspaceID string) workspacemodule.WorkspaceView {
			return s.workspaceModule.WorkspaceViewContext(ctx, workspaceID)
		},
		Presentation: workspacemodule.RefreshPresentation{},
	}
	if s.persistenceConfigured {
		support.DataVersions = s.refreshModule
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
	if inputs.managedDataValidation != nil {
		hooks = append(hooks, inputs.managedDataValidation)
	}
	return refreshmodule.Service{
		ServingStates: repo,
		Runtime:       inputs.reloader,
		Publisher: refreshmodule.Publisher{
			Workspace: s.workspaceRefreshSupport,
			SemanticModelVersion: func(ctx context.Context, workspaceID, environment, modelID string) {
				refreshedAt := ""
				if s.refreshModule != nil {
					if version, ok, err := s.refreshModule.DataVersion(ctx, workspaceID, environment, modelID); err == nil && ok {
						refreshedAt = version.RefreshedAt.Format(time.RFC3339)
					}
				}
				if s.dashboardModule != nil {
					s.dashboardModule.PublishSemanticModelRefresh(workspaceID, environment, modelID, refreshedAt)
				}
			},
		},
		CandidateValidationHooks: hooks,
	}, nil
}
