package authz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/flidai/leapview/internal/access"
	"github.com/flidai/leapview/internal/analytics/dataquery"
)

// ViewAsCapability is installed by a trusted preview handler. It identifies
// both the authenticated actor and the effective subject; neither identity is
// accepted from the authored query payload.
type ViewAsCapability struct {
	ActorPrincipalID   string
	SubjectPrincipalID string
	WorkspaceID        string
}

type viewAsCapabilityKey struct{}

func WithViewAsCapability(ctx context.Context, capability ViewAsCapability) context.Context {
	return context.WithValue(ctx, viewAsCapabilityKey{}, capability)
}

func viewAsCapabilityFromContext(ctx context.Context) (ViewAsCapability, bool) {
	capability, ok := ctx.Value(viewAsCapabilityKey{}).(ViewAsCapability)
	return capability, ok
}

func (m Metrics) authorizeViewAs(
	ctx context.Context,
	actor Principal,
	request dataquery.Query,
	capability ViewAsCapability,
) (dataquery.Query, error) {
	actorID := strings.TrimSpace(capability.ActorPrincipalID)
	subjectID := strings.TrimSpace(capability.SubjectPrincipalID)
	workspaceID := strings.TrimSpace(capability.WorkspaceID)
	deny := func(cause error) (dataquery.Query, error) {
		denied := DeniedError{PrincipalID: actor.ID, Privilege: access.PrivilegeTestDataPolicy}
		if auditErr := m.recordViewAsAudit(ctx, request, actor.ID, subjectID, "denied", cause); auditErr != nil {
			return request, errors.Join(denied, auditErr)
		}
		return request, denied
	}
	if actorID == "" || subjectID == "" || workspaceID == "" {
		return deny(errors.New("view-as capability is incomplete"))
	}
	if actor.ID == "" || actor.ID != actorID {
		return deny(errors.New("view-as actor does not match the authenticated principal"))
	}
	if subjectID == actorID {
		return deny(errors.New("view-as subject must differ from the authenticated principal"))
	}
	if request.WorkspaceID != workspaceID {
		return deny(fmt.Errorf("view-as workspace %q does not match query workspace %q", workspaceID, request.WorkspaceID))
	}
	if credential, ok := m.currentCredential(ctx); ok &&
		!m.allowsToken(credential.Token, workspaceID, access.PrivilegeTestDataPolicy) {
		return deny(errors.New("view-as credential lacks TEST_DATA_POLICY"))
	}
	decision, err := m.repo.Authorize(
		ctx,
		actorID,
		access.PrivilegeTestDataPolicy,
		access.WorkspaceObject(workspaceID),
	)
	if err != nil {
		if auditErr := m.recordViewAsAudit(ctx, request, actorID, subjectID, "error", err); auditErr != nil {
			return request, errors.Join(err, auditErr)
		}
		return request, err
	}
	if !decision.Allowed {
		return deny(errors.New("actor lacks TEST_DATA_POLICY"))
	}
	if err := m.recordViewAsAudit(ctx, request, actorID, subjectID, "authorized", nil); err != nil {
		return request, err
	}
	request.PrincipalID = subjectID
	return request, nil
}

func (m Metrics) recordViewAsAudit(
	ctx context.Context,
	request dataquery.Query,
	actorID string,
	subjectID string,
	status string,
	cause error,
) error {
	metadata := map[string]any{
		"candidateId": request.CandidateID,
		"operation":   request.Operation,
		"surface":     request.Surface,
	}
	if cause != nil {
		metadata["error"] = cause.Error()
	}
	bytes, _ := json.Marshal(metadata)
	return access.PersistAuditEvent(ctx, m.repo, access.AuditEventInput{
		WorkspaceID:   request.WorkspaceID,
		PrincipalID:   actorID,
		Action:        "data_policy.view_as",
		TargetType:    "principal",
		TargetID:      subjectID,
		Privilege:     access.PrivilegeTestDataPolicy,
		Status:        status,
		RequestID:     request.RequestID,
		CorrelationID: request.CorrelationID,
		MetadataJSON:  string(bytes),
	})
}
