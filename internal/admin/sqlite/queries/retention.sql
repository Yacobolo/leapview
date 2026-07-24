-- Admin-coordinated operational retention spanning agent runs and async events.

-- name: DeleteAsyncEventsForArchivedAgentRuns :exec
DELETE FROM api_async_events
WHERE resource_kind = 'agent_run'
  AND resource_id IN (
    SELECT r.id FROM agent_runs r
    JOIN agent_conversations c ON c.id = r.conversation_id
    WHERE c.archived_at IS NOT NULL
      AND c.archived_at <> ''
      AND c.archived_at < sqlc.arg(cutoff)
  );
