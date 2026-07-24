package app

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Yacobolo/leapview/internal/access"
	queryhttp "github.com/Yacobolo/leapview/internal/analytics/query/http"
	"github.com/Yacobolo/leapview/internal/api"
)

func (s *Server) semanticQueryHTTP() queryhttp.Handler {
	return queryhttp.Handler{
		Metrics: s.metrics,
		MetricsForWorkspace: func(workspaceID string) (queryhttp.Metrics, bool) {
			return s.metricsForWorkspace(workspaceID)
		},
		CurrentPrincipalID: func(r *http.Request) string {
			principal, ok := principalFromContext(r.Context())
			if !ok {
				return ""
			}
			return principal.ID
		},
		AuthorizeListObject: func(ctx context.Context, principalID string, object access.ObjectRef) (bool, error) {
			return s.authorizeListObject(ctx, principalID, object)
		},
		QueryFreshness: func(ctx context.Context, workspaceID, modelID, servingSnapshot string) (api.QueryFreshness, bool) {
			if s.refreshPipelineRepo == nil {
				return api.QueryFreshness{}, false
			}
			version, ok, err := s.refreshPipelineRepo.DataVersion(ctx, workspaceID, s.defaultEnvironment, modelID)
			if err != nil || !ok {
				return api.QueryFreshness{}, false
			}
			status := "stale"
			if version.ServingStateID == servingSnapshot {
				status = "current"
			}
			return api.QueryFreshness{
				LastSuccessfulRefreshAt: version.RefreshedAt.UTC().Format(time.RFC3339),
				SnapshotID:              strconv.FormatInt(version.SnapshotID, 10),
				ServingStateID:          version.ServingStateID,
				Source:                  version.Source,
				Status:                  status,
			}, true
		},
	}
}
