package app

import (
	"net/http"

	queryhttp "github.com/Yacobolo/libredash/internal/analytics/query/http"
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
	}
}
