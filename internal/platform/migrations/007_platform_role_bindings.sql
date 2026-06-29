-- +goose Up
-- Forward-only migration: platform migrations do not rebuild SQLite tables for rollback.

CREATE TABLE IF NOT EXISTS platform_role_bindings (
  id TEXT PRIMARY KEY,
  role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  principal_id TEXT NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS platform_role_bindings_principal_unique_idx
  ON platform_role_bindings(role_id, principal_id);
