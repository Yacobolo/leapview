-- +goose Up

ALTER TABLE project_candidates
ADD COLUMN provenance_digest TEXT NOT NULL DEFAULT '';

CREATE TABLE release_candidate_provenance (
  project_id TEXT NOT NULL,
  candidate_id TEXT NOT NULL,
  candidate_revision INTEGER NOT NULL CHECK (candidate_revision > 0),
  provenance_digest TEXT NOT NULL,
  provenance_json TEXT NOT NULL CHECK (json_valid(provenance_json)),
  retained_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (candidate_id, candidate_revision),
  UNIQUE (provenance_digest)
);

CREATE INDEX release_candidate_provenance_project_idx
  ON release_candidate_provenance(project_id, retained_at DESC);

-- +goose Down

DROP INDEX release_candidate_provenance_project_idx;
DROP TABLE release_candidate_provenance;
ALTER TABLE project_candidates DROP COLUMN provenance_digest;
