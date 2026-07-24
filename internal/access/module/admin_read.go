package module

import (
	"context"

	"github.com/Yacobolo/leapview/internal/access"
)

// AdminReader is the read-only access surface consumed by platform
// administration. It deliberately excludes every access mutation.
type AdminReader struct {
	repository access.Repository
}

func (m *Module) AdminReader() *AdminReader {
	if m == nil {
		return nil
	}
	repository := m.repositoryValue()
	if repository == nil {
		return nil
	}
	return &AdminReader{repository: repository}
}

func (r *AdminReader) ListPrincipals(ctx context.Context, filter access.PrincipalFilter) ([]access.Principal, error) {
	return r.repository.ListPrincipals(ctx, filter)
}

func (r *AdminReader) ListAllGroups(ctx context.Context) ([]access.Group, error) {
	return r.repository.ListAllGroups(ctx)
}

func (r *AdminReader) ListGroupMembersByGroup(ctx context.Context, groupID string) ([]access.GroupMember, error) {
	return r.repository.ListGroupMembersByGroup(ctx, groupID)
}

func (r *AdminReader) ListRoles(ctx context.Context) ([]access.Role, error) {
	return r.repository.ListRoles(ctx)
}

func (r *AdminReader) ListAllRoleBindings(ctx context.Context) ([]access.RoleBinding, error) {
	return r.repository.ListAllRoleBindings(ctx)
}

func (r *AdminReader) Authorize(ctx context.Context, principalID string, privilege access.Privilege, object access.ObjectRef) (access.AuthorizationDecision, error) {
	return r.repository.Authorize(ctx, principalID, privilege, object)
}
