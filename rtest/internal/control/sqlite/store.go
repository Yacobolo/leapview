package sqlite

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/flidai/leapview/rtest/internal/control"
	"github.com/flidai/leapview/rtest/internal/protocol"
	_ "modernc.org/sqlite"
)

type Store struct {
	db     *sql.DB
	root   string
	pepper []byte
}

func Open(root string, pepper []byte) (*Store, error) {
	if len(pepper) < 32 {
		return nil, errors.New("token pepper must contain at least 32 bytes")
	}
	if err := ensurePrivateDir(root); err != nil {
		return nil, err
	}
	databasePath := filepath.Join(root, "control.db")
	db, err := sql.Open("sqlite", databasePath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db, root: root, pepper: append([]byte(nil), pepper...)}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func ensurePrivateDir(root string) error {
	if root == "" {
		return errors.New("control store root is required")
	}
	return os.MkdirAll(root, 0o700)
}

func (s *Store) Root() string { return s.root }

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Initialized(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  admin INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS project_members (
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at INTEGER NOT NULL,
  PRIMARY KEY(project_id, user_id)
);
CREATE TABLE IF NOT EXISTS access_tokens (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  subject TEXT NOT NULL DEFAULT '',
  digest TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  expires_at INTEGER,
  last_used_at INTEGER,
  revoked_at INTEGER
);
CREATE INDEX IF NOT EXISTS access_tokens_user_idx ON access_tokens(user_id, created_at);
CREATE TABLE IF NOT EXISTS github_trusts (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  repository_owner_id TEXT NOT NULL,
  repository_id TEXT NOT NULL,
  workflow_ref TEXT NOT NULL,
  ref_pattern TEXT NOT NULL,
  environment TEXT NOT NULL,
  events_json TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  revoked_at INTEGER
);
CREATE INDEX IF NOT EXISTS github_trust_lookup_idx ON github_trusts(project_id, repository_owner_id, repository_id);
CREATE TABLE IF NOT EXISTS audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  actor_kind TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  project_id TEXT,
  action TEXT NOT NULL,
  target_id TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  metadata_json TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS control_jobs (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  image TEXT NOT NULL,
  command_json TEXT NOT NULL,
  working_directory TEXT NOT NULL,
  environment_json TEXT NOT NULL,
  root_digest TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  timeout_millis INTEGER NOT NULL,
  cpus TEXT NOT NULL,
  memory TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  started_at INTEGER,
  finished_at INTEGER,
  exit_code INTEGER,
  error_message TEXT NOT NULL DEFAULT '',
  cancel_requested INTEGER NOT NULL DEFAULT 0,
  worker_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  request_hash TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS control_jobs_project_idx ON control_jobs(project_id, created_at DESC);
CREATE TABLE IF NOT EXISTS control_builds (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  finished_at INTEGER,
  exit_code INTEGER,
  idempotency_key TEXT NOT NULL DEFAULT '',
  request_hash TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS control_builds_project_idx ON control_builds(project_id, created_at DESC);
`)
	if err != nil {
		return err
	}
	for _, column := range []struct {
		table, name string
	}{
		{"control_jobs", "idempotency_key"}, {"control_jobs", "request_hash"},
		{"control_builds", "idempotency_key"}, {"control_builds", "request_hash"},
	} {
		if err := s.ensureTextColumn(ctx, column.table, column.name); err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS control_jobs_idempotency_idx ON control_jobs(project_id,idempotency_key) WHERE idempotency_key <> '';
CREATE UNIQUE INDEX IF NOT EXISTS control_builds_idempotency_idx ON control_builds(project_id,idempotency_key) WHERE idempotency_key <> '';
`)
	return err
}

func (s *Store) ensureTextColumn(ctx context.Context, table, column string) error {
	if table != "control_jobs" && table != "control_builds" {
		return errors.New("unsupported migration table")
	}
	if column != "idempotency_key" && column != "request_hash" {
		return errors.New("unsupported migration column")
	}
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		found = found || name == column
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *Store) Bootstrap(ctx context.Context, input control.Bootstrap) (control.BootstrapResult, error) {
	if strings.TrimSpace(input.UserName) == "" || strings.TrimSpace(input.ProjectSlug) == "" || strings.TrimSpace(input.TokenName) == "" {
		return control.BootstrapResult{}, errors.New("bootstrap user, project slug, and token name are required")
	}
	if input.ProjectName == "" {
		input.ProjectName = input.ProjectSlug
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return control.BootstrapResult{}, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return control.BootstrapResult{}, err
	}
	if count != 0 {
		return control.BootstrapResult{}, control.ErrAlreadyExists
	}
	userID, err := randomID("usr")
	if err != nil {
		return control.BootstrapResult{}, err
	}
	projectID, err := randomID("prj")
	if err != nil {
		return control.BootstrapResult{}, err
	}
	now := time.Now().UTC()
	user := control.User{ID: userID, Name: input.UserName, Admin: true, CreatedAt: now}
	project := control.Project{ID: projectID, Slug: input.ProjectSlug, Name: input.ProjectName, CreatedAt: now}
	if _, err := tx.ExecContext(ctx, `INSERT INTO users(id,name,admin,created_at) VALUES(?,?,1,?)`, user.ID, user.Name, unix(now)); err != nil {
		return control.BootstrapResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO projects(id,slug,name,created_at) VALUES(?,?,?,?)`, project.ID, project.Slug, project.Name, unix(now)); err != nil {
		return control.BootstrapResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_members(project_id,user_id,created_at) VALUES(?,?,?)`, project.ID, user.ID, unix(now)); err != nil {
		return control.BootstrapResult{}, err
	}
	tokenID, secret, digest, err := s.newToken("dt")
	if err != nil {
		return control.BootstrapResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO access_tokens(id,kind,user_id,name,digest,created_at) VALUES(?,?,?,?,?,?)`,
		tokenID, control.PrincipalDevice, user.ID, input.TokenName, digest, unix(now)); err != nil {
		return control.BootstrapResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return control.BootstrapResult{}, err
	}
	return control.BootstrapResult{User: user, Project: project, Token: secret}, nil
}

func (s *Store) Authenticate(ctx context.Context, token string) (control.Principal, error) {
	kind, id, ok := parseToken(token)
	if !ok {
		return control.Principal{}, control.ErrUnauthenticated
	}
	var storedKind, userID, projectID, subject, digest string
	var expires, revoked sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT kind,COALESCE(user_id,''),COALESCE(project_id,''),subject,digest,expires_at,revoked_at FROM access_tokens WHERE id=?`, id).
		Scan(&storedKind, &userID, &projectID, &subject, &digest, &expires, &revoked)
	if err != nil {
		return control.Principal{}, control.ErrUnauthenticated
	}
	if storedKind != string(kind) || revoked.Valid || expires.Valid && time.Now().UTC().After(fromUnix(expires.Int64)) {
		return control.Principal{}, control.ErrUnauthenticated
	}
	want := s.digest(token)
	if len(want) != len(digest) || subtle.ConstantTimeCompare([]byte(want), []byte(digest)) != 1 {
		return control.Principal{}, control.ErrUnauthenticated
	}
	principal := control.Principal{Kind: kind, TokenID: id, UserID: userID, ProjectID: projectID, Subject: subject}
	if userID != "" {
		var admin int
		if err := s.db.QueryRowContext(ctx, `SELECT admin FROM users WHERE id=?`, userID).Scan(&admin); err != nil {
			return control.Principal{}, control.ErrUnauthenticated
		}
		principal.Admin = admin != 0
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE access_tokens SET last_used_at=? WHERE id=?`, unix(time.Now().UTC()), id)
	return principal, nil
}

func (s *Store) AuthorizeProject(ctx context.Context, principal control.Principal, selector string) (control.Project, error) {
	project, err := s.project(ctx, selector)
	if err != nil {
		return control.Project{}, err
	}
	if principal.ProjectID != "" {
		if principal.ProjectID != project.ID {
			return control.Project{}, control.ErrForbidden
		}
		return project, nil
	}
	if principal.UserID == "" {
		return control.Project{}, control.ErrForbidden
	}
	if principal.Admin {
		return project, nil
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM project_members WHERE project_id=? AND user_id=?`, project.ID, principal.UserID).Scan(&exists); err != nil {
		return control.Project{}, control.ErrForbidden
	}
	return project, nil
}

func (s *Store) CreateUser(ctx context.Context, principal control.Principal, name string, admin bool) (control.User, error) {
	if !principal.Admin {
		return control.User{}, control.ErrForbidden
	}
	if strings.TrimSpace(name) == "" {
		return control.User{}, errors.New("user name is required")
	}
	id, err := randomID("usr")
	if err != nil {
		return control.User{}, err
	}
	now := time.Now().UTC()
	user := control.User{ID: id, Name: name, Admin: admin, CreatedAt: now}
	_, err = s.db.ExecContext(ctx, `INSERT INTO users(id,name,admin,created_at) VALUES(?,?,?,?)`, user.ID, user.Name, boolInt(user.Admin), unix(now))
	return user, err
}

func (s *Store) CreateProject(ctx context.Context, principal control.Principal, slug, name string) (control.Project, error) {
	if !principal.Admin {
		return control.Project{}, control.ErrForbidden
	}
	if strings.TrimSpace(slug) == "" {
		return control.Project{}, errors.New("project slug is required")
	}
	if name == "" {
		name = slug
	}
	id, err := randomID("prj")
	if err != nil {
		return control.Project{}, err
	}
	now := time.Now().UTC()
	project := control.Project{ID: id, Slug: slug, Name: name, CreatedAt: now}
	_, err = s.db.ExecContext(ctx, `INSERT INTO projects(id,slug,name,created_at) VALUES(?,?,?,?)`, project.ID, project.Slug, project.Name, unix(now))
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return control.Project{}, control.ErrAlreadyExists
	}
	return project, err
}

func (s *Store) ListProjects(ctx context.Context, principal control.Principal) ([]control.Project, error) {
	var rows *sql.Rows
	var err error
	switch {
	case principal.ProjectID != "":
		rows, err = s.db.QueryContext(ctx, `SELECT id,slug,name,created_at FROM projects WHERE id=? ORDER BY slug,id`, principal.ProjectID)
	case principal.UserID == "":
		return nil, control.ErrForbidden
	case principal.Admin:
		rows, err = s.db.QueryContext(ctx, `SELECT id,slug,name,created_at FROM projects ORDER BY slug,id`)
	default:
		rows, err = s.db.QueryContext(ctx, `SELECT p.id,p.slug,p.name,p.created_at FROM projects p JOIN project_members m ON m.project_id=p.id WHERE m.user_id=? ORDER BY p.slug,p.id`, principal.UserID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]control.Project, 0)
	for rows.Next() {
		var project control.Project
		var created int64
		if err := rows.Scan(&project.ID, &project.Slug, &project.Name, &created); err != nil {
			return nil, err
		}
		project.CreatedAt = fromUnix(created)
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *Store) AddProjectMember(ctx context.Context, principal control.Principal, projectSelector, userID string) error {
	if !principal.Admin {
		return control.ErrForbidden
	}
	project, err := s.AuthorizeProject(ctx, principal, projectSelector)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO project_members(project_id,user_id,created_at) VALUES(?,?,?)`, project.ID, userID, unix(time.Now().UTC()))
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		var exists int
		if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id=?`, userID).Scan(&exists); err != nil {
			return control.ErrNotFound
		}
	}
	return nil
}

func (s *Store) CreateDeviceToken(ctx context.Context, principal control.Principal, input control.CreateDeviceToken) (control.IssuedDeviceToken, error) {
	if principal.Kind != control.PrincipalDevice || principal.UserID == "" {
		return control.IssuedDeviceToken{}, control.ErrForbidden
	}
	if input.UserID == "" {
		input.UserID = principal.UserID
	}
	if input.UserID != principal.UserID && !principal.Admin {
		return control.IssuedDeviceToken{}, control.ErrForbidden
	}
	if strings.TrimSpace(input.Name) == "" {
		return control.IssuedDeviceToken{}, errors.New("device token name is required")
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id=?`, input.UserID).Scan(&exists); err != nil {
		return control.IssuedDeviceToken{}, control.ErrNotFound
	}
	id, secret, digest, err := s.newToken("dt")
	if err != nil {
		return control.IssuedDeviceToken{}, err
	}
	now := time.Now().UTC()
	var expires any
	var expiresPtr *time.Time
	if !input.ExpiresAt.IsZero() {
		value := input.ExpiresAt.UTC()
		expires, expiresPtr = unix(value), &value
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO access_tokens(id,kind,user_id,name,digest,created_at,expires_at) VALUES(?,?,?,?,?,?,?)`,
		id, control.PrincipalDevice, input.UserID, input.Name, digest, unix(now), expires)
	if err != nil {
		return control.IssuedDeviceToken{}, err
	}
	return control.IssuedDeviceToken{Metadata: control.DeviceToken{ID: id, Name: input.Name, UserID: input.UserID, CreatedAt: now, ExpiresAt: expiresPtr}, Secret: secret}, nil
}

func (s *Store) ListDeviceTokens(ctx context.Context, principal control.Principal) ([]control.DeviceToken, error) {
	if principal.Kind != control.PrincipalDevice || principal.UserID == "" {
		return nil, control.ErrForbidden
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,user_id,created_at,expires_at,last_used_at,revoked_at FROM access_tokens WHERE kind=? AND user_id=? ORDER BY created_at DESC`, control.PrincipalDevice, principal.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tokens []control.DeviceToken
	for rows.Next() {
		var item control.DeviceToken
		var created int64
		var expires, used, revoked sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Name, &item.UserID, &created, &expires, &used, &revoked); err != nil {
			return nil, err
		}
		item.CreatedAt = fromUnix(created)
		item.ExpiresAt, item.LastUsedAt, item.RevokedAt = nullableTime(expires), nullableTime(used), nullableTime(revoked)
		tokens = append(tokens, item)
	}
	return tokens, rows.Err()
}

func (s *Store) RevokeDeviceToken(ctx context.Context, principal control.Principal, id string) error {
	if principal.Kind != control.PrincipalDevice || principal.UserID == "" {
		return control.ErrForbidden
	}
	query := `UPDATE access_tokens SET revoked_at=? WHERE id=? AND kind=? AND revoked_at IS NULL`
	args := []any{unix(time.Now().UTC()), id, control.PrincipalDevice}
	if !principal.Admin {
		query += ` AND user_id=?`
		args = append(args, principal.UserID)
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return control.ErrNotFound
	}
	return nil
}

func (s *Store) CreateProjectSession(ctx context.Context, projectID, subject string, expiresAt time.Time) (control.IssuedAccessToken, error) {
	id, secret, digest, err := s.newToken("gh")
	if err != nil {
		return control.IssuedAccessToken{}, err
	}
	now := time.Now().UTC()
	expiresAt = expiresAt.UTC()
	_, err = s.db.ExecContext(ctx, `INSERT INTO access_tokens(id,kind,project_id,name,subject,digest,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?)`,
		id, control.PrincipalGitHub, projectID, "github-oidc", subject, digest, unix(now), unix(expiresAt))
	if err != nil {
		return control.IssuedAccessToken{}, err
	}
	return control.IssuedAccessToken{Token: secret, ExpiresAt: expiresAt}, nil
}

func (s *Store) CreateGitHubTrust(ctx context.Context, principal control.Principal, trust control.GitHubTrust) (control.GitHubTrust, error) {
	if !principal.Admin {
		return control.GitHubTrust{}, control.ErrForbidden
	}
	project, err := s.AuthorizeProject(ctx, principal, trust.ProjectID)
	if err != nil {
		return control.GitHubTrust{}, err
	}
	if trust.RepositoryOwnerID == "" || trust.RepositoryID == "" || trust.WorkflowRef == "" || trust.Ref == "" || len(trust.Events) == 0 {
		return control.GitHubTrust{}, errors.New("repository owner ID, repository ID, workflow ref, ref, and events are required")
	}
	events, _ := json.Marshal(trust.Events)
	trustID, err := randomID("ght")
	if err != nil {
		return control.GitHubTrust{}, err
	}
	trust.ID, trust.ProjectID, trust.CreatedAt = trustID, project.ID, time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `INSERT INTO github_trusts(id,project_id,repository_owner_id,repository_id,workflow_ref,ref_pattern,environment,events_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		trust.ID, trust.ProjectID, trust.RepositoryOwnerID, trust.RepositoryID, trust.WorkflowRef, trust.Ref, trust.Environment, string(events), unix(trust.CreatedAt))
	return trust, err
}

func (s *Store) ListGitHubTrusts(ctx context.Context, principal control.Principal, projectSelector string) ([]control.GitHubTrust, error) {
	project, err := s.AuthorizeProject(ctx, principal, projectSelector)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,repository_owner_id,repository_id,workflow_ref,ref_pattern,environment,events_json,created_at,revoked_at FROM github_trusts WHERE project_id=? ORDER BY created_at`, project.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []control.GitHubTrust
	for rows.Next() {
		trust, err := scanTrust(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, trust)
	}
	return result, rows.Err()
}

func (s *Store) RevokeGitHubTrust(ctx context.Context, principal control.Principal, id string) error {
	if !principal.Admin {
		return control.ErrForbidden
	}
	result, err := s.db.ExecContext(ctx, `UPDATE github_trusts SET revoked_at=? WHERE id=? AND revoked_at IS NULL`, unix(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return control.ErrNotFound
	}
	return nil
}

func (s *Store) MatchGitHubTrust(ctx context.Context, projectSelector string, claims control.GitHubClaims) (control.GitHubTrust, error) {
	project, err := s.project(ctx, projectSelector)
	if err != nil {
		return control.GitHubTrust{}, control.ErrForbidden
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,repository_owner_id,repository_id,workflow_ref,ref_pattern,environment,events_json,created_at,revoked_at FROM github_trusts WHERE project_id=? AND repository_owner_id=? AND repository_id=? AND revoked_at IS NULL`,
		project.ID, claims.RepositoryOwnerID, claims.RepositoryID)
	if err != nil {
		return control.GitHubTrust{}, err
	}
	defer rows.Close()
	for rows.Next() {
		trust, err := scanTrust(rows)
		if err != nil {
			return control.GitHubTrust{}, err
		}
		if glob(trust.WorkflowRef, claims.WorkflowRef) && glob(trust.Ref, claims.Ref) && exactOptional(trust.Environment, claims.Environment) && contains(trust.Events, claims.EventName) {
			return trust, nil
		}
	}
	return control.GitHubTrust{}, control.ErrForbidden
}

func (s *Store) Audit(ctx context.Context, principal control.Principal, projectID, action, targetID string, metadata map[string]string) error {
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	actorID := principal.UserID
	if actorID == "" {
		actorID = principal.Subject
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO audit_events(actor_kind,actor_id,project_id,action,target_id,created_at,metadata_json) VALUES(?,?,?,?,?,?,?)`,
		principal.Kind, actorID, nullable(projectID), action, targetID, unix(time.Now().UTC()), string(encoded))
	return err
}

func (s *Store) CreatePreparedJob(ctx context.Context, input control.PrepareJob, idempotency control.Idempotency) (control.Job, bool, error) {
	command, err := json.Marshal(input.Command)
	if err != nil {
		return control.Job{}, false, err
	}
	environment, err := json.Marshal(input.Environment)
	if err != nil {
		return control.Job{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return control.Job{}, false, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `SELECT id,project_id,image,command_json,working_directory,environment_json,root_digest,status,timeout_millis,cpus,memory,created_at,started_at,finished_at,exit_code,error_message,cancel_requested,worker_id,request_hash FROM control_jobs WHERE project_id=? AND idempotency_key=?`, input.ProjectID, idempotency.Key)
	job, storedHash, err := scanIdempotentJob(row)
	if err == nil {
		if storedHash != idempotency.RequestHash {
			return control.Job{}, false, control.ErrIdempotencyConflict
		}
		return job, true, nil
	}
	if !errors.Is(err, control.ErrNotFound) {
		return control.Job{}, false, err
	}
	id, err := randomID("job")
	if err != nil {
		return control.Job{}, false, err
	}
	now := time.Now().UTC()
	job = control.Job{
		ID: id, ProjectID: input.ProjectID, Image: input.Image,
		Command: append([]string(nil), input.Command...), WorkingDirectory: input.WorkingDirectory,
		Environment: cloneMap(input.Environment), Status: protocol.StatusPreparing, Timeout: input.Timeout,
		CPUs: input.CPUs, Memory: input.Memory, CreatedAt: now,
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO control_jobs(id,project_id,image,command_json,working_directory,environment_json,status,timeout_millis,cpus,memory,created_at,idempotency_key,request_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		job.ID, job.ProjectID, job.Image, string(command), job.WorkingDirectory, string(environment), job.Status,
		job.Timeout.Milliseconds(), job.CPUs, job.Memory, unix(now), idempotency.Key, idempotency.RequestHash)
	if err != nil {
		return control.Job{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return control.Job{}, false, err
	}
	return job, false, nil
}

func (s *Store) StartJob(ctx context.Context, id, rootDigest string) (control.Job, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE control_jobs SET root_digest=?,status=? WHERE id=? AND status=?`, rootDigest, protocol.StatusQueued, id, protocol.StatusPreparing)
	if err != nil {
		return control.Job{}, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return control.Job{}, control.ErrNotFound
	}
	return s.Job(ctx, id)
}

func (s *Store) Job(ctx context.Context, id string) (control.Job, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,project_id,image,command_json,working_directory,environment_json,root_digest,status,timeout_millis,cpus,memory,created_at,started_at,finished_at,exit_code,error_message,cancel_requested,worker_id FROM control_jobs WHERE id=?`, id)
	return scanJob(row)
}

func (s *Store) ListJobs(ctx context.Context, projectID string, pageSize int, pageToken string) (control.JobPage, error) {
	if pageSize < 1 || pageSize > 100 {
		return control.JobPage{}, errors.New("job page size must be between 1 and 100")
	}
	query := `SELECT id,project_id,image,command_json,working_directory,environment_json,root_digest,status,timeout_millis,cpus,memory,created_at,started_at,finished_at,exit_code,error_message,cancel_requested,worker_id FROM control_jobs WHERE project_id=?`
	arguments := []any{projectID}
	if pageToken != "" {
		cursor, err := s.decodeJobCursor(projectID, pageToken)
		if err != nil {
			return control.JobPage{}, err
		}
		query += ` AND (created_at < ? OR (created_at = ? AND id < ?))`
		arguments = append(arguments, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT ?`
	arguments = append(arguments, pageSize+1)
	rows, err := s.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return control.JobPage{}, err
	}
	defer rows.Close()
	page := control.JobPage{Jobs: make([]control.Job, 0, pageSize)}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return control.JobPage{}, err
		}
		page.Jobs = append(page.Jobs, job)
	}
	if err := rows.Err(); err != nil {
		return control.JobPage{}, err
	}
	if len(page.Jobs) > pageSize {
		page.Jobs = page.Jobs[:pageSize]
		last := page.Jobs[len(page.Jobs)-1]
		page.NextPageToken, err = s.encodeJobCursor(projectID, last)
		if err != nil {
			return control.JobPage{}, err
		}
	}
	return page, nil
}

func (s *Store) SyncJob(ctx context.Context, id string, remote protocol.Job) (control.Job, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE control_jobs SET status=?,started_at=?,finished_at=?,exit_code=?,error_message=?,cancel_requested=?,worker_id=? WHERE id=?`,
		remote.Status, timeValue(remote.StartedAt), timeValue(remote.FinishedAt), intValue(remote.ExitCode), remote.ErrorMessage,
		boolInt(remote.CancelRequested), remote.WorkerID, id)
	if err != nil {
		return control.Job{}, err
	}
	return s.Job(ctx, id)
}

func (s *Store) RequestJobCancellation(ctx context.Context, id string) (control.Job, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE control_jobs SET cancel_requested=1,status=CASE WHEN status=? THEN ? ELSE status END,finished_at=CASE WHEN status=? THEN ? ELSE finished_at END WHERE id=? AND status NOT IN (?,?,?,?,?)`,
		protocol.StatusPreparing, protocol.StatusCancelled, protocol.StatusPreparing, unix(time.Now().UTC()), id,
		protocol.StatusSucceeded, protocol.StatusFailed, protocol.StatusCancelled, protocol.StatusTimedOut, protocol.StatusLost)
	if err != nil {
		return control.Job{}, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return control.Job{}, control.ErrNotFound
	}
	return s.Job(ctx, id)
}

func (s *Store) FailJob(ctx context.Context, id, message string) (control.Job, error) {
	exitCode := 1
	result, err := s.db.ExecContext(ctx, `UPDATE control_jobs SET status=?,finished_at=?,exit_code=?,error_message=? WHERE id=? AND status IN (?,?)`,
		protocol.StatusFailed, unix(time.Now().UTC()), exitCode, message, id, protocol.StatusPreparing, protocol.StatusQueued)
	if err != nil {
		return control.Job{}, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return control.Job{}, control.ErrNotFound
	}
	return s.Job(ctx, id)
}

func (s *Store) CreateBuild(ctx context.Context, projectID string, idempotency control.Idempotency) (control.Build, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return control.Build{}, false, err
	}
	defer tx.Rollback()
	var existing control.Build
	var created int64
	var storedHash string
	var finished, exitCode sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT id,project_id,status,created_at,finished_at,exit_code,request_hash FROM control_builds WHERE project_id=? AND idempotency_key=?`, projectID, idempotency.Key).
		Scan(&existing.ID, &existing.ProjectID, &existing.Status, &created, &finished, &exitCode, &storedHash)
	if err == nil {
		if storedHash != idempotency.RequestHash {
			return control.Build{}, false, control.ErrIdempotencyConflict
		}
		existing.CreatedAt, existing.FinishedAt = fromUnix(created), nullableTime(finished)
		if exitCode.Valid {
			value := int(exitCode.Int64)
			existing.ExitCode = &value
		}
		return existing, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return control.Build{}, false, err
	}
	id, err := randomID("bld")
	if err != nil {
		return control.Build{}, false, err
	}
	now := time.Now().UTC()
	build := control.Build{ID: id, ProjectID: projectID, Status: control.BuildRunning, CreatedAt: now}
	_, err = tx.ExecContext(ctx, `INSERT INTO control_builds(id,project_id,status,created_at,idempotency_key,request_hash) VALUES(?,?,?,?,?,?)`, build.ID, build.ProjectID, build.Status, unix(now), idempotency.Key, idempotency.RequestHash)
	if err != nil {
		return control.Build{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return control.Build{}, false, err
	}
	return build, false, nil
}

func (s *Store) Build(ctx context.Context, id string) (control.Build, error) {
	var build control.Build
	var created int64
	var finished, exitCode sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT id,project_id,status,created_at,finished_at,exit_code FROM control_builds WHERE id=?`, id).
		Scan(&build.ID, &build.ProjectID, &build.Status, &created, &finished, &exitCode)
	if errors.Is(err, sql.ErrNoRows) {
		return control.Build{}, control.ErrNotFound
	}
	build.CreatedAt, build.FinishedAt = fromUnix(created), nullableTime(finished)
	if exitCode.Valid {
		value := int(exitCode.Int64)
		build.ExitCode = &value
	}
	return build, err
}

func (s *Store) FinishBuild(ctx context.Context, id string, status control.BuildStatus, exitCode int) (control.Build, error) {
	if status != control.BuildSucceeded && status != control.BuildFailed && status != control.BuildCancelled {
		return control.Build{}, errors.New("build status is not terminal")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE control_builds SET status=?,finished_at=?,exit_code=? WHERE id=? AND status=?`, status, unix(time.Now().UTC()), exitCode, id, control.BuildRunning)
	if err != nil {
		return control.Build{}, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return control.Build{}, control.ErrNotFound
	}
	return s.Build(ctx, id)
}

func (s *Store) OperationActive(ctx context.Context, kind string, id string) bool {
	var status string
	var err error
	switch kind {
	case "job":
		err = s.db.QueryRowContext(ctx, `SELECT status FROM control_jobs WHERE id=?`, id).Scan(&status)
		return err == nil && status == string(protocol.StatusPreparing)
	case "build":
		err = s.db.QueryRowContext(ctx, `SELECT status FROM control_builds WHERE id=?`, id).Scan(&status)
		return err == nil && status == string(control.BuildRunning)
	default:
		return false
	}
}

func (s *Store) project(ctx context.Context, selector string) (control.Project, error) {
	var project control.Project
	var created int64
	err := s.db.QueryRowContext(ctx, `SELECT id,slug,name,created_at FROM projects WHERE id=? OR slug=?`, selector, selector).
		Scan(&project.ID, &project.Slug, &project.Name, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return control.Project{}, control.ErrNotFound
	}
	project.CreatedAt = fromUnix(created)
	return project, err
}

func scanTrust(row interface{ Scan(...any) error }) (control.GitHubTrust, error) {
	var trust control.GitHubTrust
	var events string
	var created int64
	var revoked sql.NullInt64
	if err := row.Scan(&trust.ID, &trust.ProjectID, &trust.RepositoryOwnerID, &trust.RepositoryID, &trust.WorkflowRef, &trust.Ref, &trust.Environment, &events, &created, &revoked); err != nil {
		return control.GitHubTrust{}, err
	}
	if err := json.Unmarshal([]byte(events), &trust.Events); err != nil {
		return control.GitHubTrust{}, err
	}
	trust.CreatedAt, trust.RevokedAt = fromUnix(created), nullableTime(revoked)
	return trust, nil
}

func scanJob(row interface{ Scan(...any) error }) (control.Job, error) {
	var job control.Job
	var command, environment, status string
	var timeoutMillis, created int64
	var started, finished, exitCode sql.NullInt64
	var cancelled int
	if err := row.Scan(&job.ID, &job.ProjectID, &job.Image, &command, &job.WorkingDirectory, &environment,
		&job.RootDigest, &status, &timeoutMillis, &job.CPUs, &job.Memory, &created, &started, &finished,
		&exitCode, &job.ErrorMessage, &cancelled, &job.WorkerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return control.Job{}, control.ErrNotFound
		}
		return control.Job{}, err
	}
	if err := json.Unmarshal([]byte(command), &job.Command); err != nil {
		return control.Job{}, err
	}
	if err := json.Unmarshal([]byte(environment), &job.Environment); err != nil {
		return control.Job{}, err
	}
	job.Status, job.Timeout, job.CreatedAt = protocol.Status(status), time.Duration(timeoutMillis)*time.Millisecond, fromUnix(created)
	job.StartedAt, job.FinishedAt, job.CancelRequested = nullableTime(started), nullableTime(finished), cancelled != 0
	if exitCode.Valid {
		value := int(exitCode.Int64)
		job.ExitCode = &value
	}
	return job, nil
}

func scanIdempotentJob(row interface{ Scan(...any) error }) (control.Job, string, error) {
	var job control.Job
	var command, environment, status, requestHash string
	var timeoutMillis, created int64
	var started, finished, exitCode sql.NullInt64
	var cancelled int
	if err := row.Scan(&job.ID, &job.ProjectID, &job.Image, &command, &job.WorkingDirectory, &environment,
		&job.RootDigest, &status, &timeoutMillis, &job.CPUs, &job.Memory, &created, &started, &finished,
		&exitCode, &job.ErrorMessage, &cancelled, &job.WorkerID, &requestHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return control.Job{}, "", control.ErrNotFound
		}
		return control.Job{}, "", err
	}
	if err := json.Unmarshal([]byte(command), &job.Command); err != nil {
		return control.Job{}, "", err
	}
	if err := json.Unmarshal([]byte(environment), &job.Environment); err != nil {
		return control.Job{}, "", err
	}
	job.Status, job.Timeout, job.CreatedAt = protocol.Status(status), time.Duration(timeoutMillis)*time.Millisecond, fromUnix(created)
	job.StartedAt, job.FinishedAt, job.CancelRequested = nullableTime(started), nullableTime(finished), cancelled != 0
	if exitCode.Valid {
		value := int(exitCode.Int64)
		job.ExitCode = &value
	}
	return job, requestHash, nil
}

type jobCursor struct {
	ProjectID string `json:"project_id"`
	CreatedAt int64  `json:"created_at"`
	ID        string `json:"id"`
}

func (s *Store) encodeJobCursor(projectID string, job control.Job) (string, error) {
	payload, err := json.Marshal(jobCursor{ProjectID: projectID, CreatedAt: unix(job.CreatedAt), ID: job.ID})
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.pepper)
	_, _ = mac.Write(payload)
	value := append(payload, mac.Sum(nil)...)
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (s *Store) decodeJobCursor(projectID, token string) (jobCursor, error) {
	value, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(value) <= sha256.Size {
		return jobCursor{}, control.ErrInvalidPageToken
	}
	payload, signature := value[:len(value)-sha256.Size], value[len(value)-sha256.Size:]
	mac := hmac.New(sha256.New, s.pepper)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return jobCursor{}, control.ErrInvalidPageToken
	}
	var cursor jobCursor
	if err := json.Unmarshal(payload, &cursor); err != nil || cursor.ProjectID != projectID || cursor.CreatedAt <= 0 || cursor.ID == "" {
		return jobCursor{}, control.ErrInvalidPageToken
	}
	return cursor, nil
}

func (s *Store) newToken(kind string) (string, string, string, error) {
	id, err := randomID("tok")
	if err != nil {
		return "", "", "", err
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", "", err
	}
	secret := "rtest_" + kind + "_" + id + "_" + base64.RawURLEncoding.EncodeToString(secretBytes)
	return id, secret, s.digest(secret), nil
}

func (s *Store) digest(token string) string {
	mac := hmac.New(sha256.New, s.pepper)
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func parseToken(token string) (control.PrincipalKind, string, bool) {
	parts := strings.SplitN(token, "_", 4)
	if len(parts) != 4 || parts[0] != "rtest" || parts[2] == "" || parts[3] == "" {
		return "", "", false
	}
	switch parts[1] {
	case "dt":
		return control.PrincipalDevice, parts[2], true
	case "gh":
		return control.PrincipalGitHub, parts[2], true
	default:
		return "", "", false
	}
}

func randomID(prefix string) (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(value), nil
}

func glob(pattern, value string) bool {
	matched, err := path.Match(pattern, value)
	return err == nil && matched
}

func exactOptional(pattern, value string) bool { return pattern == "" || pattern == value }

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func unix(value time.Time) int64     { return value.UTC().UnixNano() }
func fromUnix(value int64) time.Time { return time.Unix(0, value).UTC() }
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func nullableTime(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	result := fromUnix(value.Int64)
	return &result
}

func timeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return unix(*value)
}

func intValue(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func cloneMap(input map[string]string) map[string]string {
	if input == nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
