-- +goose Up

ALTER TABLE serving_states ADD COLUMN dashboard_publications_json TEXT NOT NULL DEFAULT '{}';

CREATE TABLE dashboard_publications (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  public_id TEXT NOT NULL UNIQUE,
  dashboard TEXT NOT NULL DEFAULT '',
  default_page TEXT NOT NULL DEFAULT '',
  configuration_digest TEXT NOT NULL DEFAULT '',
  allowed_origins_json TEXT NOT NULL DEFAULT '[]',
  dependency_asset_ids_json TEXT NOT NULL DEFAULT '[]',
  configured INTEGER NOT NULL DEFAULT 0 CHECK (configured IN (0, 1)),
  active_serving_state_id TEXT REFERENCES serving_states(id) ON DELETE SET NULL,
  suspended_at TEXT,
  suspended_by TEXT NOT NULL DEFAULT '',
  configured_at TEXT,
  disabled_at TEXT,
  rotated_at TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(project_id, workspace_id, name)
);

CREATE INDEX dashboard_publications_workspace_idx
  ON dashboard_publications(workspace_id, configured, name);

CREATE TABLE dashboard_publication_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  publication_id TEXT NOT NULL REFERENCES dashboard_publications(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL CHECK (event_type IN ('configured', 'configuration_changed', 'disabled', 'suspended', 'resumed', 'rotated')),
  actor_id TEXT NOT NULL DEFAULT '',
  serving_state_id TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX dashboard_publication_events_publication_idx
  ON dashboard_publication_events(publication_id, id DESC);

-- +goose Down

DROP INDEX dashboard_publication_events_publication_idx;
DROP TABLE dashboard_publication_events;
DROP INDEX dashboard_publications_workspace_idx;
DROP TABLE dashboard_publications;
ALTER TABLE serving_states DROP COLUMN dashboard_publications_json;
