package agent

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("agent record not found")

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
	PrincipalID    string
	Title          string
	Status         string
	MetadataJSON   string
	TranscriptJSON string
	CreatedAt      string
	UpdatedAt      string
	ArchivedAt     string
}

type Page struct {
	Limit int
	After string
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
	ID             string
	ConversationID string
	Status         string
	Model          string
	StopReason     string
	InputTokens    int64
	OutputTokens   int64
	TotalTokens    int64
	Error          string
	StartedAt      string
	FinishedAt     string
	MetadataJSON   string
	CreatedAt      string
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
	PrincipalID  string
	Title        string
	MetadataJSON string
}

type ConversationUpdate struct {
	PrincipalID    string
	ConversationID string
	Title          string
}

type MessageInput struct {
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
	PrincipalID    string
	ConversationID string
	RunID          string
	Model          string
	MetadataJSON   string
}

type RunFinish struct {
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
	PrincipalID string
	RunID       string
	Sequence    int64
	EventType   string
	Severity    string
	PayloadJSON string
}

type Repository interface {
	CreateConversation(ctx context.Context, input ConversationInput) (Conversation, error)
	ListConversations(ctx context.Context, principalID string) ([]Conversation, error)
	ListConversationsPage(ctx context.Context, principalID string, page Page) ([]Conversation, error)
	GetConversation(ctx context.Context, principalID, conversationID string) (Conversation, error)
	UpdateConversation(ctx context.Context, input ConversationUpdate) (Conversation, error)
	ArchiveConversation(ctx context.Context, principalID, conversationID string) (Conversation, error)
	UpdateDefaultConversationTitle(ctx context.Context, principalID, conversationID, title string) (Conversation, error)
	UpdateConversationTranscript(ctx context.Context, principalID, conversationID, transcriptJSON string) (Conversation, error)
	AppendMessage(ctx context.Context, input MessageInput) (Message, error)
	ListMessages(ctx context.Context, principalID, conversationID string) ([]Message, error)
	ListMessagesPage(ctx context.Context, principalID, conversationID string, page Page) ([]Message, error)
	CreateRun(ctx context.Context, input RunInput) (Run, error)
	FinishRun(ctx context.Context, input RunFinish) (Run, error)
	ListRuns(ctx context.Context, principalID, conversationID string) ([]Run, error)
	ListRunsPage(ctx context.Context, principalID, conversationID string, page Page) ([]Run, error)
	GetRun(ctx context.Context, principalID, conversationID, runID string) (Run, error)
	GetRunByID(ctx context.Context, principalID, runID string) (Run, error)
	AppendEvent(ctx context.Context, input EventInput) (Event, error)
	ListEvents(ctx context.Context, principalID, runID string) ([]Event, error)
	ListEventsPage(ctx context.Context, principalID, runID string, page Page) ([]Event, error)
}
