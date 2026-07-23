package app

import (
	"context"
	"database/sql"
	"net/http"

	accessmodule "github.com/Yacobolo/leapview/internal/access/module"
	refreshmodule "github.com/Yacobolo/leapview/internal/refresh/module"
)

func (s *runtimeRouter) configureRefreshModule(database *sql.DB) {
	if s == nil || s.refreshModule != nil {
		return
	}
	service, err := s.workspaceRefreshService()
	if err != nil && database != nil {
		s.logger.ErrorContext(context.Background(), "configure refresh service failed", "error", err)
		return
	}
	config := refreshmodule.Config{
		Database: database, Service: service,
		Analytics: s.analyticsModule.WorkspaceMaterializer(), ManagedData: s.construction.managedDataResolver,
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
				s.construction.servingStateRepo,
				nil,
				s.defaultServingEnvironment(),
			),
			AuthorizeObject: s.accessModule.AuthorizeObject,
		},
		ApplyAccessSnapshot: accessmodule.ApplySnapshot,
		Admission:           s.workloadController(), LeaseTimeout: s.construction.jobLeaseTimeout,
		Environment: string(s.defaultServingEnvironment()), Clock: s.construction.refreshPipelineClock,
		EnableDispatcher: database != nil && s.metrics != nil,
		EnableScheduler:  database != nil && s.construction.servingStateRepo != nil,
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
	module, err := refreshmodule.Build(context.Background(), config)
	if err != nil {
		s.logger.ErrorContext(context.Background(), "configure refresh module failed", "error", err)
		return
	}
	s.refreshModule = module
}
