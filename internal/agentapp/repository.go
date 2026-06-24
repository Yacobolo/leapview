package agentapp

import (
	"context"
)

const (
	ConversationDefaultTitle   = "New conversation"
	ConversationStatusActive   = "active"
	ConversationStatusArchived = "archived"

	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
	RunStatusCanceled  = "canceled"

	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"
	MessageRoleTool      = "tool"
	MessageRoleSummary   = "summary"
)

type Conversation struct {
	ID             string
	WorkspaceID    string
	PrincipalID    string
	Title          string
	Status         string
	MetadataJSON   string
	TranscriptJSON string
	CreatedAt      string
	UpdatedAt      string
	ArchivedAt     string
}

type Message struct {
	ID             string
	ConversationID string
	RunID          string
	Seq            int64
	Role           string
	ContentText    string
	ContentJSON    string
	ToolCallID     string
	ToolName       string
	IsError        bool
	CreatedAt      string
}

type Run struct {
	ID        string
	Status    string
	Model     string
	CreatedAt string
}

type Event struct {
	ID          string
	RunID       string
	Seq         int64
	EventType   string
	Severity    string
	PayloadJSON string
	CreatedAt   string
}

type ConversationInput struct {
	WorkspaceID  string
	PrincipalID  string
	Title        string
	MetadataJSON string
}

type MessageInput struct {
	WorkspaceID    string
	PrincipalID    string
	ConversationID string
	RunID          string
	Role           string
	ContentText    string
	ContentJSON    string
	ToolCallID     string
	ToolName       string
	IsError        bool
}

type RunInput struct {
	WorkspaceID    string
	PrincipalID    string
	ConversationID string
	RunID          string
	Model          string
	MetadataJSON   string
}

type RunFinish struct {
	WorkspaceID    string
	PrincipalID    string
	ConversationID string
	RunID          string
	Status         string
	StopReason     string
	InputTokens    int64
	OutputTokens   int64
	TotalTokens    int64
	Error          string
	MetadataJSON   string
}

type EventInput struct {
	WorkspaceID string
	PrincipalID string
	RunID       string
	Sequence    int64
	EventType   string
	Severity    string
	PayloadJSON string
}

type Repository interface {
	CreateConversation(ctx context.Context, input ConversationInput) (Conversation, error)
	ListConversations(ctx context.Context, workspaceID, principalID string) ([]Conversation, error)
	GetConversation(ctx context.Context, workspaceID, principalID, conversationID string) (Conversation, error)
	UpdateDefaultConversationTitle(ctx context.Context, workspaceID, principalID, conversationID, title string) (Conversation, error)
	UpdateConversationTranscript(ctx context.Context, workspaceID, principalID, conversationID, transcriptJSON string) (Conversation, error)
	AppendMessage(ctx context.Context, input MessageInput) (Message, error)
	ListMessages(ctx context.Context, workspaceID, principalID, conversationID string) ([]Message, error)
	CreateRun(ctx context.Context, input RunInput) (Run, error)
	FinishRun(ctx context.Context, input RunFinish) (Run, error)
	ListRuns(ctx context.Context, workspaceID, principalID, conversationID string) ([]Run, error)
	AppendEvent(ctx context.Context, input EventInput) (Event, error)
	ListEvents(ctx context.Context, workspaceID, principalID, runID string) ([]Event, error)
}
