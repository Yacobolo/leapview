-- Project deployment transaction and activation records.

-- name: CreateProjectDeployment :exec
INSERT INTO project_deployments (id, project_id, environment, request_digest, status, created_by)
VALUES (?, ?, ?, ?, 'pending', ?);

-- name: CreateProjectDeploymentTarget :exec
INSERT INTO project_deployment_targets (
  deployment_id, workspace_id, serving_state_id, prior_serving_state_id, status
)
VALUES (?, ?, ?, ?, 'pending');

-- name: CreateProjectDeploymentConnection :exec
INSERT INTO project_deployment_connections (
  deployment_id, collection_id, revision_id, prior_revision_id, prior_generation
)
VALUES (?, ?, ?, ?, ?);

-- name: GetProjectDeployment :one
SELECT * FROM project_deployments WHERE id = ?;

-- These serving-state statements belong to the deployment activation unit of
-- work. They deliberately live with the consuming workflow so deployment does
-- not import serving-state generated queries while retaining one atomic SQLite
-- transaction.

-- name: GetServingState :one
SELECT * FROM serving_states WHERE id = ?;

-- name: MarkOtherServingStatesDraining :exec
UPDATE serving_states
SET status = 'draining',
    superseded_at = CURRENT_TIMESTAMP,
    error = ''
WHERE workspace_id = ?
  AND environment = ?
  AND id <> ?
  AND status = 'active';

-- name: MarkServingStateActive :exec
UPDATE serving_states
SET status = 'active', activated_at = CURRENT_TIMESTAMP, error = ''
WHERE id = ?;

-- name: SetActiveServingState :exec
INSERT INTO workspace_active_serving_states (workspace_id, environment, serving_state_id, updated_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(workspace_id, environment) DO UPDATE SET
  serving_state_id = excluded.serving_state_id,
  updated_at = CURRENT_TIMESTAMP;

-- name: PersistPublishSemanticModelDataVersions :exec
INSERT INTO semantic_model_data_versions (
  workspace_id, environment, semantic_model_id, snapshot_id, serving_state_id, refreshed_at, source, pipeline_id, run_id
)
SELECT workspace_id, sqlc.arg(environment), substr(asset_key, instr(asset_key, '.') + 1), sqlc.arg(snapshot_id), serving_state_id, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), 'publish', NULL, NULL
FROM assets
WHERE assets.serving_state_id = sqlc.arg(target_serving_state_id) AND asset_type = 'semantic_model'
ON CONFLICT (workspace_id, environment, semantic_model_id) DO UPDATE SET
  snapshot_id = excluded.snapshot_id,
  serving_state_id = excluded.serving_state_id,
  refreshed_at = excluded.refreshed_at,
  source = excluded.source,
  pipeline_id = NULL,
  run_id = NULL;

-- name: DeleteUndeployedSemanticModelDataVersions :exec
DELETE FROM semantic_model_data_versions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND environment = sqlc.arg(environment)
  AND serving_state_id <> sqlc.arg(target_serving_state_id);

-- name: ListProjectDeploymentTargets :many
SELECT * FROM project_deployment_targets
WHERE deployment_id = ?
ORDER BY workspace_id;

-- name: ListProjectDeploymentConnections :many
SELECT * FROM project_deployment_connections
WHERE deployment_id = ?
ORDER BY collection_id;

-- Deployment approval decisions are immutable in scope and optimistic in
-- transition. A revoked decision remains as audit evidence; a later request
-- receives a new identity.

-- name: CreateDeploymentApproval :exec
INSERT INTO deployment_approvals (
  id, project_id, deployment_id, environment, request_digest, release_id,
  status, requested_by, request_credential_class, request_credential_id,
  requested_at, approved_by, approval_credential_class,
  approval_credential_id, approval_credential_expires_at, approved_at, revoked_by, revoked_at,
  expires_at, revision
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetCurrentDeploymentApproval :one
SELECT *
FROM deployment_approvals
WHERE deployment_id = ?
ORDER BY requested_at DESC, id DESC
LIMIT 1;

-- name: UpdateDeploymentApproval :execrows
UPDATE deployment_approvals
SET status = ?,
    approved_by = ?,
    approval_credential_class = ?,
    approval_credential_id = ?,
    approval_credential_expires_at = ?,
    approved_at = ?,
    revoked_by = ?,
    revoked_at = ?,
    expires_at = ?,
    revision = ?
WHERE id = ?
  AND deployment_id = ?
  AND revision = ?;

-- name: GetWorkspaceActiveServingStateID :one
SELECT serving_state_id
FROM workspace_active_serving_states
WHERE workspace_id = ? AND environment = ?;

-- name: GetManagedDataEnvironmentPointer :one
SELECT * FROM managed_data_environment_pointers
WHERE collection_id = ? AND environment = ?;

-- name: UpsertManagedDataEnvironmentPointer :exec
INSERT INTO managed_data_environment_pointers (
  collection_id, environment, revision_id, deployment_id, generation, updated_by
)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(collection_id, environment) DO UPDATE SET
  revision_id = excluded.revision_id,
  deployment_id = excluded.deployment_id,
  generation = excluded.generation,
  updated_by = excluded.updated_by,
  updated_at = CURRENT_TIMESTAMP;

-- name: ActivateProjectDeploymentTarget :execresult
UPDATE project_deployment_targets
SET status = 'active', activated_at = CURRENT_TIMESTAMP, error = ''
WHERE deployment_id = ? AND workspace_id = ? AND status = 'pending';

-- name: ActivateProjectDeploymentConnection :execresult
UPDATE project_deployment_connections
SET activated_generation = ?
WHERE deployment_id = ? AND collection_id = ? AND activated_generation IS NULL;

-- name: ActivateProjectDeployment :execresult
UPDATE project_deployments
SET status = 'active',
    activated_at = CURRENT_TIMESTAMP,
    activation_principal = ?,
    verification_digest = ?,
    verified_at = CURRENT_TIMESTAMP,
    error = ''
WHERE id = ? AND status = 'pending';

-- name: SupersedeOtherProjectDeployments :exec
UPDATE project_deployments
SET status = 'superseded'
WHERE project_id = ? AND environment = ? AND id <> ? AND status = 'active';

-- name: FailProjectDeployment :execresult
UPDATE project_deployments
SET status = 'failed', error = ?
WHERE id = ? AND status = 'pending';

-- name: CancelProjectDeployment :execresult
UPDATE project_deployments
SET status = 'cancelled'
WHERE id = ? AND status = 'pending';

-- name: DeleteManagedDataServingStateBindings :exec
DELETE FROM managed_data_serving_state_bindings
WHERE serving_state_id = ?;

-- name: CreateManagedDataServingStateBinding :exec
INSERT INTO managed_data_serving_state_bindings (
  serving_state_id, collection_id, revision_id, environment
)
VALUES (?, ?, ?, ?)
ON CONFLICT(serving_state_id, collection_id) DO UPDATE SET
  revision_id = excluded.revision_id,
  environment = excluded.environment,
  bound_at = CURRENT_TIMESTAMP;

-- name: ListManagedDataServingStateBindings :many
SELECT * FROM managed_data_serving_state_bindings
WHERE serving_state_id = ?
ORDER BY collection_id;

-- Deployment-owned validation projections over managed-data records.

-- Durable private project candidate sessions.

-- name: CreateProjectCandidate :exec
INSERT INTO project_candidates (
  id, project_id, target_id, environment, owner_principal_id, candidate_key,
  base_generation, artifact_digest, provenance_digest, status, failure_reason,
  expires_at, created_at, updated_at, ready_at, cancelled_at,
  expired_at, revision
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetProjectCandidate :one
SELECT *
FROM project_candidates
WHERE id = ?;

-- name: GetActiveProjectCandidateSession :one
SELECT *
FROM project_candidates
WHERE target_id = ?
  AND project_id = ?
  AND owner_principal_id = ?
  AND candidate_key = ?
  AND status IN ('preparing', 'ready', 'failed')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetActiveProjectCandidateByKey :one
SELECT *
FROM project_candidates
WHERE target_id = ?
  AND project_id = ?
  AND owner_principal_id = ?
  AND candidate_key = ?
  AND status IN ('preparing', 'ready', 'failed')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: CountActiveProjectCandidatesForOwner :one
SELECT count(*)
FROM project_candidates
WHERE owner_principal_id = ?
  AND status IN ('preparing', 'ready', 'failed');

-- name: GetActiveProjectCandidateBaseGeneration :one
SELECT id
FROM project_deployments
WHERE project_id = ?
  AND environment = ?
  AND status = 'active'
ORDER BY activated_at DESC, created_at DESC, id DESC
LIMIT 1;

-- name: UpdateProjectCandidate :execrows
UPDATE project_candidates
SET artifact_digest = ?,
    provenance_digest = ?,
    status = ?,
    failure_reason = ?,
    expires_at = ?,
    updated_at = ?,
    ready_at = ?,
    cancelled_at = ?,
    expired_at = ?,
    revision = ?
WHERE id = ?
  AND revision = ?;

-- name: ExpireProjectCandidates :execrows
UPDATE project_candidates
SET status = 'expired',
    provenance_digest = '',
    failure_reason = '',
    ready_at = NULL,
    expired_at = ?,
    updated_at = ?,
    revision = revision + 1
WHERE target_id = ?
  AND status IN ('preparing', 'ready', 'failed')
  AND expires_at <= ?;

-- name: GetManagedDataCollection :one
SELECT * FROM managed_data_collections WHERE id = ?;

-- name: GetManagedDataRevision :one
SELECT * FROM managed_data_revisions WHERE id = ?;
