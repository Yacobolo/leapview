-- Cursor-signing key ring.

-- name: CreateInitialAPICursorSigningKey :exec
INSERT OR IGNORE INTO api_cursor_signing_keys(key_id, secret, active, created_at)
SELECT 'v1', ?, 1, ? WHERE NOT EXISTS (SELECT 1 FROM api_cursor_signing_keys WHERE active = 1);

-- name: ListAPICursorSigningKeys :many
SELECT key_id, secret, active FROM api_cursor_signing_keys
WHERE retired_at IS NULL ORDER BY created_at, key_id;
