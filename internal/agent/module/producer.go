package module

import (
	"context"
	"errors"

	"github.com/Yacobolo/leapview/internal/agent"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
)

type JobStore interface {
	jobs.Enqueuer
	jobs.EventAppender
	jobs.Canceller
}

func (m *Module) EnqueueRun(ctx context.Context, scope agent.Scope, started *agent.StartedPrompt) error {
	if m == nil || started == nil {
		return errors.New("agent run queue is unavailable")
	}
	if err := jobs.AppendJSONEvent(ctx, m.jobs, "agent_run", started.RunID, "agent_run.queued", map[string]any{
		"runId": started.RunID, "conversationId": started.ConversationID, "status": "running",
	}); err != nil {
		return err
	}
	return jobs.EnqueueJSON(ctx, m.jobs, jobs.JSONEnqueueInput{
		ID: "agent:" + started.RunID + ":run", Kind: RunJobKind,
		WorkloadClass: m.runWorkloadClass, WorkspaceID: runWorkspaceID(scope, m.defaultWorkspaceID, m.globalWorkspaceID),
		ResourceKind: "agent_run", ResourceID: started.RunID,
		Payload: RunJob{
			Scope: scope, Conversation: started.ConversationID,
			Run: started.RunID, CorrelationID: started.CorrelationID,
		},
	})
}

func (m *Module) CancelQueuedRun(ctx context.Context, scope agent.Scope, conversationID, runID string) (bool, error) {
	if m == nil {
		return false, errors.New("agent run queue is unavailable")
	}
	cancelled, err := jobs.CancelQueued(ctx, m.jobs, "agent:"+runID+":run")
	if err != nil || !cancelled {
		return cancelled, err
	}
	if m.service == nil {
		return false, errors.New("agent service is unavailable")
	}
	if err := m.service.CancelPersistedRun(ctx, scope, conversationID, runID); err != nil {
		return false, err
	}
	_ = jobs.AppendJSONEvent(ctx, m.jobs, "agent_run", runID, "agent_run.cancelled", map[string]any{
		"runId": runID, "conversationId": conversationID,
	})
	return true, nil
}

func runWorkspaceID(scope agent.Scope, defaultWorkspaceID, globalWorkspaceID string) string {
	if scope.WorkspaceID != "" {
		return scope.WorkspaceID
	}
	if scope.Credential.WorkspaceID != "" {
		return scope.Credential.WorkspaceID
	}
	if defaultWorkspaceID != "" {
		return defaultWorkspaceID
	}
	return globalWorkspaceID
}
