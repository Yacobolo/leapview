-- name: CreateDashboardViewSession :exec
INSERT INTO dashboard_view_sessions
  (id, key_json, version, state_json, expires_at)
VALUES
  (sqlc.arg(id), sqlc.arg(key_json), 1, sqlc.arg(state_json), sqlc.arg(expires_at));

-- name: GetActiveDashboardViewSession :one
SELECT key_json, version, state_json, expires_at
FROM dashboard_view_sessions
WHERE id = sqlc.arg(id)
  AND expires_at > sqlc.arg(now);

-- name: CompareAndSwapDashboardViewSession :execresult
UPDATE dashboard_view_sessions
SET version = version + 1,
    state_json = sqlc.arg(state_json),
    expires_at = sqlc.arg(expires_at),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(version)
  AND expires_at > sqlc.arg(now);

-- name: TouchDashboardViewSession :execresult
UPDATE dashboard_view_sessions
SET expires_at = sqlc.arg(expires_at),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
  AND expires_at > sqlc.arg(now);

-- name: DeleteExpiredDashboardViewSessions :exec
DELETE FROM dashboard_view_sessions
WHERE expires_at <= sqlc.arg(now);
