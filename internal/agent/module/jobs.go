package module

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Yacobolo/leapview/internal/agent"
	agenthttp "github.com/Yacobolo/leapview/internal/agent/http"
	"github.com/Yacobolo/leapview/internal/agent/ui"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
)

const RunJobKind = "agent.run"

type RunJob struct {
	Scope                            agent.Scope
	Conversation, Run, CorrelationID string
	ChatClientID                     string
}

func (m *Module) JobHandlers(events jobs.EventAppender) []jobs.Handler {
	return []jobs.Handler{jobs.HandlerFunc{JobKind: RunJobKind, Run: func(ctx context.Context, job jobs.Job) error {
		var payload RunJob
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return err
		}
		if m.service == nil {
			return fmt.Errorf("agent service is unavailable")
		}
		started, err := m.service.ResumePrompt(ctx, payload.Scope, payload.Conversation, payload.Run, payload.CorrelationID)
		if err != nil {
			return err
		}
		if payload.ChatClientID == "" {
			_, err = started.Complete(ctx, nil)
		} else {
			_, err = m.executeStartedChatTurn(ctx, m.service, payload.Scope, started, agenthttp.ChatTurnExecution{
				EmitInitialRunning: true,
				GenerateTitle:      true,
				ClientID:           payload.ChatClientID,
				Emit: func(signal ui.ChatViewState) error {
					if m.broker != nil {
						m.broker.Publish(ChatStreamID(payload.Scope, payload.ChatClientID), chatSignalPatch(signal))
					}
					return nil
				},
			})
		}
		event := "agent_run.completed"
		if err != nil {
			event = "agent_run.failed"
		}
		data, _ := json.Marshal(map[string]any{"runId": payload.Run, "conversationId": payload.Conversation})
		_, _ = events.AppendEvent(context.WithoutCancel(ctx), "agent_run", payload.Run, event, data)
		return err
	}}}
}
