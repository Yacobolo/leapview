package agentapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Yacobolo/libredash/pkg/agent"
)

type PromptInput struct {
	Scope          Scope
	ConversationID string
	Input          string
	CorrelationID  string
	OnEvent        func(EventEnvelope)
}

type PromptResult struct {
	ConversationID string           `json:"conversationId"`
	RunID          string           `json:"runId"`
	StopReason     agent.StopReason `json:"stopReason"`
	Content        string           `json:"content"`
}

type StartedPrompt struct {
	Scope          Scope
	ConversationID string
	RunID          string
	Input          string
	CorrelationID  string

	service *Service
	initial []agent.Message
	mu      sync.Mutex
	closed  bool
}

func (s *Service) Prompt(ctx context.Context, input PromptInput) (PromptResult, error) {
	started, err := s.StartPrompt(ctx, input)
	if err != nil {
		return PromptResult{}, err
	}
	return started.Complete(ctx, input.OnEvent)
}

func (s *Service) StartPrompt(ctx context.Context, input PromptInput) (*StartedPrompt, error) {
	if !s.Enabled() {
		return nil, ErrDisabled
	}
	if policy, ok := s.policyForScope(input.Scope); ok && !policy.Enabled {
		return nil, ErrPolicyDisabled
	}
	if s.repo == nil {
		return nil, fmt.Errorf("agent store is required")
	}
	if strings.TrimSpace(input.Input) == "" {
		return nil, fmt.Errorf("prompt input is required")
	}
	if err := s.acquire(input.ConversationID); err != nil {
		return nil, err
	}
	release := true
	defer func() {
		if release {
			s.release(input.ConversationID)
		}
	}()

	conversation, err := s.repo.GetConversation(ctx, input.Scope.WorkspaceID, input.Scope.PrincipalID, input.ConversationID)
	if err != nil {
		return nil, err
	}
	initial, err := decodeTranscript(conversation.TranscriptJSON)
	if err != nil {
		return nil, err
	}
	runID := newID("run")
	run, err := s.repo.CreateRun(ctx, RunInput{
		WorkspaceID:    input.Scope.WorkspaceID,
		PrincipalID:    input.Scope.PrincipalID,
		ConversationID: input.ConversationID,
		RunID:          runID,
		Model:          s.config.Model,
		MetadataJSON:   metadataJSON(map[string]any{"base_url": s.config.normalizedBaseURL(), "model": s.config.Model}),
	})
	if err != nil {
		return nil, err
	}
	userMessage := agent.Message{
		ID:      newID("msg"),
		Role:    agent.RoleUser,
		Content: input.Input,
	}
	if err := s.appendMessage(ctx, PromptInput{
		Scope:          input.Scope,
		ConversationID: input.ConversationID,
	}, run.ID, userMessage); err != nil {
		_ = s.finishRun(ctx, input, run.ID, RunStatusFailed, "", agent.Usage{}, err)
		return nil, err
	}
	initial = append(initial, userMessage)
	if err := s.persistTranscript(ctx, input, initial); err != nil {
		_ = s.finishRun(ctx, input, run.ID, RunStatusFailed, "", agent.Usage{}, err)
		return nil, err
	}
	release = false
	return &StartedPrompt{
		Scope:          input.Scope,
		ConversationID: input.ConversationID,
		RunID:          run.ID,
		Input:          input.Input,
		CorrelationID:  input.CorrelationID,
		service:        s,
		initial:        initial,
	}, nil
}

func (s *Service) CompletePrompt(ctx context.Context, started *StartedPrompt, onEvent func(EventEnvelope)) (PromptResult, error) {
	if started == nil {
		return PromptResult{}, fmt.Errorf("started prompt is required")
	}
	return started.Complete(ctx, onEvent)
}

func (p *StartedPrompt) Complete(ctx context.Context, onEvent func(EventEnvelope)) (PromptResult, error) {
	if err := p.claim(); err != nil {
		return PromptResult{}, err
	}
	defer p.release()
	s := p.service
	input := PromptInput{
		Scope:          p.Scope,
		ConversationID: p.ConversationID,
		Input:          p.Input,
		CorrelationID:  p.CorrelationID,
		OnEvent:        onEvent,
	}

	sink := &storeEventSink{repo: s.repo, scope: input.Scope, conversationID: input.ConversationID, runID: p.RunID, onEvent: input.OnEvent}
	def := agent.Definition{
		Name:              "libredash-readonly",
		SystemPrompt:      s.systemPrompt(input.Scope),
		Model:             s.model,
		Tools:             s.toolDefinitions(input.Scope),
		InitialTranscript: p.initial,
		Events:            sink,
		IDGenerator:       fixedRunIDGenerator{runID: p.RunID},
	}
	harness, err := agent.New(def)
	if err != nil {
		_ = s.finishRun(ctx, input, p.RunID, RunStatusFailed, "", sink.usage, err)
		return PromptResult{}, err
	}
	result, promptErr := promptFromPersistedUser(ctx, harness, input)
	transcript := harness.Transcript()
	if err := s.persistNewMessages(ctx, input, p.RunID, p.initial, transcript); err != nil && promptErr == nil {
		promptErr = err
	}
	if err := s.persistTranscript(ctx, input, transcript); err != nil && promptErr == nil {
		promptErr = err
	}
	status := RunStatusCompleted
	if promptErr != nil {
		status = RunStatusFailed
		if errors.Is(promptErr, context.Canceled) {
			status = RunStatusCanceled
		}
	}
	if err := s.finishRun(ctx, input, p.RunID, status, result.StopReason, sink.usage, promptErr); err != nil && promptErr == nil {
		promptErr = err
	}
	if promptErr != nil {
		return PromptResult{}, promptErr
	}
	return PromptResult{
		ConversationID: input.ConversationID,
		RunID:          result.RunID,
		StopReason:     result.StopReason,
		Content:        result.FinalMessage.Content,
	}, nil
}

func (p *StartedPrompt) Abort(ctx context.Context, runErr error) error {
	if p == nil {
		return nil
	}
	if err := p.claim(); err != nil {
		return nil
	}
	defer p.release()
	if runErr == nil {
		runErr = fmt.Errorf("prompt aborted")
	}
	input := PromptInput{
		Scope:          p.Scope,
		ConversationID: p.ConversationID,
		Input:          p.Input,
		CorrelationID:  p.CorrelationID,
	}
	return p.service.finishRun(ctx, input, p.RunID, RunStatusFailed, "", agent.Usage{}, runErr)
}

func (p *StartedPrompt) claim() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("started prompt is already closed")
	}
	p.closed = true
	return nil
}

func (p *StartedPrompt) release() {
	if p.service != nil {
		p.service.release(p.ConversationID)
	}
}

func promptFromPersistedUser(ctx context.Context, harness *agent.Agent, input PromptInput) (agent.RunResult, error) {
	return harness.Prompt(ctx, agent.PromptRequest{Input: input.Input, CorrelationID: input.CorrelationID, InputAlreadyAppended: true})
}

func (s *Service) acquire(conversationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[conversationID]; ok {
		return ErrBusy
	}
	s.running[conversationID] = struct{}{}
	return nil
}

func (s *Service) release(conversationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, conversationID)
}

func (s *Service) persistNewMessages(ctx context.Context, input PromptInput, runID string, initial, transcript []agent.Message) error {
	seen := map[string]struct{}{}
	for _, message := range initial {
		if message.ID != "" {
			seen[message.ID] = struct{}{}
		}
	}
	for _, message := range transcript {
		if message.ID != "" {
			if _, ok := seen[message.ID]; ok {
				continue
			}
			seen[message.ID] = struct{}{}
		}
		if err := s.appendMessage(ctx, input, runID, message); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) appendMessage(ctx context.Context, input PromptInput, runID string, message agent.Message) error {
	if message.Role == agent.RoleSystem {
		return nil
	}
	row, err := s.repo.AppendMessage(ctx, MessageInput{
		WorkspaceID:    input.Scope.WorkspaceID,
		PrincipalID:    input.Scope.PrincipalID,
		ConversationID: input.ConversationID,
		RunID:          runID,
		Role:           platformRole(message.Role),
		ContentText:    message.Content,
		ContentJSON:    messageContentJSON(message),
		ToolCallID:     message.ToolCallID,
		ToolName:       message.ToolName,
		IsError:        message.IsError,
	})
	if err == nil && input.OnEvent != nil {
		input.OnEvent(messageEnvelope(input.ConversationID, row))
	}
	return err
}

func (s *Service) persistTranscript(ctx context.Context, input PromptInput, transcript []agent.Message) error {
	bytes, err := json.Marshal(compactTranscriptForStorage(transcript))
	if err != nil {
		return err
	}
	_, err = s.repo.UpdateConversationTranscript(ctx, input.Scope.WorkspaceID, input.Scope.PrincipalID, input.ConversationID, string(bytes))
	return err
}

func compactTranscriptForStorage(transcript []agent.Message) []agent.Message {
	out := make([]agent.Message, len(transcript))
	for i, message := range transcript {
		message.DisplayContent = nil
		out[i] = message
	}
	return out
}

func (s *Service) finishRun(ctx context.Context, input PromptInput, runID, status string, stop agent.StopReason, usage agent.Usage, runErr error) error {
	errText := ""
	if runErr != nil {
		errText = runErr.Error()
	}
	_, err := s.repo.FinishRun(ctx, RunFinish{
		WorkspaceID:    input.Scope.WorkspaceID,
		PrincipalID:    input.Scope.PrincipalID,
		ConversationID: input.ConversationID,
		RunID:          runID,
		Status:         status,
		StopReason:     string(stop),
		InputTokens:    int64(usage.InputTokens),
		OutputTokens:   int64(usage.OutputTokens),
		TotalTokens:    int64(usage.TotalTokens),
		Error:          errText,
		MetadataJSON:   metadataJSON(map[string]any{"model": s.config.Model}),
	})
	return err
}

type fixedRunIDGenerator struct {
	runID string
}

func (g fixedRunIDGenerator) NewID(prefix string) string {
	if prefix == "run" {
		return g.runID
	}
	return newID(prefix)
}

func decodeTranscript(raw string) ([]agent.Message, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var messages []agent.Message
	if err := json.Unmarshal([]byte(raw), &messages); err != nil {
		return nil, err
	}
	return messages, nil
}
