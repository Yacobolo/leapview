package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	refreshmodule "github.com/Yacobolo/leapview/internal/refresh/module"
)

func (s *applicationAssembly) configureRefreshModule(ctx context.Context, database *sql.DB, inputs moduleAssemblyInputs) error {
	if s == nil || s.routes.refreshModule != nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	service, err := s.workspaceRefreshService(inputs)
	if err != nil && database != nil {
		return fmt.Errorf("configure refresh service: %w", err)
	}
	config := refreshmodule.Config{
		Database: database, Service: service,
		Analytics: s.runtime.analyticsModule.WorkspaceMaterializer(), ManagedData: inputs.workflow.managedDataResolver,
		HTTP: refreshmodule.HTTPConfig{
			RunnerConfigured: func() bool { return s.runtime.metrics != nil },
			CurrentPrincipal: func(r *http.Request) (refreshmodule.HTTPPrincipal, bool) {
				principal, ok := s.routes.accessModule.CurrentPrincipal(r)
				return refreshmodule.HTTPPrincipal{ID: principal.ID}, ok
			},
			WorkspaceID: s.workspaceID,
			Environment: func(*http.Request) string { return string(s.defaultServingEnvironment()) },
		},
		Authorization: refreshmodule.AuthorizationConfig{
			CurrentPrincipal: func(r *http.Request) (refreshmodule.AuthorizationPrincipal, bool) {
				principal, ok := s.routes.accessModule.CurrentPrincipal(r)
				return refreshmodule.AuthorizationPrincipal{ID: principal.ID, DevBypass: principal.DevBypass}, ok
			},
			CurrentCredential: func(r *http.Request) (accessmodule.APICredential, bool) {
				return accessmodule.APICredentialFromContext(r.Context())
			},
			ResolvePipelineModel: refreshmodule.PipelineModelResolver(
				inputs.persistence.servingStateRepo,
				nil,
				s.defaultServingEnvironment(),
			),
			AuthorizeObject: s.routes.accessModule.AuthorizeObject,
		},
		ApplyAccessSnapshot: accessmodule.ApplySnapshot,
		Admission:           s.workloadController(), LeaseTimeout: inputs.storage.jobLeaseTimeout,
		Environment: string(s.defaultServingEnvironment()), Clock: inputs.workflow.refreshPipelineClock,
		EnableDispatcher: database != nil && s.runtime.metrics != nil,
		EnableScheduler:  database != nil && inputs.persistence.servingStateRepo != nil,
		Logger:           s.platform.logger, Events: s.platform.asyncJobs,
		WorkloadStats: func() refreshmodule.WorkloadStats {
			return s.workloadController().Stats()
		},
		RunFinished: func(ctx context.Context, run refreshmodule.RunRecord) {
			if run.Status == refreshmodule.RunStatusSucceeded && s.runtime.storageRetention != nil {
				_ = s.runtime.storageRetention.Run(ctx, false)
			}
		},
	}
	module, err := refreshmodule.Build(ctx, config)
	if err != nil {
		return fmt.Errorf("build refresh module: %w", err)
	}
	s.routes.refreshModule = module
	return nil
}
