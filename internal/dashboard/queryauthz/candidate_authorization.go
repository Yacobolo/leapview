package authz

import (
	"context"
	"fmt"
	"strings"

	"github.com/flidai/leapview/internal/access"
	"github.com/flidai/leapview/internal/analytics/dataquery"
	"github.com/flidai/leapview/internal/platform/digest"
)

// CandidateQueryCapability is installed by the candidate runtime adapter after
// it resolves an authenticated author's owned candidate. Candidate policy data
// is deliberately restrictions-only: it may narrow the author's current
// access, but it cannot introduce grants or replace active data policies.
type CandidateQueryCapability struct {
	CandidateID      string
	OwnerPrincipalID string
	WorkspaceID      string
	PolicyDigest     string
	Restrictions     []access.DataPolicy
}

type candidateQueryCapabilityKey struct{}

func WithCandidateQueryCapability(ctx context.Context, capability CandidateQueryCapability) context.Context {
	return context.WithValue(ctx, candidateQueryCapabilityKey{}, capability)
}

func candidateQueryCapabilityFromContext(ctx context.Context) (CandidateQueryCapability, bool) {
	capability, ok := ctx.Value(candidateQueryCapabilityKey{}).(CandidateQueryCapability)
	return capability, ok
}

func validateCandidateQueryCapability(
	capability CandidateQueryCapability,
	actor Principal,
	request dataquery.Query,
) (dataquery.Query, error) {
	candidateID := strings.TrimSpace(capability.CandidateID)
	ownerID := strings.TrimSpace(capability.OwnerPrincipalID)
	workspaceID := strings.TrimSpace(capability.WorkspaceID)
	policyDigest := strings.TrimSpace(capability.PolicyDigest)
	if candidateID == "" || ownerID == "" || workspaceID == "" ||
		candidateID != capability.CandidateID ||
		ownerID != capability.OwnerPrincipalID ||
		workspaceID != capability.WorkspaceID ||
		policyDigest != capability.PolicyDigest {
		return request, fmt.Errorf("candidate query capability is incomplete")
	}
	if err := digest.ValidateSHA256Identity(policyDigest); err != nil {
		return request, fmt.Errorf("candidate query policy digest is invalid: %w", err)
	}
	if strings.TrimSpace(actor.ID) == "" || actor.ID != ownerID {
		return request, fmt.Errorf("candidate %q is not owned by the authenticated principal", candidateID)
	}
	if request.WorkspaceID != workspaceID {
		return request, fmt.Errorf("candidate query workspace %q is outside candidate workspace %q", request.WorkspaceID, workspaceID)
	}
	if request.CandidateID != "" && request.CandidateID != candidateID {
		return request, fmt.Errorf("candidate query identity %q does not match capability %q", request.CandidateID, candidateID)
	}
	for _, policy := range capability.Restrictions {
		if strings.TrimSpace(policy.ID) == "" || policy.WorkspaceID != workspaceID {
			return request, fmt.Errorf("candidate query restriction is outside candidate workspace")
		}
		if !policy.Compiled.Matches(policy.PolicyType, policy.ExpressionJSON) {
			return request, fmt.Errorf("candidate query restriction %q is not compiled", policy.ID)
		}
		switch policy.PolicyType {
		case "row_filter", "column_mask":
		default:
			return request, fmt.Errorf("candidate query restriction %q has unsupported type %q", policy.ID, policy.PolicyType)
		}
	}
	request.CandidateID = candidateID
	request.PrincipalID = ownerID
	return request, nil
}
