-- +goose Up
ALTER TABLE api_async_events ADD COLUMN event_key TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX api_async_events_resource_key_idx
  ON api_async_events(resource_kind, resource_id, event_key)
  WHERE event_key <> '';

-- +goose Down
DROP INDEX IF EXISTS api_async_events_resource_key_idx;
