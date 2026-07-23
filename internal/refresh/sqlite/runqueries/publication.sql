-- Fenced refresh publication is a refresh-owned cross-table unit of work.

-- name: RefreshPublicationCandidate :one
SELECT id, workspace_id, environment, status, ducklake_snapshot_id
FROM serving_states
WHERE id = sqlc.arg(serving_state_id);

-- name: RefreshPublicationFenceActive :one
SELECT EXISTS(
  SELECT 1
  FROM refresh_job_runs candidate
  JOIN refresh_jobs candidate_job ON candidate_job.id = candidate.job_id
  WHERE candidate.id = sqlc.arg(run_id)
    AND candidate.status = 'prepared'
    AND candidate.target_generation = sqlc.arg(target_generation)
    AND candidate_job.status = 'running'
    AND candidate_job.lease_owner = sqlc.arg(lease_owner)
    AND candidate_job.lease_generation = sqlc.arg(lease_generation)
    AND NOT EXISTS (
      SELECT 1
      FROM refresh_job_runs newer
      JOIN refresh_jobs newer_job ON newer_job.id = newer.job_id
      WHERE newer_job.workspace_id = candidate_job.workspace_id
        AND newer.environment = candidate.environment
        AND newer.target_type = candidate.target_type
        AND newer.target_id = candidate.target_id
        AND newer.target_generation > candidate.target_generation
    )
) AS active;

-- name: DrainOtherRefreshServingStates :exec
UPDATE serving_states
SET status = 'draining', superseded_at = CURRENT_TIMESTAMP, error = ''
WHERE workspace_id = sqlc.arg(workspace_id)
  AND environment = sqlc.arg(environment)
  AND id <> sqlc.arg(serving_state_id)
  AND status = 'active';

-- name: ActivateRefreshServingState :exec
UPDATE serving_states
SET status = 'active', activated_at = CURRENT_TIMESTAMP, error = ''
WHERE id = sqlc.arg(serving_state_id);

-- name: SetRefreshActiveServingState :exec
INSERT INTO workspace_active_serving_states (workspace_id, environment, serving_state_id, updated_at)
VALUES (sqlc.arg(workspace_id), sqlc.arg(environment), sqlc.arg(serving_state_id), CURRENT_TIMESTAMP)
ON CONFLICT(workspace_id, environment) DO UPDATE SET
  serving_state_id = excluded.serving_state_id,
  updated_at = CURRENT_TIMESTAMP;

-- name: AdvanceRefreshSemanticModelDataVersions :exec
UPDATE semantic_model_data_versions
SET snapshot_id = sqlc.arg(snapshot_id), serving_state_id = sqlc.arg(serving_state_id)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND environment = sqlc.arg(environment)
  AND semantic_model_id <> sqlc.arg(semantic_model_id);

-- name: UpsertRefreshPublicationDataVersion :exec
INSERT INTO semantic_model_data_versions (
  workspace_id, environment, semantic_model_id, snapshot_id, serving_state_id, refreshed_at, source, pipeline_id, run_id
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(environment), sqlc.arg(semantic_model_id), sqlc.arg(snapshot_id),
  sqlc.arg(serving_state_id), sqlc.arg(refreshed_at), 'refresh', NULLIF(sqlc.arg(pipeline_id), ''), NULLIF(sqlc.arg(run_id), '')
)
ON CONFLICT (workspace_id, environment, semantic_model_id) DO UPDATE SET
  snapshot_id = excluded.snapshot_id,
  serving_state_id = excluded.serving_state_id,
  refreshed_at = excluded.refreshed_at,
  source = excluded.source,
  pipeline_id = excluded.pipeline_id,
  run_id = excluded.run_id;

-- name: CompleteRefreshPublicationRun :execrows
UPDATE refresh_job_runs
SET status = 'succeeded', finished_at = CURRENT_TIMESTAMP, error = ''
WHERE refresh_job_runs.id = sqlc.arg(run_id) AND refresh_job_runs.status = 'prepared'
  AND refresh_job_runs.target_generation = sqlc.arg(target_generation)
  AND refresh_job_runs.job_id IN (
    SELECT refresh_jobs.id FROM refresh_jobs
    WHERE refresh_jobs.status = 'running' AND refresh_jobs.lease_owner = sqlc.arg(lease_owner)
      AND refresh_jobs.lease_generation = sqlc.arg(lease_generation)
  );

-- name: CompleteRefreshPublicationJob :execrows
UPDATE refresh_jobs
SET status = 'succeeded', updated_at = CURRENT_TIMESTAMP, finished_at = CURRENT_TIMESTAMP,
    lease_owner = '', lease_expires_at = NULL, last_error = ''
WHERE refresh_jobs.id = (SELECT refresh_job_runs.job_id FROM refresh_job_runs WHERE refresh_job_runs.id = sqlc.arg(run_id))
  AND refresh_jobs.status = 'running' AND refresh_jobs.lease_owner = sqlc.arg(lease_owner)
  AND refresh_jobs.lease_generation = sqlc.arg(lease_generation);
