-- name: InsertQueryEvent :exec
INSERT INTO query_events (
  id,
  workspace_id,
  principal_id,
  surface,
  operation,
  query_kind,
  model_id,
  target,
  object_type,
  object_id,
  request_id,
  correlation_id,
  status,
  duration_ms,
  queue_wait_ms,
  planning_ms,
  connection_wait_ms,
  database_ms,
  execution_ms,
  execution_state,
  rows_returned,
  bytes_estimate,
  error,
  sql_text,
  plan_text,
  query_json
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(workspace_id),
  sqlc.arg(principal_id),
  sqlc.arg(surface),
  sqlc.arg(operation),
  sqlc.arg(query_kind),
  sqlc.arg(model_id),
  sqlc.arg(target),
  sqlc.arg(object_type),
  sqlc.arg(object_id),
  sqlc.arg(request_id),
  sqlc.arg(correlation_id),
  sqlc.arg(status),
  sqlc.arg(duration_ms),
  sqlc.arg(queue_wait_ms),
  sqlc.arg(planning_ms),
  sqlc.arg(connection_wait_ms),
  sqlc.arg(database_ms),
  sqlc.arg(execution_ms),
  sqlc.arg(execution_state),
  sqlc.arg(rows_returned),
  sqlc.arg(bytes_estimate),
  sqlc.arg(error),
  sqlc.arg(sql_text),
  sqlc.arg(plan_text),
  sqlc.arg(query_json)
);

-- name: GetQueryEvent :one
SELECT *
FROM query_events
WHERE id = sqlc.arg(id);

-- name: ListQueryEvents :many
SELECT *
FROM query_events
WHERE (sqlc.arg(workspace_id) = '' OR workspace_id = sqlc.arg(workspace_id))
  AND (sqlc.arg(principal_id) = '' OR principal_id = sqlc.arg(principal_id))
  AND (sqlc.arg(surface) = '' OR surface = sqlc.arg(surface))
  AND (sqlc.arg(operation) = '' OR operation = sqlc.arg(operation))
  AND (sqlc.arg(query_kind) = '' OR query_kind = sqlc.arg(query_kind))
  AND (sqlc.arg(model_id) = '' OR model_id = sqlc.arg(model_id))
  AND (sqlc.arg(target) = '' OR target = sqlc.arg(target))
  AND (sqlc.arg(status) = '' OR status = sqlc.arg(status))
  AND (sqlc.arg(from_time) = '' OR created_at >= sqlc.arg(from_time))
  AND (sqlc.arg(to_time) = '' OR created_at <= sqlc.arg(to_time))
  AND (
    sqlc.arg(search) = ''
    OR target LIKE '%' || sqlc.arg(search) || '%'
    OR sql_text LIKE '%' || sqlc.arg(search) || '%'
    OR query_json LIKE '%' || sqlc.arg(search) || '%'
  )
  AND (
    sqlc.arg(cursor_time) = ''
    OR created_at < sqlc.arg(cursor_time)
    OR (created_at = sqlc.arg(cursor_time) AND id < sqlc.arg(cursor_id))
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit);

