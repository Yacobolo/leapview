package app

import (
	"context"
	"fmt"
	"strings"

	analyticsmodule "github.com/flidai/leapview/internal/analytics/module"
	releasemodule "github.com/flidai/leapview/internal/release/module"
)

type servingStateProvenanceReader interface {
	ProvenanceForServingState(context.Context, string, string) (releasemodule.Provenance, error)
}

type activeConnectionEvidenceSource struct {
	releases    servingStateProvenanceReader
	targetID    string
	environment string
}

func (source activeConnectionEvidenceSource) BindingEvidence(
	ctx context.Context,
	servingStateID string,
	workspaceID string,
) ([]analyticsmodule.ActiveRuntimeBindingEvidence, error) {
	if source.releases == nil {
		return nil, releasemodule.ErrNotFound
	}
	provenance, err := source.releases.ProvenanceForServingState(ctx, servingStateID, workspaceID)
	if err != nil {
		return nil, err
	}
	if provenance.Plan.TargetID != strings.TrimSpace(source.targetID) ||
		provenance.Plan.Environment != strings.TrimSpace(source.environment) {
		return nil, fmt.Errorf("%w: release target does not match runtime target", releasemodule.ErrProvenanceInvalid)
	}
	for _, workspace := range provenance.Plan.Workspaces {
		if workspace.WorkspaceID != strings.TrimSpace(workspaceID) ||
			workspace.ServingStateID != strings.TrimSpace(servingStateID) {
			continue
		}
		result := make([]analyticsmodule.ActiveRuntimeBindingEvidence, len(workspace.Bindings))
		for index, evidence := range workspace.Bindings {
			result[index] = analyticsmodule.ActiveRuntimeBindingEvidence{
				BindingID: evidence.BindingID, LogicalConnection: evidence.LogicalConnection,
				ConnectorKind: evidence.ConnectorKind, Revision: evidence.Revision,
				ValidatedVersion: evidence.ValidatedVersion, EndpointConfigHash: evidence.EndpointConfigHash,
			}
		}
		return result, nil
	}
	return nil, releasemodule.ErrNotFound
}
