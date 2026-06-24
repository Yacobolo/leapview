package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/platform"
	"github.com/Yacobolo/libredash/internal/workspace"
	workspacesqlite "github.com/Yacobolo/libredash/internal/workspace/sqlite"
)

func TestRepositoryChecksRBAC(t *testing.T) {
	ctx := context.Background()
	_, repo := openAccessRepo(t, ctx)

	principal, err := repo.SetPrincipalRole(ctx, access.PrincipalRoleInput{
		WorkspaceID: "test",
		Email:       "owner@example.com",
		DisplayName: "Owner",
		Role:        "owner",
	})
	if err != nil {
		t.Fatalf("set principal role: %v", err)
	}
	allowed, err := repo.HasPermission(ctx, "test", principal.ID, access.PermissionDeploymentActivate)
	if err != nil {
		t.Fatalf("check permission: %v", err)
	}
	if !allowed {
		t.Fatal("owner missing deployment activation permission")
	}
}

func TestRepositoryResolveExternalPrincipalAttachesBootstrappedEmail(t *testing.T) {
	ctx := context.Background()
	_, repo := openAccessRepo(t, ctx)

	if err := repo.BootstrapAdmin(ctx, "test", "owner@example.com"); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}
	principal, err := repo.ResolveExternalPrincipal(ctx, access.ExternalIdentityInput{
		Provider:    "azureadv2",
		TenantID:    "tenant",
		Subject:     "object-id",
		Email:       "OWNER@example.com",
		DisplayName: "Owner",
	})
	if err != nil {
		t.Fatalf("resolve external principal: %v", err)
	}
	if principal.ID != access.PrincipalIDForEmail("owner@example.com") {
		t.Fatalf("principal id = %q, want bootstrapped email principal", principal.ID)
	}
	allowed, err := repo.HasPermission(ctx, "test", principal.ID, access.PermissionDeploymentActivate)
	if err != nil {
		t.Fatalf("check permission: %v", err)
	}
	if !allowed {
		t.Fatal("attached Azure identity did not inherit owner permissions")
	}

	again, err := repo.ResolveExternalPrincipal(ctx, access.ExternalIdentityInput{
		Provider:    "azureadv2",
		TenantID:    "tenant",
		Subject:     "object-id",
		Email:       "owner@example.com",
		DisplayName: "Owner Updated",
	})
	if err != nil {
		t.Fatalf("resolve existing identity: %v", err)
	}
	if again.ID != principal.ID {
		t.Fatalf("existing identity principal = %q, want %q", again.ID, principal.ID)
	}
}

func TestRepositoryBootstrapAdminIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store, repo := openAccessRepo(t, ctx)

	for i := 0; i < 2; i++ {
		if err := repo.BootstrapAdmin(ctx, "test", "owner@example.com"); err != nil {
			t.Fatalf("bootstrap admin %d: %v", i, err)
		}
	}
	var count int
	err := store.SQLDB().QueryRowContext(ctx, `
		SELECT count(*)
		FROM role_bindings rb
		JOIN roles r ON r.id = rb.role_id
		WHERE rb.workspace_id = ? AND rb.principal_id = ? AND r.name = ?
	`, "test", access.PrincipalIDForEmail("owner@example.com"), "owner").Scan(&count)
	if err != nil {
		t.Fatalf("count role bindings: %v", err)
	}
	if count != 1 {
		t.Fatalf("owner role bindings = %d, want 1", count)
	}
}

func TestRepositoryResolveExternalPrincipalWithoutEmailCreatesUnprivilegedPrincipal(t *testing.T) {
	ctx := context.Background()
	_, repo := openAccessRepo(t, ctx)

	principal, err := repo.ResolveExternalPrincipal(ctx, access.ExternalIdentityInput{
		Provider:    "azureadv2",
		TenantID:    "tenant",
		Subject:     "new-object-id",
		DisplayName: "New User",
	})
	if err != nil {
		t.Fatalf("resolve external principal: %v", err)
	}
	allowed, err := repo.HasPermission(ctx, "test", principal.ID, access.PermissionDeploymentActivate)
	if err != nil {
		t.Fatalf("check permission: %v", err)
	}
	if allowed {
		t.Fatal("new external principal unexpectedly has deployment activation permission")
	}
}

func TestRepositorySessionsAndAPITokensResolvePrincipals(t *testing.T) {
	ctx := context.Background()
	_, repo := openAccessRepo(t, ctx)
	principal, err := repo.SetPrincipalRole(ctx, access.PrincipalRoleInput{
		WorkspaceID: "test",
		Email:       "viewer@example.com",
		DisplayName: "Viewer",
		Role:        "viewer",
	})
	if err != nil {
		t.Fatalf("set principal role: %v", err)
	}

	sessionToken, err := repo.CreateSession(ctx, principal.ID, time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sessionPrincipal, err := repo.PrincipalForToken(ctx, sessionToken)
	if err != nil {
		t.Fatalf("principal for session: %v", err)
	}
	if sessionPrincipal.ID != principal.ID {
		t.Fatalf("session principal = %q, want %q", sessionPrincipal.ID, principal.ID)
	}

	apiToken, err := repo.CreateAPIToken(ctx, principal.ID, "test")
	if err != nil {
		t.Fatalf("create api token: %v", err)
	}
	apiPrincipal, err := repo.PrincipalForAPIToken(ctx, apiToken)
	if err != nil {
		t.Fatalf("principal for api token: %v", err)
	}
	if apiPrincipal.ID != principal.ID {
		t.Fatalf("api token principal = %q, want %q", apiPrincipal.ID, principal.ID)
	}
}

func openAccessRepo(t *testing.T, ctx context.Context) (*platform.Store, *Repository) {
	t.Helper()
	store, err := platform.Open(ctx, filepath.Join(t.TempDir(), "libredash.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := workspacesqlite.NewRepository(store.SQLDB()).Ensure(ctx, workspace.EnsureInput{ID: "test", Title: "Test"}); err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	return store, NewRepository(store.SQLDB())
}
