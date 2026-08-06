package app

import (
	"context"

	analyticsmodule "github.com/flidai/leapview/internal/analytics/module"
	deploymentmodule "github.com/flidai/leapview/internal/deployment/module"
)

type candidateConnectionLeaser struct {
	leaser *analyticsmodule.RuntimeBindingLeaser
	module *analyticsmodule.Module
}

func (adapter candidateConnectionLeaser) Acquire(
	ctx context.Context,
	request deploymentmodule.CandidateConnectionRequest,
) (deploymentmodule.CandidateConnectionLeases, error) {
	requirements := make([]analyticsmodule.ConnectionRequirement, 0, len(request.Requirements))
	for _, requirement := range request.Requirements {
		logical, err := analyticsmodule.ParseLogicalConnectionID(
			requirement.LogicalConnectionID,
		)
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, analyticsmodule.ConnectionRequirement{
			LogicalConnectionID: logical, ConnectorKind: requirement.ConnectorKind,
		})
	}
	leases, err := adapter.leaser.Acquire(ctx, analyticsmodule.RuntimeBindingRequest{
		Actor: request.Actor,
		Scope: analyticsmodule.ConnectionBindingScope{
			WorkspaceID: request.WorkspaceID, Environment: request.Environment,
		},
		TargetID: request.TargetID, Requirements: requirements,
	})
	if err != nil {
		return nil, err
	}
	registration, err := adapter.module.BindCandidateRuntime(
		request.CandidateID,
		request.WorkspaceID,
		leases,
		candidateAuthoredConnections(request.AuthoredConnections),
	)
	if err != nil {
		_ = leases.Close()
		return nil, err
	}
	return candidateConnectionLeases{RuntimeBindingRegistration: registration}, nil
}

func candidateAuthoredConnections(
	values []deploymentmodule.CandidateAuthoredConnection,
) []analyticsmodule.CandidateAuthoredConnection {
	result := make([]analyticsmodule.CandidateAuthoredConnection, len(values))
	for index, value := range values {
		result[index] = analyticsmodule.CandidateAuthoredConnection{
			LogicalConnectionID: value.LogicalConnectionID,
			ConnectorKind:       value.ConnectorKind,
		}
	}
	return result
}

type candidateConnectionLeases struct {
	*analyticsmodule.RuntimeBindingRegistration
}

func (leases candidateConnectionLeases) Evidence() []deploymentmodule.CandidateConnectionEvidence {
	source := leases.RuntimeBindingRegistration.Evidence()
	result := make([]deploymentmodule.CandidateConnectionEvidence, 0, len(source))
	for _, evidence := range source {
		result = append(result, deploymentmodule.CandidateConnectionEvidence{
			BindingID: evidence.BindingID, LogicalConnection: evidence.LogicalConnection.String(),
			ConnectorKind: evidence.ConnectorKind, Revision: evidence.BindingRevision,
			ProviderVersion: evidence.ValidatedVersion, EndpointConfigHash: evidence.EndpointConfigHash,
		})
	}
	return result
}
