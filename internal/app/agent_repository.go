package app

import (
	"context"

	"github.com/Yacobolo/libredash/internal/agentapp"
	"github.com/Yacobolo/libredash/internal/platform"
)

type agentRepository struct {
	store *platform.Store
}

func NewAgentRepository(store *platform.Store) agentRepository {
	return agentRepository{store: store}
}

func (r agentRepository) CreateConversation(ctx context.Context, input agentapp.ConversationInput) (agentapp.Conversation, error) {
	row, err := r.store.CreateAgentConversation(ctx, platform.AgentConversationInput(input))
	if err != nil {
		return agentapp.Conversation{}, err
	}
	out := agentapp.Conversation{
		ID:             row.ID,
		WorkspaceID:    row.WorkspaceID,
		PrincipalID:    row.PrincipalID,
		Title:          row.Title,
		Status:         row.Status,
		MetadataJSON:   row.MetadataJson,
		TranscriptJSON: row.TranscriptJson,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if row.ArchivedAt.Valid {
		out.ArchivedAt = row.ArchivedAt.String
	}
	return out, nil
}

func (r agentRepository) ListConversations(ctx context.Context, workspaceID, principalID string) ([]agentapp.Conversation, error) {
	rows, err := r.store.ListAgentConversations(ctx, workspaceID, principalID)
	if err != nil {
		return nil, err
	}
	out := make([]agentapp.Conversation, 0, len(rows))
	for _, row := range rows {
		conversation := agentapp.Conversation{
			ID:             row.ID,
			WorkspaceID:    row.WorkspaceID,
			PrincipalID:    row.PrincipalID,
			Title:          row.Title,
			Status:         row.Status,
			MetadataJSON:   row.MetadataJson,
			TranscriptJSON: row.TranscriptJson,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
		}
		if row.ArchivedAt.Valid {
			conversation.ArchivedAt = row.ArchivedAt.String
		}
		out = append(out, conversation)
	}
	return out, nil
}

func (r agentRepository) GetConversation(ctx context.Context, workspaceID, principalID, conversationID string) (agentapp.Conversation, error) {
	row, err := r.store.GetAgentConversation(ctx, workspaceID, principalID, conversationID)
	if err != nil {
		return agentapp.Conversation{}, err
	}
	out := agentapp.Conversation{
		ID:             row.ID,
		WorkspaceID:    row.WorkspaceID,
		PrincipalID:    row.PrincipalID,
		Title:          row.Title,
		Status:         row.Status,
		MetadataJSON:   row.MetadataJson,
		TranscriptJSON: row.TranscriptJson,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if row.ArchivedAt.Valid {
		out.ArchivedAt = row.ArchivedAt.String
	}
	return out, nil
}

func (r agentRepository) UpdateDefaultConversationTitle(ctx context.Context, workspaceID, principalID, conversationID, title string) (agentapp.Conversation, error) {
	row, err := r.store.UpdateDefaultAgentConversationTitle(ctx, workspaceID, principalID, conversationID, title)
	if err != nil {
		return agentapp.Conversation{}, err
	}
	return agentapp.Conversation{ID: row.ID, WorkspaceID: row.WorkspaceID, PrincipalID: row.PrincipalID, Title: row.Title, Status: row.Status, MetadataJSON: row.MetadataJson, TranscriptJSON: row.TranscriptJson, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func (r agentRepository) UpdateConversationTranscript(ctx context.Context, workspaceID, principalID, conversationID, transcriptJSON string) (agentapp.Conversation, error) {
	row, err := r.store.UpdateAgentConversationTranscript(ctx, workspaceID, principalID, conversationID, transcriptJSON)
	if err != nil {
		return agentapp.Conversation{}, err
	}
	return agentapp.Conversation{ID: row.ID, WorkspaceID: row.WorkspaceID, PrincipalID: row.PrincipalID, Title: row.Title, Status: row.Status, MetadataJSON: row.MetadataJson, TranscriptJSON: row.TranscriptJson, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func (r agentRepository) AppendMessage(ctx context.Context, input agentapp.MessageInput) (agentapp.Message, error) {
	row, err := r.store.AppendAgentMessage(ctx, platform.AgentMessageInput(input))
	if err != nil {
		return agentapp.Message{}, err
	}
	out := agentapp.Message{ID: row.ID, ConversationID: row.ConversationID, Seq: row.Seq, Role: row.Role, ContentText: row.ContentText, ContentJSON: row.ContentJson, ToolCallID: row.ToolCallID, ToolName: row.ToolName, IsError: row.IsError, CreatedAt: row.CreatedAt}
	if row.RunID.Valid {
		out.RunID = row.RunID.String
	}
	return out, nil
}

func (r agentRepository) ListMessages(ctx context.Context, workspaceID, principalID, conversationID string) ([]agentapp.Message, error) {
	rows, err := r.store.ListAgentMessages(ctx, workspaceID, principalID, conversationID)
	if err != nil {
		return nil, err
	}
	out := make([]agentapp.Message, 0, len(rows))
	for _, row := range rows {
		message := agentapp.Message{ID: row.ID, ConversationID: row.ConversationID, Seq: row.Seq, Role: row.Role, ContentText: row.ContentText, ContentJSON: row.ContentJson, ToolCallID: row.ToolCallID, ToolName: row.ToolName, IsError: row.IsError, CreatedAt: row.CreatedAt}
		if row.RunID.Valid {
			message.RunID = row.RunID.String
		}
		out = append(out, message)
	}
	return out, nil
}

func (r agentRepository) CreateRun(ctx context.Context, input agentapp.RunInput) (agentapp.Run, error) {
	row, err := r.store.CreateAgentRun(ctx, platform.AgentRunInput(input))
	if err != nil {
		return agentapp.Run{}, err
	}
	return agentapp.Run{ID: row.ID, Status: row.Status, Model: row.Model, CreatedAt: row.StartedAt}, nil
}

func (r agentRepository) FinishRun(ctx context.Context, input agentapp.RunFinish) (agentapp.Run, error) {
	row, err := r.store.FinishAgentRun(ctx, platform.AgentRunFinish(input))
	if err != nil {
		return agentapp.Run{}, err
	}
	return agentapp.Run{ID: row.ID, Status: row.Status, Model: row.Model, CreatedAt: row.StartedAt}, nil
}

func (r agentRepository) ListRuns(ctx context.Context, workspaceID, principalID, conversationID string) ([]agentapp.Run, error) {
	rows, err := r.store.ListAgentRuns(ctx, workspaceID, principalID, conversationID)
	if err != nil {
		return nil, err
	}
	out := make([]agentapp.Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, agentapp.Run{ID: row.ID, Status: row.Status, Model: row.Model, CreatedAt: row.StartedAt})
	}
	return out, nil
}

func (r agentRepository) AppendEvent(ctx context.Context, input agentapp.EventInput) (agentapp.Event, error) {
	row, err := r.store.AppendAgentEvent(ctx, platform.AgentEventInput(input))
	if err != nil {
		return agentapp.Event{}, err
	}
	return agentapp.Event{ID: row.ID, RunID: row.RunID, Seq: row.Seq, EventType: row.EventType, Severity: row.Severity, PayloadJSON: row.PayloadJson, CreatedAt: row.CreatedAt}, nil
}

func (r agentRepository) ListEvents(ctx context.Context, workspaceID, principalID, runID string) ([]agentapp.Event, error) {
	rows, err := r.store.ListAgentEvents(ctx, workspaceID, principalID, runID)
	if err != nil {
		return nil, err
	}
	out := make([]agentapp.Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, agentapp.Event{ID: row.ID, RunID: row.RunID, Seq: row.Seq, EventType: row.EventType, Severity: row.Severity, PayloadJSON: row.PayloadJson, CreatedAt: row.CreatedAt})
	}
	return out, nil
}
