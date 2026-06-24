package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

const (
	PermissionDashboardView           = "dashboard:view"
	PermissionDeploymentCreate        = "deployment:create"
	PermissionDeploymentActivate      = "deployment:activate"
	PermissionDeploymentRollback      = "deployment:rollback"
	PermissionMaterializationsRefresh = "materializations:refresh"
	PermissionRBACManage              = "rbac:manage"
)

const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleDeployer = "deployer"
	RoleEditor   = "editor"
	RoleViewer   = "viewer"
)

var defaultRoles = []Role{
	{
		Name: RoleOwner,
		Permissions: []string{
			PermissionDashboardView,
			PermissionDeploymentCreate,
			PermissionDeploymentActivate,
			PermissionDeploymentRollback,
			PermissionMaterializationsRefresh,
			PermissionRBACManage,
		},
	},
	{
		Name: RoleAdmin,
		Permissions: []string{
			PermissionDashboardView,
			PermissionDeploymentCreate,
			PermissionDeploymentActivate,
			PermissionDeploymentRollback,
			PermissionMaterializationsRefresh,
			PermissionRBACManage,
		},
	},
	{
		Name: RoleDeployer,
		Permissions: []string{
			PermissionDashboardView,
			PermissionDeploymentCreate,
			PermissionDeploymentActivate,
			PermissionDeploymentRollback,
			PermissionMaterializationsRefresh,
		},
	},
	{
		Name: RoleEditor,
		Permissions: []string{
			PermissionDashboardView,
			PermissionMaterializationsRefresh,
		},
	},
	{
		Name: RoleViewer,
		Permissions: []string{
			PermissionDashboardView,
		},
	},
}

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

type PrincipalInput struct {
	ID          string
	Email       string
	DisplayName string
}

type ExternalIdentityInput struct {
	Provider    string
	TenantID    string
	Subject     string
	Email       string
	DisplayName string
}

type Repository interface {
	PrincipalByID(ctx context.Context, id string) (Principal, error)
	UpsertPrincipal(ctx context.Context, input PrincipalInput) (Principal, error)
	SetPrincipalRole(ctx context.Context, input PrincipalRoleInput) (Principal, error)
	RemovePrincipalRoles(ctx context.Context, workspaceID, principalID string) error
	ListRoleBindings(ctx context.Context, workspaceID string) ([]RoleBinding, error)
	ListRoles(ctx context.Context) ([]Role, error)
	HasPermission(ctx context.Context, workspaceID, principalID, permission string) (bool, error)
	BootstrapAdmin(ctx context.Context, workspaceID, email string) error
	ResolveExternalPrincipal(ctx context.Context, input ExternalIdentityInput) (Principal, error)
	CreateSession(ctx context.Context, principalID string, ttl time.Duration) (string, error)
	PrincipalForToken(ctx context.Context, token string) (Principal, error)
	DeleteSession(ctx context.Context, token string) error
	CreateAPIToken(ctx context.Context, principalID, name string) (string, error)
	PrincipalForAPIToken(ctx context.Context, token string) (Principal, error)
}

func DefaultRoles() []Role {
	roles := make([]Role, len(defaultRoles))
	for i, role := range defaultRoles {
		roles[i] = Role{
			Name:        role.Name,
			Permissions: append([]string(nil), role.Permissions...),
		}
	}
	return roles
}

func PrincipalIDForEmail(email string) string {
	return "email_" + stableID(NormalizeEmail(email))
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func stableID(value string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(value)))
	return hex.EncodeToString(sum[:])[:32]
}
