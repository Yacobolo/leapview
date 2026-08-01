-- +goose Up

CREATE TABLE target_connection_bindings (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL,
  logical_connection_id TEXT NOT NULL,
  connector_kind TEXT NOT NULL,
  authentication_mode TEXT NOT NULL
    CHECK (authentication_mode IN ('none', 'external_bundle', 'workload_identity')),
  workspace_id TEXT NOT NULL,
  environment TEXT NOT NULL,
  endpoint_json TEXT NOT NULL,
  credential_project_id TEXT NOT NULL DEFAULT '',
  credential_environment TEXT NOT NULL DEFAULT '',
  credential_secret_path TEXT NOT NULL DEFAULT '',
  credential_secret_key TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
  validated_version TEXT NOT NULL DEFAULT '',
  health TEXT NOT NULL CHECK (health IN ('pending', 'healthy', 'degraded', 'disabled')),
  health_reason TEXT NOT NULL DEFAULT '',
  last_validated_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK (revision > 0)
);

CREATE UNIQUE INDEX target_connection_bindings_scope_idx
  ON target_connection_bindings(target_id, workspace_id, environment, logical_connection_id);

CREATE INDEX target_connection_bindings_health_idx
  ON target_connection_bindings(target_id, environment, health, updated_at DESC);

-- +goose Down

DROP INDEX target_connection_bindings_health_idx;
DROP INDEX target_connection_bindings_scope_idx;
DROP TABLE target_connection_bindings;
