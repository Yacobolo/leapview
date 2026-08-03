-- +goose Up

CREATE TABLE deployment_approvals (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  deployment_id TEXT NOT NULL REFERENCES project_deployments(id) ON DELETE CASCADE,
  environment TEXT NOT NULL,
  request_digest TEXT NOT NULL,
  release_id TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'revoked', 'expired')),
  requested_by TEXT NOT NULL REFERENCES principals(id),
  request_credential_class TEXT NOT NULL CHECK (
    request_credential_class IN ('human', 'workload', 'api_token', 'session')
  ),
  request_credential_id TEXT NOT NULL,
  requested_at TEXT NOT NULL,
  approved_by TEXT REFERENCES principals(id),
  approval_credential_class TEXT CHECK (
    approval_credential_class IS NULL OR
    approval_credential_class IN ('human', 'workload', 'api_token', 'session')
  ),
  approval_credential_id TEXT,
  approval_credential_expires_at TEXT,
  approved_at TEXT,
  revoked_by TEXT REFERENCES principals(id),
  revoked_at TEXT,
  expires_at TEXT NOT NULL,
  revision INTEGER NOT NULL CHECK (revision > 0)
);

CREATE UNIQUE INDEX deployment_approvals_live_deployment_idx
  ON deployment_approvals(deployment_id)
  WHERE status IN ('pending', 'approved');

CREATE INDEX deployment_approvals_project_history_idx
  ON deployment_approvals(project_id, requested_at DESC, id DESC);

CREATE INDEX deployment_approvals_expiry_idx
  ON deployment_approvals(status, expires_at);

-- +goose Down

DROP INDEX deployment_approvals_expiry_idx;
DROP INDEX deployment_approvals_project_history_idx;
DROP INDEX deployment_approvals_live_deployment_idx;
DROP TABLE deployment_approvals;
