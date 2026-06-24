package sqlite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Yacobolo/libredash/internal/access"
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

func (r *Repository) UpsertPrincipal(ctx context.Context, input access.PrincipalInput) (access.Principal, error) {
	if strings.TrimSpace(input.ID) == "" {
		input.ID = newID("principal")
	}
	if err := r.q.UpsertPrincipal(ctx, platformdb.UpsertPrincipalParams{
		ID:          input.ID,
		Email:       input.Email,
		DisplayName: input.DisplayName,
	}); err != nil {
		return access.Principal{}, err
	}
	row, err := r.q.GetPrincipal(ctx, input.ID)
	if err != nil {
		return access.Principal{}, err
	}
	return mapPrincipal(row), nil
}

func (r *Repository) SetPrincipalRole(ctx context.Context, input access.PrincipalRoleInput) (access.Principal, error) {
	email := access.NormalizeEmail(input.Email)
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
	principal, err := r.UpsertPrincipal(ctx, access.PrincipalInput{
		ID:          access.PrincipalIDForEmail(email),
		Email:       email,
		DisplayName: firstNonEmpty(strings.TrimSpace(input.DisplayName), email),
	})
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
	return principal, nil
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

func (r *Repository) BootstrapAdmin(ctx context.Context, workspaceID, email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	principal, err := r.UpsertPrincipal(ctx, access.PrincipalInput{
		ID:          access.PrincipalIDForEmail(email),
		Email:       email,
		DisplayName: email,
	})
	if err != nil {
		return err
	}
	role, err := r.q.GetRoleByName(ctx, access.RoleOwner)
	if err != nil {
		return err
	}
	return r.q.InsertRoleBinding(ctx, platformdb.InsertRoleBindingParams{
		ID:          newID("rolebinding"),
		WorkspaceID: workspaceID,
		RoleID:      role.ID,
		PrincipalID: sql.NullString{String: principal.ID, Valid: principal.ID != ""},
	})
}

func (r *Repository) ResolveExternalPrincipal(ctx context.Context, input access.ExternalIdentityInput) (access.Principal, error) {
	input.Email = access.NormalizeEmail(input.Email)
	if input.Provider == "" || input.Subject == "" {
		return access.Principal{}, fmt.Errorf("external identity requires provider and subject")
	}
	identity, err := r.q.GetExternalIdentity(ctx, platformdb.GetExternalIdentityParams{
		Provider: input.Provider,
		TenantID: input.TenantID,
		Subject:  input.Subject,
	})
	if err == nil {
		return r.UpsertPrincipal(ctx, access.PrincipalInput{
			ID:          identity.PrincipalID,
			Email:       input.Email,
			DisplayName: input.DisplayName,
		})
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return access.Principal{}, err
	}

	var principal access.Principal
	if input.Email != "" {
		row, err := r.q.GetPrincipalByEmail(ctx, input.Email)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return access.Principal{}, err
		}
		if err == nil {
			principal = mapPrincipal(row)
		}
	}
	if principal.ID == "" {
		principal, err = r.UpsertPrincipal(ctx, access.PrincipalInput{
			ID:          "external_" + stableID(input.Provider+"|"+input.TenantID+"|"+input.Subject),
			Email:       input.Email,
			DisplayName: input.DisplayName,
		})
		if err != nil {
			return access.Principal{}, err
		}
	} else {
		principal, err = r.UpsertPrincipal(ctx, access.PrincipalInput{
			ID:          principal.ID,
			Email:       input.Email,
			DisplayName: input.DisplayName,
		})
		if err != nil {
			return access.Principal{}, err
		}
	}

	if err := r.q.UpsertExternalIdentity(ctx, platformdb.UpsertExternalIdentityParams{
		ID:          "identity_" + stableID(input.Provider+"|"+input.TenantID+"|"+input.Subject),
		PrincipalID: principal.ID,
		Provider:    input.Provider,
		TenantID:    input.TenantID,
		Subject:     input.Subject,
		Email:       input.Email,
	}); err != nil {
		return access.Principal{}, err
	}
	return principal, nil
}

func (r *Repository) CreateSession(ctx context.Context, principalID string, ttl time.Duration) (string, error) {
	token := newSecret()
	expires := time.Now().Add(ttl).UTC().Format(time.RFC3339)
	return token, r.q.CreateSession(ctx, platformdb.CreateSessionParams{
		ID:          newID("session"),
		PrincipalID: principalID,
		TokenHash:   tokenHash(token),
		ExpiresAt:   expires,
	})
}

func (r *Repository) PrincipalForToken(ctx context.Context, token string) (access.Principal, error) {
	session, err := r.q.GetSessionByTokenHash(ctx, tokenHash(token))
	if err != nil {
		return access.Principal{}, err
	}
	_ = r.q.TouchSession(ctx, session.ID)
	row, err := r.q.GetPrincipal(ctx, session.PrincipalID)
	if err != nil {
		return access.Principal{}, err
	}
	return mapPrincipal(row), nil
}

func (r *Repository) DeleteSession(ctx context.Context, token string) error {
	return r.q.DeleteSessionByTokenHash(ctx, tokenHash(token))
}

func (r *Repository) CreateAPIToken(ctx context.Context, principalID, name string) (string, error) {
	token := newSecret()
	return token, r.q.CreateAPIToken(ctx, platformdb.CreateAPITokenParams{
		ID:          newID("token"),
		PrincipalID: principalID,
		Name:        name,
		TokenHash:   tokenHash(token),
	})
}

func (r *Repository) PrincipalForAPIToken(ctx context.Context, token string) (access.Principal, error) {
	apiToken, err := r.q.GetAPITokenByHash(ctx, tokenHash(token))
	if err != nil {
		return access.Principal{}, err
	}
	_ = r.q.TouchAPIToken(ctx, apiToken.ID)
	row, err := r.q.GetPrincipal(ctx, apiToken.PrincipalID)
	if err != nil {
		return access.Principal{}, err
	}
	return mapPrincipal(row), nil
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

func stableID(value string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(value)))
	return hex.EncodeToString(sum[:])[:32]
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
