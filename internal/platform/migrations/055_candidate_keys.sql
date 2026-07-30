-- +goose Up

ALTER TABLE project_candidates
  ADD COLUMN candidate_key TEXT NOT NULL DEFAULT 'default';

DROP INDEX project_candidates_active_session_idx;

CREATE UNIQUE INDEX project_candidates_active_session_idx
  ON project_candidates(target_id, project_id, owner_principal_id, candidate_key)
  WHERE status IN ('preparing', 'ready', 'failed');

-- +goose Down

DROP INDEX project_candidates_active_session_idx;

ALTER TABLE project_candidates
  DROP COLUMN candidate_key;

CREATE UNIQUE INDEX project_candidates_active_session_idx
  ON project_candidates(target_id, project_id, owner_principal_id)
  WHERE status IN ('preparing', 'ready', 'failed');
