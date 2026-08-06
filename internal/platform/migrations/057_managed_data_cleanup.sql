-- +goose Up

ALTER TABLE managed_data_upload_sessions
  ADD COLUMN cleanup_completed_at TEXT;

CREATE INDEX managed_data_upload_sessions_cleanup_idx
  ON managed_data_upload_sessions(status, cleanup_completed_at, updated_at, id);

-- +goose Down

DROP INDEX managed_data_upload_sessions_cleanup_idx;
ALTER TABLE managed_data_upload_sessions DROP COLUMN cleanup_completed_at;
