package sqlite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/platform"
	platformdb "github.com/Yacobolo/libredash/internal/platform/db"
)

type Repository struct {
	q *platformdb.Queries
}

func NewRepository(sqlDB *sql.DB) *Repository {
	return &Repository{q: platformdb.New(sqlDB)}
}

func (r *Repository) PrincipalByID(ctx context.Context, id string) (access.Principal, error) {
	row, err := r.q.GetPrincipal(ctx, id)
	if err != nil {
		return access.Principal{}, err
	}
	return mapPrincipal(row), nil
}

func (r *Repository) SetPrincipalRole(ctx context.Context, input access.PrincipalRoleInput) (access.Principal, error) {
	email := normalizeEmail(input.Email)
	if email == "" {
		return access.Principal{}, fmt.Errorf("email is required")
	}
	if strings.TrimSpace(input.Role) == "" {
		return access.Principal{}, fmt.Errorf("role is required")
	}
	role, err := r.q.GetRoleByName(ctx, input.Role)
	if err != nil {
		return access.Principal{}, err
	}
	principalID := platform.PrincipalIDForEmail(email)
	if err := r.q.UpsertPrincipal(ctx, platformdb.UpsertPrincipalParams{
		ID:          principalID,
		Email:       email,
		DisplayName: firstNonEmpty(strings.TrimSpace(input.DisplayName), email),
	}); err != nil {
		return access.Principal{}, err
	}
	principal, err := r.q.GetPrincipal(ctx, principalID)
	if err != nil {
		return access.Principal{}, err
	}
	if err := r.q.DeletePrincipalRoleBindings(ctx, platformdb.DeletePrincipalRoleBindingsParams{
		WorkspaceID: input.WorkspaceID,
		PrincipalID: sql.NullString{String: principal.ID, Valid: true},
	}); err != nil {
		return access.Principal{}, err
	}
	if err := r.q.InsertRoleBinding(ctx, platformdb.InsertRoleBindingParams{
		ID:          newID("rolebinding"),
		WorkspaceID: input.WorkspaceID,
		RoleID:      role.ID,
		PrincipalID: sql.NullString{String: principal.ID, Valid: true},
	}); err != nil {
		return access.Principal{}, err
	}
	return mapPrincipal(principal), nil
}

func (r *Repository) RemovePrincipalRoles(ctx context.Context, workspaceID, principalID string) error {
	if strings.TrimSpace(principalID) == "" {
		return fmt.Errorf("principal id is required")
	}
	return r.q.DeletePrincipalRoleBindings(ctx, platformdb.DeletePrincipalRoleBindingsParams{
		WorkspaceID: workspaceID,
		PrincipalID: sql.NullString{String: principalID, Valid: true},
	})
}

func (r *Repository) ListRoleBindings(ctx context.Context, workspaceID string) ([]access.RoleBinding, error) {
	rows, err := r.q.ListRoleBindingsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	bindings := make([]access.RoleBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, access.RoleBinding{
			ID:          row.ID,
			WorkspaceID: row.WorkspaceID,
			PrincipalID: nullString(row.PrincipalID),
			Email:       nullString(row.Email),
			DisplayName: nullString(row.DisplayName),
			Role:        row.RoleName,
			CreatedAt:   row.CreatedAt,
		})
	}
	return bindings, nil
}

func (r *Repository) ListRoles(ctx context.Context) ([]access.Role, error) {
	rows, err := r.q.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	roles := make([]access.Role, 0, len(rows))
	for _, row := range rows {
		var permissions []string
		_ = json.Unmarshal([]byte(row.PermissionsJson), &permissions)
		roles = append(roles, access.Role{Name: row.Name, Permissions: permissions})
	}
	return roles, nil
}

func (r *Repository) HasPermission(ctx context.Context, workspaceID, principalID, permission string) (bool, error) {
	if principalID == "" {
		return false, nil
	}
	rows, err := r.q.ListPrincipalRolePermissions(ctx, platformdb.ListPrincipalRolePermissionsParams{
		WorkspaceID: workspaceID,
		PrincipalID: sql.NullString{String: principalID, Valid: true},
	})
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		var permissions []string
		if err := json.Unmarshal([]byte(row), &permissions); err != nil {
			return false, err
		}
		for _, candidate := range permissions {
			if candidate == permission {
				return true, nil
			}
		}
	}
	return false, nil
}

func mapPrincipal(row platformdb.Principal) access.Principal {
	return access.Principal{
		ID:          row.ID,
		Email:       row.Email,
		DisplayName: row.DisplayName,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func nullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func newID(prefix string) string {
	return prefix + "_" + newSecret()[:24]
}

func newSecret() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		sum := sha256.Sum256([]byte(time.Now().Format(time.RFC3339Nano)))
		return hex.EncodeToString(sum[:])
	}
	return hex.EncodeToString(b[:])
}
