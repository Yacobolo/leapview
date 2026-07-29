package module

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/flidai/leapview/internal/access"
	dashboarddefinition "github.com/flidai/leapview/internal/dashboard/definition"
	queryauthz "github.com/flidai/leapview/internal/dashboard/queryauthz"
	dashboardsession "github.com/flidai/leapview/internal/dashboard/session"
	dashboardui "github.com/flidai/leapview/internal/dashboard/ui"
	"github.com/flidai/leapview/internal/platform/digest"
	"github.com/flidai/leapview/pkg/pagestream"
)

type CandidateHTTPConfig struct {
	Metrics                  Metrics
	CandidateID              string
	OwnerPrincipalID         string
	WorkspaceID              string
	ArtifactDigest           string
	AuthorizationFingerprint string
	RouteBasePath            string
	Restrictions             []CandidateRestriction
}

type CandidateRestriction struct {
	ID             string
	WorkspaceID    string
	ObjectID       string
	PolicyType     string
	ExpressionJSON string
}

// CandidateHTTP derives an isolated dashboard adapter from the shared
// Dashboard capability. The returned handler owns no runtime or policy state;
// those remain in the server-resolved candidate provider and query context.
func (m *Module) CandidateHTTP(config CandidateHTTPConfig) (HTTP, error) {
	if m == nil || config.Metrics == nil {
		return HTTP{}, fmt.Errorf("candidate dashboard metrics are required")
	}
	config.CandidateID = strings.TrimSpace(config.CandidateID)
	config.OwnerPrincipalID = strings.TrimSpace(config.OwnerPrincipalID)
	config.WorkspaceID = strings.TrimSpace(config.WorkspaceID)
	config.ArtifactDigest = strings.TrimSpace(config.ArtifactDigest)
	config.AuthorizationFingerprint = strings.TrimSpace(config.AuthorizationFingerprint)
	config.RouteBasePath = strings.TrimSuffix(strings.TrimSpace(config.RouteBasePath), "/")
	if config.CandidateID == "" || config.OwnerPrincipalID == "" ||
		config.WorkspaceID == "" || config.RouteBasePath == "" {
		return HTTP{}, fmt.Errorf("candidate dashboard identity, owner, workspace, and route are required")
	}
	if err := digest.ValidateSHA256Identity(config.ArtifactDigest); err != nil {
		return HTTP{}, fmt.Errorf("candidate artifact digest is invalid: %w", err)
	}
	if err := digest.ValidateSHA256Identity(config.AuthorizationFingerprint); err != nil {
		return HTTP{}, fmt.Errorf("candidate authorization fingerprint is invalid: %w", err)
	}

	handler := m.handler
	handler.Metrics = config.Metrics
	handler.MetricsForWorkspace = nil
	handler.RouteScope = dashboardui.RouteScope{BasePath: config.RouteBasePath}
	handler.StreamNamespace = "candidate:" + config.CandidateID
	handler.AgentBootstrap = nil
	baseAnalyticalContext := handler.AnalyticalContext
	handler.AnalyticalContext = func(ctx context.Context) context.Context {
		if baseAnalyticalContext != nil {
			ctx = baseAnalyticalContext(ctx)
		}
		restrictions := make([]access.DataPolicy, len(config.Restrictions))
		for index, restriction := range config.Restrictions {
			restrictions[index] = access.DataPolicy{
				ID: restriction.ID, WorkspaceID: restriction.WorkspaceID,
				ObjectID: restriction.ObjectID, PolicyType: restriction.PolicyType,
				ExpressionJSON: restriction.ExpressionJSON,
			}
		}
		return queryauthz.WithCandidateQueryCapability(ctx, queryauthz.CandidateQueryCapability{
			CandidateID: config.CandidateID, OwnerPrincipalID: config.OwnerPrincipalID,
			WorkspaceID: config.WorkspaceID, PolicyDigest: config.AuthorizationFingerprint,
			Restrictions: restrictions,
		})
	}
	currentPrincipalID := handler.CurrentPrincipalID
	handler.SessionKey = func(
		r *http.Request,
		report dashboarddefinition.Definition,
		clientID, streamInstanceID string,
	) dashboardsession.Key {
		principalOrClient := clientID
		if currentPrincipalID != nil {
			if principalID := currentPrincipalID(r); principalID != "" {
				principalOrClient = principalID + ":" + clientID
			}
		}
		if principalOrClient == "" {
			principalOrClient = pagestream.ClientIDFromRequest(r, clientID)
		}
		return dashboardsession.Key{
			WorkspaceOrPublication: "candidate:" + config.CandidateID + ":" + config.WorkspaceID,
			PrincipalOrClient:      principalOrClient,
			DashboardID:            report.ID,
			ServingStateID:         "candidate:" + config.CandidateID + ":" + config.ArtifactDigest,
			StreamInstanceID:       streamInstanceID,
		}
	}
	return handler, nil
}
