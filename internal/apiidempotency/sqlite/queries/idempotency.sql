-- Public API idempotency records and execution leases.

-- name: DeleteExpiredAPIIdempotencyRecord :exec
DELETE FROM api_idempotency_records WHERE scope = ? AND expires_at <= ?;

-- name: CreateAPIIdempotencyRecord :execrows
INSERT INTO api_idempotency_records
  (scope, request_digest, state, owner_id, lease_expires_at, created_at, updated_at, expires_at)
VALUES (?, ?, 'pending', ?, ?, ?, ?, ?) ON CONFLICT(scope) DO NOTHING;

-- name: ReclaimAPIIdempotencyRecord :execrows
UPDATE api_idempotency_records SET owner_id = sqlc.arg(owner_id),
  lease_expires_at = sqlc.arg(new_lease_expires_at), updated_at = sqlc.arg(updated_at)
WHERE scope = sqlc.arg(scope) AND request_digest = sqlc.arg(request_digest)
  AND state = 'pending' AND lease_expires_at <= sqlc.arg(now);

-- name: GetAPIIdempotencyRecord :one
SELECT request_digest, state, owner_id, lease_expires_at, response_status,
  response_headers_json, response_body FROM api_idempotency_records WHERE scope = ?;

-- name: RenewAPIIdempotencyRecord :execrows
UPDATE api_idempotency_records SET lease_expires_at = ?, updated_at = ?
WHERE scope = ? AND request_digest = ? AND owner_id = ? AND state = 'pending';

-- name: CompleteAPIIdempotencyRecord :execrows
UPDATE api_idempotency_records SET state = 'completed', response_status = ?, response_headers_json = ?,
  response_body = ?, updated_at = ?
WHERE scope = ? AND request_digest = ? AND owner_id = ? AND state = 'pending';

-- name: AbandonAPIIdempotencyRecord :execrows
DELETE FROM api_idempotency_records
WHERE scope = ? AND request_digest = ? AND owner_id = ? AND state = 'pending';
