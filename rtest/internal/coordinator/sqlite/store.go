package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/flidai/leapview/rtest/internal/protocol"
	_ "modernc.org/sqlite"
)

type Store struct {
	db      *sql.DB
	logsDir string
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	logsDir := filepath.Join(filepath.Dir(path), "logs")
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		return nil, fmt.Errorf("create logs directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db, logsDir: logsDir}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  repository TEXT NOT NULL,
  suite TEXT NOT NULL DEFAULT 'command',
  runner TEXT NOT NULL,
  command_json TEXT NOT NULL,
  source_digest TEXT NOT NULL,
  status TEXT NOT NULL,
  worker_id TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  started_at INTEGER,
  finished_at INTEGER,
  lease_expires_at INTEGER,
  timeout_seconds INTEGER NOT NULL,
  cancel_requested INTEGER NOT NULL DEFAULT 0,
  exit_code INTEGER,
  error_message TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS jobs_queue_idx ON jobs(status, created_at);
`)
	if err != nil {
		return err
	}
	columns, err := s.tableColumns(ctx, "jobs")
	if err != nil {
		return err
	}
	if !columns["suite"] {
		_, err = s.db.ExecContext(ctx, `ALTER TABLE jobs ADD COLUMN suite TEXT NOT NULL DEFAULT 'command'`)
	}
	return err
}

func (s *Store) tableColumns(ctx context.Context, table string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func (s *Store) CreateJob(ctx context.Context, input protocol.CreateJob) (protocol.Job, error) {
	command, err := json.Marshal(input.Command)
	if err != nil {
		return protocol.Job{}, err
	}
	now := time.Now().UTC()
	if input.Suite == "" {
		input.Suite = "command"
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO jobs(id, repository, suite, runner, command_json, source_digest, status, created_at, timeout_seconds)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`, input.ID, input.Repository, input.Suite, input.Runner, string(command), input.SourceDigest,
		protocol.StatusQueued, now.UnixNano(), max(1, int(input.Timeout.Seconds())))
	if err != nil {
		return protocol.Job{}, fmt.Errorf("insert job: %w", err)
	}
	if err := os.WriteFile(s.logPath(input.ID), nil, 0o600); err != nil {
		return protocol.Job{}, fmt.Errorf("create job log: %w", err)
	}
	return s.Job(ctx, input.ID)
}

func (s *Store) Job(ctx context.Context, id string) (protocol.Job, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, repository, suite, runner, command_json, source_digest, status,
worker_id, created_at, started_at, finished_at, lease_expires_at, timeout_seconds,
cancel_requested, exit_code, error_message FROM jobs WHERE id = ?`, id)
	return scanJob(row)
}

func (s *Store) ListJobs(ctx context.Context, repository string, limit int) ([]protocol.Job, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("job list limit must be between 1 and 100")
	}
	query := `SELECT id, repository, suite, runner, command_json, source_digest, status,
worker_id, created_at, started_at, finished_at, lease_expires_at, timeout_seconds,
cancel_requested, exit_code, error_message FROM jobs`
	args := make([]any, 0, 2)
	if repository != "" {
		query += ` WHERE repository = ?`
		args = append(args, repository)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]protocol.Job, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) MarkExpiredLeases(ctx context.Context, now time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE jobs
SET status = ?, finished_at = ?, lease_expires_at = NULL,
    error_message = CASE WHEN error_message = '' THEN 'worker lease expired' ELSE error_message END
WHERE status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at < ?`,
		protocol.StatusLost, now.UTC().UnixNano(), protocol.StatusRunning, now.UTC().UnixNano())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) Claim(ctx context.Context, workerID string, leaseExpires time.Time) (protocol.Job, bool, error) {
	if _, err := s.MarkExpiredLeases(ctx, time.Now()); err != nil {
		return protocol.Job{}, false, err
	}
	now := time.Now().UTC()
	row := s.db.QueryRowContext(ctx, `
UPDATE jobs
SET status = ?, worker_id = ?, started_at = ?, lease_expires_at = ?
WHERE id = (
  SELECT id FROM jobs WHERE status = ? AND cancel_requested = 0 ORDER BY created_at, id LIMIT 1
)
RETURNING id, repository, suite, runner, command_json, source_digest, status, worker_id, created_at,
started_at, finished_at, lease_expires_at, timeout_seconds, cancel_requested, exit_code, error_message`,
		protocol.StatusRunning, workerID, now.UnixNano(), leaseExpires.UTC().UnixNano(), protocol.StatusQueued)
	job, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return protocol.Job{}, false, nil
	}
	return job, err == nil, err
}

func (s *Store) RenewLease(ctx context.Context, id, workerID string, leaseExpires time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE jobs SET lease_expires_at = ? WHERE id = ? AND worker_id = ? AND status = ?`,
		leaseExpires.UTC().UnixNano(), id, workerID, protocol.StatusRunning)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("job %s is not leased by worker %s", id, workerID)
	}
	return nil
}

func (s *Store) RequestCancel(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE jobs SET cancel_requested = 1,
status = CASE WHEN status = ? THEN ? ELSE status END,
finished_at = CASE WHEN status = ? THEN ? ELSE finished_at END
WHERE id = ? AND status IN (?, ?)`,
		protocol.StatusQueued, protocol.StatusCancelled, protocol.StatusQueued, time.Now().UTC().UnixNano(),
		id, protocol.StatusQueued, protocol.StatusRunning)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("job %s is not cancellable", id)
	}
	return nil
}

func (s *Store) Finish(ctx context.Context, id string, status protocol.Status, exitCode *int, message string) error {
	if !status.Terminal() {
		return fmt.Errorf("status %q is not terminal", status)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE jobs SET status = ?, finished_at = ?, lease_expires_at = NULL,
exit_code = ?, error_message = ? WHERE id = ? AND status IN (?, ?)`, status, time.Now().UTC().UnixNano(),
		exitCode, message, id, protocol.StatusQueued, protocol.StatusRunning)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("job %s cannot transition to %s", id, status)
	}
	return nil
}

func (s *Store) AppendLog(_ context.Context, id string, data []byte) error {
	file, err := os.OpenFile(s.logPath(id), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func (s *Store) ReadLog(_ context.Context, id string, offset int64, limit int) ([]byte, int64, error) {
	if offset < 0 || limit <= 0 {
		return nil, offset, errors.New("invalid log range")
	}
	file, err := os.Open(s.logPath(id))
	if err != nil {
		return nil, offset, err
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)))
	return data, offset + int64(len(data)), err
}

func (s *Store) logPath(id string) string { return filepath.Join(s.logsDir, id+".log") }

type scanner interface{ Scan(...any) error }

func scanJob(row scanner) (protocol.Job, error) {
	var job protocol.Job
	var command string
	var status string
	var created int64
	var started, finished, lease sql.NullInt64
	var cancelled int
	var exitCode sql.NullInt64
	if err := row.Scan(&job.ID, &job.Repository, &job.Suite, &job.Runner, &command, &job.SourceDigest, &status,
		&job.WorkerID, &created, &started, &finished, &lease, &job.TimeoutSeconds, &cancelled,
		&exitCode, &job.ErrorMessage); err != nil {
		return protocol.Job{}, err
	}
	if err := json.Unmarshal([]byte(command), &job.Command); err != nil {
		return protocol.Job{}, fmt.Errorf("decode job command: %w", err)
	}
	job.Status = protocol.Status(status)
	job.CreatedAt = time.Unix(0, created).UTC()
	job.StartedAt = nullableTime(started)
	job.FinishedAt = nullableTime(finished)
	job.LeaseExpiresAt = nullableTime(lease)
	job.CancelRequested = cancelled != 0
	if exitCode.Valid {
		value := int(exitCode.Int64)
		job.ExitCode = &value
	}
	return job, nil
}

func nullableTime(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	result := time.Unix(0, value.Int64).UTC()
	return &result
}
