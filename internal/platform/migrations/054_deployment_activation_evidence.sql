-- +goose Up

ALTER TABLE project_deployments
  ADD COLUMN activation_principal TEXT;

ALTER TABLE project_deployments
  ADD COLUMN verification_digest TEXT;

ALTER TABLE project_deployments
  ADD COLUMN verified_at TEXT;

-- +goose Down

-- Forward-only: removing activation and verification evidence would make a
-- historical cutover less attributable. Preserve the safer schema when an
-- operator rolls back application binaries.
SELECT 1;
