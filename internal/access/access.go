package access

import "context"

type Principal struct {
	ID          string
	Email       string
	DisplayName string
	CreatedAt   string
	UpdatedAt   string
}

type Role struct {
	Name        string
	Permissions []string
}

type RoleBinding struct {
	ID          string
	WorkspaceID string
	PrincipalID string
	Email       string
	DisplayName string
	Role        string
	CreatedAt   string
}

type PrincipalRoleInput struct {
	WorkspaceID string
	Email       string
	DisplayName string
	Role        string
}

type Repository interface {
	PrincipalByID(ctx context.Context, id string) (Principal, error)
	SetPrincipalRole(ctx context.Context, input PrincipalRoleInput) (Principal, error)
	RemovePrincipalRoles(ctx context.Context, workspaceID, principalID string) error
	ListRoleBindings(ctx context.Context, workspaceID string) ([]RoleBinding, error)
	ListRoles(ctx context.Context) ([]Role, error)
	HasPermission(ctx context.Context, workspaceID, principalID, permission string) (bool, error)
}
