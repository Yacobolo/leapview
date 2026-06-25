package agentapp

type ChatTranscriptItem struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Text           string `json:"text,omitempty"`
	Markdown       string `json:"markdown,omitempty"`
	ToolCallID     string `json:"toolCallId,omitempty"`
	Name           string `json:"name,omitempty"`
	Title          string `json:"title,omitempty"`
	Status         string `json:"status,omitempty"`
	Summary        string `json:"summary,omitempty"`
	ResultSummary  string `json:"resultSummary,omitempty"`
	InputJSON      string `json:"inputJson,omitempty"`
	ArgumentsJSON  string `json:"argumentsJson,omitempty"`
	ResultJSON     string `json:"resultJson,omitempty"`
	Error          string `json:"error,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	RunID          string `json:"runId,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

type EventEnvelope struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversationId,omitempty"`
	RunID          string         `json:"runId,omitempty"`
	Seq            int64          `json:"seq"`
	Type           string         `json:"type"`
	Severity       string         `json:"severity,omitempty"`
	CreatedAt      string         `json:"createdAt,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
}
