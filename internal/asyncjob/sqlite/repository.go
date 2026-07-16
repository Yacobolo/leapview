package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Yacobolo/libredash/internal/asyncjob"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Enqueue(ctx context.Context, input asyncjob.EnqueueInput) (asyncjob.Job, error) {
	input.ID, input.Kind = strings.TrimSpace(input.ID), strings.TrimSpace(input.Kind)
	input.ResourceKind, input.ResourceID = strings.TrimSpace(input.ResourceKind), strings.TrimSpace(input.ResourceID)
	if input.ID == "" || input.Kind == "" || input.ResourceKind == "" || input.ResourceID == "" || !json.Valid(input.Payload) {
		return asyncjob.Job{}, fmt.Errorf("invalid async job")
	}
	digest := jobDigest(input)
	_, err := r.db.ExecContext(ctx, `INSERT INTO api_async_jobs
		(id, job_kind, resource_kind, resource_id, payload_json, request_digest, status)
		VALUES (?, ?, ?, ?, ?, ?, 'queued')`, input.ID, input.Kind, input.ResourceKind, input.ResourceID, string(input.Payload), digest)
	if err != nil {
		existing, getErr := r.Get(ctx, input.ID)
		if getErr == nil {
			var storedDigest string
			if scanErr := r.db.QueryRowContext(ctx, `SELECT request_digest FROM api_async_jobs WHERE id = ?`, input.ID).Scan(&storedDigest); scanErr == nil && storedDigest == digest {
				return existing, nil
			}
			return asyncjob.Job{}, asyncjob.ErrConflict
		}
		return asyncjob.Job{}, err
	}
	return r.Get(ctx, input.ID)
}

func jobDigest(input asyncjob.EnqueueInput) string {
	sum := sha256.Sum256([]byte(input.Kind + "\x00" + input.ResourceKind + "\x00" + input.ResourceID + "\x00" + string(input.Payload)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (r *Repository) Get(ctx context.Context, id string) (asyncjob.Job, error) {
	return scanJob(r.db.QueryRowContext(ctx, `SELECT id, job_kind, resource_kind, resource_id, payload_json, status,
		attempt_count, lease_owner, COALESCE(lease_expires_at, ''), created_at, COALESCE(started_at, ''), COALESCE(finished_at, ''), error_json
		FROM api_async_jobs WHERE id = ?`, strings.TrimSpace(id)))
}

func (r *Repository) Claim(ctx context.Context, owner string, lease time.Duration) (asyncjob.Job, bool, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" || lease <= 0 {
		return asyncjob.Job{}, false, fmt.Errorf("worker owner and positive lease are required")
	}
	modifier := fmt.Sprintf("+%d seconds", max(1, int(lease.Seconds())))
	row := r.db.QueryRowContext(ctx, `UPDATE api_async_jobs SET
		status = 'running', started_at = COALESCE(started_at, CURRENT_TIMESTAMP),
		lease_owner = ?, lease_expires_at = datetime('now', ?), attempt_count = attempt_count + 1
		WHERE id = (SELECT id FROM api_async_jobs
			WHERE status = 'queued' OR (status = 'running' AND lease_expires_at < CURRENT_TIMESTAMP)
			ORDER BY created_at, id LIMIT 1)
		RETURNING id, job_kind, resource_kind, resource_id, payload_json, status,
		attempt_count, lease_owner, COALESCE(lease_expires_at, ''), created_at, COALESCE(started_at, ''), COALESCE(finished_at, ''), error_json`, owner, modifier)
	job, err := scanJob(row)
	if errors.Is(err, asyncjob.ErrNotFound) {
		return asyncjob.Job{}, false, nil
	}
	return job, err == nil, err
}

func (r *Repository) Renew(ctx context.Context, id, owner string, lease time.Duration) error {
	modifier := fmt.Sprintf("+%d seconds", max(1, int(lease.Seconds())))
	result, err := r.db.ExecContext(ctx, `UPDATE api_async_jobs SET lease_expires_at = datetime('now', ?)
		WHERE id = ? AND status = 'running' AND lease_owner = ?`, modifier, strings.TrimSpace(id), strings.TrimSpace(owner))
	return requireChanged(result, err)
}

func (r *Repository) Complete(ctx context.Context, id, owner string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE api_async_jobs SET status = 'succeeded', finished_at = CURRENT_TIMESTAMP,
		lease_owner = '', lease_expires_at = NULL, error_json = '{}'
		WHERE id = ? AND status = 'running' AND lease_owner = ?`, strings.TrimSpace(id), strings.TrimSpace(owner))
	return requireChanged(result, err)
}

func (r *Repository) Fail(ctx context.Context, id, owner string, problem []byte) error {
	if !json.Valid(problem) {
		problem = []byte(`{"code":"ASYNC_JOB_FAILED"}`)
	}
	result, err := r.db.ExecContext(ctx, `UPDATE api_async_jobs SET status = 'failed', finished_at = CURRENT_TIMESTAMP,
		lease_owner = '', lease_expires_at = NULL, error_json = ?
		WHERE id = ? AND status = 'running' AND lease_owner = ?`, string(problem), strings.TrimSpace(id), strings.TrimSpace(owner))
	return requireChanged(result, err)
}

func (r *Repository) Cancel(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE api_async_jobs SET status = 'cancelled', finished_at = CURRENT_TIMESTAMP,
		lease_owner = '', lease_expires_at = NULL WHERE id = ? AND status = 'queued'`, strings.TrimSpace(id))
	return requireChanged(result, err)
}

func (r *Repository) CancelClaimed(ctx context.Context, id, owner string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE api_async_jobs SET status = 'cancelled', finished_at = CURRENT_TIMESTAMP,
		lease_owner = '', lease_expires_at = NULL WHERE id = ? AND status = 'running' AND lease_owner = ?`, strings.TrimSpace(id), strings.TrimSpace(owner))
	return requireChanged(result, err)
}

func (r *Repository) AppendEvent(ctx context.Context, resourceKind, resourceID, eventType string, data []byte) (asyncjob.Event, error) {
	resourceKind, resourceID, eventType = strings.TrimSpace(resourceKind), strings.TrimSpace(resourceID), strings.TrimSpace(eventType)
	if resourceKind == "" || resourceID == "" || eventType == "" || !json.Valid(data) {
		return asyncjob.Event{}, fmt.Errorf("invalid async event")
	}
	row := r.db.QueryRowContext(ctx, `INSERT INTO api_async_events (resource_kind, resource_id, event_id, event_type, data_json)
		SELECT ?, ?, COALESCE(MAX(event_id), 0) + 1, ?, ?
		FROM api_async_events WHERE resource_kind = ? AND resource_id = ?
		RETURNING event_id, resource_kind, resource_id, event_type, data_json, created_at`,
		resourceKind, resourceID, eventType, string(data), resourceKind, resourceID)
	var event asyncjob.Event
	var encoded string
	if err := row.Scan(&event.ID, &event.ResourceKind, &event.ResourceID, &event.EventType, &encoded, &event.CreatedAt); err != nil {
		return asyncjob.Event{}, err
	}
	event.Data = []byte(encoded)
	return event, nil
}

func (r *Repository) ListEvents(ctx context.Context, resourceKind, resourceID string, after int64, limit int) ([]asyncjob.Event, error) {
	if limit < 1 || limit > 200 {
		return nil, fmt.Errorf("event limit must be between 1 and 200")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT event_id, resource_kind, resource_id, event_type, data_json, created_at
		FROM api_async_events WHERE resource_kind = ? AND resource_id = ? AND event_id > ? ORDER BY event_id LIMIT ?`, resourceKind, resourceID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []asyncjob.Event
	for rows.Next() {
		var event asyncjob.Event
		var data string
		if err := rows.Scan(&event.ID, &event.ResourceKind, &event.ResourceID, &event.EventType, &data, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.Data = []byte(data)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *Repository) event(ctx context.Context, kind, id string, eventID int64) (asyncjob.Event, error) {
	var event asyncjob.Event
	var data string
	err := r.db.QueryRowContext(ctx, `SELECT event_id, resource_kind, resource_id, event_type, data_json, created_at FROM api_async_events WHERE resource_kind = ? AND resource_id = ? AND event_id = ?`, kind, id, eventID).Scan(&event.ID, &event.ResourceKind, &event.ResourceID, &event.EventType, &data, &event.CreatedAt)
	event.Data = []byte(data)
	return event, err
}

type rowScanner interface{ Scan(...any) error }

func scanJob(row rowScanner) (asyncjob.Job, error) {
	var job asyncjob.Job
	var payload string
	err := row.Scan(&job.ID, &job.Kind, &job.ResourceKind, &job.ResourceID, &payload, &job.Status, &job.Attempts, &job.LeaseOwner, &job.LeaseExpiresAt, &job.CreatedAt, &job.StartedAt, &job.FinishedAt, &job.ErrorJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return asyncjob.Job{}, asyncjob.ErrNotFound
	}
	job.Payload = []byte(payload)
	return job, err
}
func requireChanged(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return asyncjob.ErrConflict
	}
	return nil
}
