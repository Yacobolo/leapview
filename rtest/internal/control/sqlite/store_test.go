package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/flidai/leapview/rtest/internal/control"
	controlsqlite "github.com/flidai/leapview/rtest/internal/control/sqlite"
)

func TestOpenMigratesAdmissionIdempotencyColumns(t *testing.T) {
	root := t.TempDir()
	database, err := sql.Open("sqlite", filepath.Join(root, "control.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`
CREATE TABLE control_jobs (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL, image TEXT NOT NULL, command_json TEXT NOT NULL,
  working_directory TEXT NOT NULL, environment_json TEXT NOT NULL, root_digest TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL, timeout_millis INTEGER NOT NULL, cpus TEXT NOT NULL, memory TEXT NOT NULL,
  created_at INTEGER NOT NULL, started_at INTEGER, finished_at INTEGER, exit_code INTEGER,
  error_message TEXT NOT NULL DEFAULT '', cancel_requested INTEGER NOT NULL DEFAULT 0, worker_id TEXT NOT NULL DEFAULT ''
);
CREATE TABLE control_builds (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL, status TEXT NOT NULL, created_at INTEGER NOT NULL,
  finished_at INTEGER, exit_code INTEGER
);`)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := controlsqlite.Open(root, []byte("test-pepper-that-is-at-least-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	database, err = sql.Open("sqlite", filepath.Join(root, "control.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for _, table := range []string{"control_jobs", "control_builds"} {
		rows, err := database.Query(`PRAGMA table_info(` + table + `)`)
		if err != nil {
			t.Fatal(err)
		}
		columns := map[string]bool{}
		for rows.Next() {
			var cid, notNull, primaryKey int
			var name, kind string
			var defaultValue any
			if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
				t.Fatal(err)
			}
			columns[name] = true
		}
		_ = rows.Close()
		if !columns["idempotency_key"] || !columns["request_hash"] {
			t.Fatalf("%s columns = %#v", table, columns)
		}
	}
}

func TestBootstrapTokenAuthenticatesWithoutPersistingSecret(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	bootstrap, err := store.Bootstrap(ctx, control.Bootstrap{
		UserName: "Owner", ProjectSlug: "example-service", ProjectName: "Example Service", TokenName: "owner-laptop",
	})
	if err != nil {
		t.Fatal(err)
	}
	if bootstrap.Token == "" || bootstrap.User.ID == "" || bootstrap.Project.ID == "" {
		t.Fatalf("bootstrap = %#v", bootstrap)
	}

	principal, err := store.Authenticate(ctx, bootstrap.Token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.UserID != bootstrap.User.ID || !principal.Admin || principal.Kind != control.PrincipalDevice {
		t.Fatalf("principal = %#v", principal)
	}
	project, err := store.AuthorizeProject(ctx, principal, "example-service")
	if err != nil || project.ID != bootstrap.Project.ID {
		t.Fatalf("project=%#v err=%v", project, err)
	}

	database, err := sql.Open("sqlite", filepath.Join(store.Root(), "control.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var persisted string
	if err := database.QueryRowContext(ctx, `SELECT digest FROM access_tokens LIMIT 1`).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if persisted == bootstrap.Token || persisted == "" {
		t.Fatalf("persisted token material = %q", persisted)
	}
}

func TestDeviceTokensAreIndependentlyRevocable(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	bootstrap, err := store.Bootstrap(ctx, control.Bootstrap{
		UserName: "Owner", ProjectSlug: "example-service", ProjectName: "Example Service", TokenName: "first",
	})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := store.Authenticate(ctx, bootstrap.Token)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateDeviceToken(ctx, principal, control.CreateDeviceToken{
		UserID: bootstrap.User.ID, Name: "second", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(ctx, second.Secret); err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeDeviceToken(ctx, principal, second.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(ctx, second.Secret); !errors.Is(err, control.ErrUnauthenticated) {
		t.Fatalf("revoked authentication error = %v", err)
	}
	if _, err := store.Authenticate(ctx, bootstrap.Token); err != nil {
		t.Fatalf("first device token was affected: %v", err)
	}
}

func TestProjectMembershipAndGitHubTrustAreScoped(t *testing.T) {
	store := openStore(t)
	ctx := context.Background()
	bootstrap, err := store.Bootstrap(ctx, control.Bootstrap{
		UserName: "Owner", ProjectSlug: "one", ProjectName: "One", TokenName: "owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	owner, _ := store.Authenticate(ctx, bootstrap.Token)
	second, err := store.CreateProject(ctx, owner, "two", "Two")
	if err != nil {
		t.Fatal(err)
	}
	member, err := store.CreateUser(ctx, owner, "Member", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddProjectMember(ctx, owner, bootstrap.Project.ID, member.ID); err != nil {
		t.Fatal(err)
	}
	memberToken, err := store.CreateDeviceToken(ctx, owner, control.CreateDeviceToken{UserID: member.ID, Name: "member"})
	if err != nil {
		t.Fatal(err)
	}
	memberPrincipal, _ := store.Authenticate(ctx, memberToken.Secret)
	if _, err := store.AuthorizeProject(ctx, memberPrincipal, bootstrap.Project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthorizeProject(ctx, memberPrincipal, second.ID); !errors.Is(err, control.ErrForbidden) {
		t.Fatalf("other project authorization error = %v", err)
	}

	trust, err := store.CreateGitHubTrust(ctx, owner, control.GitHubTrust{
		ProjectID: bootstrap.Project.ID, RepositoryOwnerID: "100", RepositoryID: "200",
		WorkflowRef: "flidai/leapview/.github/workflows/ci.yml@refs/heads/*", Ref: "refs/heads/*",
		Environment: "rtest", Events: []string{"push", "workflow_dispatch"},
	})
	if err != nil {
		t.Fatal(err)
	}
	matched, err := store.MatchGitHubTrust(ctx, bootstrap.Project.ID, control.GitHubClaims{
		RepositoryOwnerID: "100", RepositoryID: "200",
		WorkflowRef: "flidai/leapview/.github/workflows/ci.yml@refs/heads/main", Ref: "refs/heads/main",
		Environment: "rtest", EventName: "push",
	})
	if err != nil || matched.ID != trust.ID {
		t.Fatalf("matched=%#v err=%v", matched, err)
	}
	if _, err := store.MatchGitHubTrust(ctx, bootstrap.Project.ID, control.GitHubClaims{
		RepositoryOwnerID: "100", RepositoryID: "999", WorkflowRef: "x", Ref: "refs/heads/main", Environment: "rtest", EventName: "push",
	}); !errors.Is(err, control.ErrForbidden) {
		t.Fatalf("mismatched repository error = %v", err)
	}
}

func openStore(t *testing.T) *controlsqlite.Store {
	t.Helper()
	store, err := controlsqlite.Open(t.TempDir(), []byte("test-pepper-that-is-at-least-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
