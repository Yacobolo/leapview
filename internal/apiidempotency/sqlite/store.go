// Package sqlite persists public API idempotency records and execution leases.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Record struct {
	Digest       string
	Owner        string
	LeaseExpires time.Time
	Status       int
	Header       http.Header
	Body         []byte
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Claim(ctx context.Context, scope, digest, owner string, lease, lifetime time.Duration) (Record, bool, error) {
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM api_idempotency_records WHERE scope = ? AND expires_at <= ?`, scope, now.Format(time.RFC3339Nano)); err != nil {
		return Record{}, false, err
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO api_idempotency_records(scope, request_digest, state, owner_id, lease_expires_at, created_at, updated_at, expires_at)
		VALUES (?, ?, 'pending', ?, ?, ?, ?, ?)
		ON CONFLICT(scope) DO NOTHING`, scope, digest, owner, now.Add(lease).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Add(lifetime).Format(time.RFC3339Nano))
	if err != nil {
		return Record{}, false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return Record{}, false, err
	}
	execute := rows == 1
	if !execute {
		result, err = s.db.ExecContext(ctx, `UPDATE api_idempotency_records
			SET owner_id = ?, lease_expires_at = ?, updated_at = ?
			WHERE scope = ? AND request_digest = ? AND state = 'pending' AND lease_expires_at <= ?`,
			owner, now.Add(lease).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), scope, digest, now.Format(time.RFC3339Nano))
		if err != nil {
			return Record{}, false, err
		}
		rows, err = result.RowsAffected()
		if err != nil {
			return Record{}, false, err
		}
		execute = rows == 1
	}
	record, err := s.Load(ctx, scope)
	return record, execute, err
}

func (s *Store) Load(ctx context.Context, scope string) (Record, error) {
	var digest, state, owner, leaseExpires string
	var status sql.NullInt64
	var headersJSON sql.NullString
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT request_digest, state, owner_id, lease_expires_at,
		response_status, response_headers_json, response_body FROM api_idempotency_records WHERE scope = ?`, scope).
		Scan(&digest, &state, &owner, &leaseExpires, &status, &headersJSON, &body)
	if err != nil {
		return Record{}, err
	}
	parsedLease, _ := time.Parse(time.RFC3339Nano, leaseExpires)
	record := Record{Digest: digest, Owner: owner, LeaseExpires: parsedLease}
	if state != "completed" {
		return record, nil
	}
	record.Status = int(status.Int64)
	record.Body = append([]byte(nil), body...)
	record.Header = http.Header{}
	if headersJSON.Valid && headersJSON.String != "" {
		if err := json.Unmarshal([]byte(headersJSON.String), &record.Header); err != nil {
			return Record{}, err
		}
	}
	return record, nil
}

func (s *Store) Renew(ctx context.Context, scope, digest, owner string, lease time.Duration) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE api_idempotency_records SET lease_expires_at = ?, updated_at = ?
		WHERE scope = ? AND request_digest = ? AND owner_id = ? AND state = 'pending'`,
		now.Add(lease).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), scope, digest, owner)
	return requireOne(result, err, "renew")
}

func (s *Store) Complete(ctx context.Context, scope, digest, owner string, status int, header http.Header, body []byte) error {
	headersJSON, err := json.Marshal(header)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE api_idempotency_records
		SET state = 'completed', response_status = ?, response_headers_json = ?, response_body = ?, updated_at = ?
		WHERE scope = ? AND request_digest = ? AND owner_id = ? AND state = 'pending'`,
		status, string(headersJSON), body, time.Now().UTC().Format(time.RFC3339Nano), scope, digest, owner)
	return requireOne(result, err, "complete")
}

func (s *Store) Abandon(ctx context.Context, scope, digest, owner string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM api_idempotency_records
		WHERE scope = ? AND request_digest = ? AND owner_id = ? AND state = 'pending'`, scope, digest, owner)
	return requireOne(result, err, "abandon")
}

func requireOne(result sql.Result, err error, operation string) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("idempotency %s changed %d records", operation, rows)
	}
	return nil
}
