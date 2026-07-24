package app

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Yacobolo/leapview/internal/access"
	agentcontracts "github.com/Yacobolo/leapview/internal/agent/contracts"
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
		QueryFreshness: s.semanticQueryFreshness,
	}
}

type queryFreshnessMetadata struct {
	lastSuccessfulRefreshAt string
	snapshotID              string
	servingStateID          string
	source                  string
	status                  string
}

func (s *Server) queryFreshnessMetadata(ctx context.Context, workspaceID, modelID, servingSnapshot string) (queryFreshnessMetadata, bool) {
	if s.refreshPipelineRepo == nil {
		return queryFreshnessMetadata{}, false
	}
	version, ok, err := s.refreshPipelineRepo.DataVersion(ctx, workspaceID, s.defaultEnvironment, modelID)
	if err != nil || !ok {
		return queryFreshnessMetadata{}, false
	}
	status := "stale"
	if version.ServingStateID == servingSnapshot {
		status = "current"
	}
	return queryFreshnessMetadata{
		lastSuccessfulRefreshAt: version.RefreshedAt.UTC().Format(time.RFC3339),
		snapshotID:              strconv.FormatInt(version.SnapshotID, 10),
		servingStateID:          version.ServingStateID,
		source:                  version.Source,
		status:                  status,
	}, true
}

func (s *Server) semanticQueryFreshness(ctx context.Context, workspaceID, modelID, servingSnapshot string) (api.QueryFreshness, bool) {
	metadata, ok := s.queryFreshnessMetadata(ctx, workspaceID, modelID, servingSnapshot)
	if !ok {
		return api.QueryFreshness{}, false
	}
	return api.QueryFreshness{
		LastSuccessfulRefreshAt: metadata.lastSuccessfulRefreshAt,
		SnapshotID:              metadata.snapshotID,
		ServingStateID:          metadata.servingStateID,
		Source:                  metadata.source,
		Status:                  metadata.status,
	}, true
}

func (s *Server) dashboardQueryFreshness(ctx context.Context, workspaceID, modelID, servingSnapshot string) (agentcontracts.QueryFreshness, bool) {
	metadata, ok := s.queryFreshnessMetadata(ctx, workspaceID, modelID, servingSnapshot)
	if !ok {
		return agentcontracts.QueryFreshness{}, false
	}
	return agentcontracts.QueryFreshness{
		LastSuccessfulRefreshAt: metadata.lastSuccessfulRefreshAt,
		SnapshotID:              metadata.snapshotID,
		ServingStateID:          metadata.servingStateID,
		Source:                  metadata.source,
		Status:                  metadata.status,
	}, true
}
