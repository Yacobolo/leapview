-- +goose Up

CREATE TABLE project_candidates (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  target_id TEXT NOT NULL,
  environment TEXT NOT NULL,
  owner_principal_id TEXT NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
  base_generation TEXT NOT NULL,
  artifact_digest TEXT NOT NULL,
  status TEXT NOT NULL
    CHECK (status IN ('preparing', 'ready', 'failed', 'cancelled', 'expired')),
  failure_reason TEXT NOT NULL DEFAULT '',
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  ready_at TEXT,
  cancelled_at TEXT,
  expired_at TEXT,
  revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0)
);

CREATE UNIQUE INDEX project_candidates_active_session_idx
  ON project_candidates(target_id, project_id, owner_principal_id)
  WHERE status IN ('preparing', 'ready', 'failed');

CREATE INDEX project_candidates_owner_status_idx
  ON project_candidates(owner_principal_id, status, updated_at DESC);

CREATE INDEX project_candidates_expiry_idx
  ON project_candidates(target_id, expires_at)
  WHERE status IN ('preparing', 'ready', 'failed');

-- +goose Down

DROP INDEX project_candidates_expiry_idx;
DROP INDEX project_candidates_owner_status_idx;
DROP INDEX project_candidates_active_session_idx;
DROP TABLE project_candidates;
