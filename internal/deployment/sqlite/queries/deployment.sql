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
SET status = 'active', activated_at = CURRENT_TIMESTAMP, error = ''
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

-- name: GetManagedDataCollection :one
SELECT * FROM managed_data_collections WHERE id = ?;

-- name: GetManagedDataRevision :one
SELECT * FROM managed_data_revisions WHERE id = ?;
