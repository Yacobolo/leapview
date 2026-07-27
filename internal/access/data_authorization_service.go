package access

import "context"

// DataAuthorizationService is the access-owned policy surface consumed by
// governed analytical query execution.
type DataAuthorizationService interface {
	Authorize(context.Context, string, Privilege, ObjectRef) (AuthorizationDecision, error)
	AuthorizeAny(context.Context, string, Privilege, []ObjectRef) (AuthorizationDecision, error)
	ListEffectiveDataPolicies(context.Context, string, ObjectRef, bool) ([]DataPolicy, error)
	RecordAuditEvent(context.Context, AuditEventInput) error
}
