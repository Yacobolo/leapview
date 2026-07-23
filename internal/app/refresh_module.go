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
	if s == nil || s.refreshModule != nil {
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
		Analytics: s.analyticsModule.WorkspaceMaterializer(), ManagedData: inputs.managedDataResolver,
		HTTP: refreshmodule.HTTPConfig{
			RunnerConfigured: func() bool { return s.metrics != nil },
			CurrentPrincipal: func(r *http.Request) (refreshmodule.HTTPPrincipal, bool) {
				principal, ok := s.accessModule.CurrentPrincipal(r)
				return refreshmodule.HTTPPrincipal{ID: principal.ID}, ok
			},
			WorkspaceID: s.workspaceID,
			Environment: func(*http.Request) string { return string(s.defaultServingEnvironment()) },
		},
		Authorization: refreshmodule.AuthorizationConfig{
			CurrentPrincipal: func(r *http.Request) (refreshmodule.AuthorizationPrincipal, bool) {
				principal, ok := s.accessModule.CurrentPrincipal(r)
				return refreshmodule.AuthorizationPrincipal{ID: principal.ID, DevBypass: principal.DevBypass}, ok
			},
			CurrentCredential: func(r *http.Request) (accessmodule.APICredential, bool) {
				return accessmodule.APICredentialFromContext(r.Context())
			},
			ResolvePipelineModel: refreshmodule.PipelineModelResolver(
				inputs.servingStateRepo,
				nil,
				s.defaultServingEnvironment(),
			),
			AuthorizeObject: s.accessModule.AuthorizeObject,
		},
		ApplyAccessSnapshot: accessmodule.ApplySnapshot,
		Admission:           s.workloadController(), LeaseTimeout: inputs.jobLeaseTimeout,
		Environment: string(s.defaultServingEnvironment()), Clock: inputs.refreshPipelineClock,
		EnableDispatcher: database != nil && s.metrics != nil,
		EnableScheduler:  database != nil && inputs.servingStateRepo != nil,
		Logger:           s.logger, Events: s.asyncJobs,
		WorkloadStats: func() refreshmodule.WorkloadStats {
			return s.workloadController().Stats()
		},
		RunFinished: func(ctx context.Context, run refreshmodule.RunRecord) {
			if run.Status == refreshmodule.RunStatusSucceeded && s.storageRetention != nil {
				_ = s.storageRetention.Run(ctx, false)
			}
		},
	}
	module, err := refreshmodule.Build(ctx, config)
	if err != nil {
		return fmt.Errorf("build refresh module: %w", err)
	}
	s.refreshModule = module
	return nil
}
