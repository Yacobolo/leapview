package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/Yacobolo/libredash/internal/agentapp"
	"github.com/Yacobolo/libredash/internal/api"
	"github.com/Yacobolo/libredash/internal/platform"
	"github.com/Yacobolo/libredash/internal/ui"
	"github.com/Yacobolo/libredash/pkg/agent"
	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
)

type chatSignals struct {
	Agent api.AgentChatSignal `json:"agent"`
}

func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	scope := s.chatScope(r)
	if s.agent == nil || !s.agent.Enabled() || scope.PrincipalID == "" {
		s.renderChat(w, r, s.chatSignal(r.Context(), scope, "", "", false))
		return
	}
	conversations, err := s.agent.ListConversations(r.Context(), scope)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(conversations) == 0 {
		http.Redirect(w, r, "/chat/new", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/chat/"+conversations[0].ID, http.StatusFound)
}

func (s *Server) chatNew(w http.ResponseWriter, r *http.Request) {
	scope := s.chatScope(r)
	s.renderChat(w, r, s.chatSignal(r.Context(), scope, "", "", false))
}

func (s *Server) chatConversation(w http.ResponseWriter, r *http.Request) {
	scope := s.chatScope(r)
	conversationID := strings.TrimSpace(chi.URLParam(r, "conversation"))
	if s.agent == nil || !s.agent.Enabled() {
		s.renderChat(w, r, s.chatSignal(r.Context(), scope, "", "", false))
		return
	}
	if scope.PrincipalID == "" {
		http.Error(w, "chat requires an authenticated principal", http.StatusUnauthorized)
		return
	}
	transcript, err := s.agent.ConversationTranscript(r.Context(), scope, conversationID)
	if err != nil {
		http.Error(w, err.Error(), statusForNotFound(err))
		return
	}
	s.renderChat(w, r, s.chatSignalWith(scope, conversationID, transcript, "", false))
}

func (s *Server) renderChat(w http.ResponseWriter, r *http.Request, signal api.AgentChatSignal) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := ui.ChatPage(s.metrics.Catalog(), csrfToken(r, s.auth), s.currentRoleLabel(r), signal).Render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) chatTurn(w http.ResponseWriter, r *http.Request) {
	service, scope, ok := s.chatService(w, r)
	if !ok {
		return
	}
	signals := chatSignals{}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	input := strings.TrimSpace(signals.Agent.Composer.Value)
	if input == "" {
		http.Error(w, "input is required", http.StatusBadRequest)
		return
	}
	conversationID := strings.TrimSpace(signals.Agent.ActiveConversationID)
	createdConversation := false
	if conversationID == "" {
		conversation, err := service.CreateConversation(r.Context(), scope, "New conversation")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		conversationID = conversation.ID
		createdConversation = true
	}

	transcript, err := service.ConversationTranscript(r.Context(), scope, conversationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	transcript = appendServerUserTranscript(transcript, conversationID, input)
	sse := datastar.NewSSE(w, r)
	if createdConversation {
		_ = sse.ReplaceURL(url.URL{Path: "/chat/" + conversationID})
	}

	streamActiveID := strings.TrimSpace(signals.Agent.ActiveConversationID)
	emit := func(event api.AgentEventEnvelope) {
		transcript = applyLiveTranscriptEvent(transcript, conversationID, event)
		_ = sse.MarshalAndPatchSignals(map[string]any{"agent": chatSignalFromClient(signals.Agent, streamActiveID, transcript, "", true, true)})
	}
	result, err := service.Prompt(r.Context(), agentapp.PromptInput{
		Scope:          scope,
		ConversationID: conversationID,
		Input:          input,
		OnEvent:        emit,
	})
	statusErr := ""
	if err != nil {
		statusErr = err.Error()
		if agentapp.IsBusy(err) {
			statusErr = "A turn is already running for this conversation."
		}
	}
	if result.RunID != "" {
		if refreshed, refreshErr := service.ConversationTranscript(r.Context(), scope, conversationID); refreshErr == nil {
			transcript = refreshed
		}
	}
	_ = sse.MarshalAndPatchSignals(map[string]any{"agent": s.chatSignalWith(scope, conversationID, transcript, statusErr, false)})
}

func chatSignalFromClient(base api.AgentChatSignal, activeID string, transcript []api.AgentChatTranscriptItem, statusErr string, running, enabled bool) api.AgentChatSignal {
	if !enabled && statusErr == "" {
		statusErr = "Agent is not configured"
	}
	conversations := base.Conversations
	if conversations == nil {
		conversations = []api.AgentConversationResponse{}
	}
	return api.AgentChatSignal{
		Conversations:        conversations,
		ActiveConversationID: activeID,
		Transcript:           transcript,
		Status: api.AgentChatStatus{
			Enabled: enabled,
			Running: running,
			Error:   statusErr,
		},
		Composer: api.AgentComposerSignal{
			Value:       "",
			Disabled:    !enabled || running,
			Placeholder: chatPlaceholder(enabled, running),
		},
	}
}

func (s *Server) chatService(w http.ResponseWriter, r *http.Request) (*agentapp.Service, agentapp.Scope, bool) {
	if s.agent == nil || !s.agent.Enabled() {
		http.Error(w, agentapp.ErrDisabled.Error(), http.StatusServiceUnavailable)
		return nil, agentapp.Scope{}, false
	}
	scope := s.chatScope(r)
	if scope.PrincipalID == "" {
		http.Error(w, "chat requires an authenticated principal", http.StatusUnauthorized)
		return nil, agentapp.Scope{}, false
	}
	return s.agent, scope, true
}

func (s *Server) chatScope(r *http.Request) agentapp.Scope {
	principalID := ""
	if s.auth != nil {
		if principal, ok := s.auth.Principal(r); ok {
			principalID = principal.ID
			if principal.DevBypass && s.store != nil {
				_, _ = s.store.UpsertPrincipal(r.Context(), platform.PrincipalInput{ID: principal.ID, Email: principal.Email, DisplayName: principal.DisplayName})
			}
		}
	}
	return agentapp.Scope{WorkspaceID: s.workspaceID(""), PrincipalID: principalID}
}

func (s *Server) chatSignal(ctx context.Context, scope agentapp.Scope, activeID, statusErr string, running bool) api.AgentChatSignal {
	transcript := []api.AgentChatTranscriptItem{}
	if activeID != "" && s.agent != nil && scope.PrincipalID != "" {
		if loaded, err := s.agent.ConversationTranscript(ctx, scope, activeID); err == nil {
			transcript = loaded
		}
	}
	return s.chatSignalWith(scope, activeID, transcript, statusErr, running)
}

func (s *Server) chatSignalWith(scope agentapp.Scope, activeID string, transcript []api.AgentChatTranscriptItem, statusErr string, running bool) api.AgentChatSignal {
	conversations := []api.AgentConversationResponse{}
	if s.agent != nil && scope.PrincipalID != "" {
		if rows, err := s.agent.ListConversations(context.Background(), scope); err == nil {
			for _, row := range rows {
				conversations = append(conversations, agentConversationDTO(row))
			}
		}
	}
	enabled := s.agent != nil && s.agent.Enabled()
	if !enabled && statusErr == "" {
		statusErr = "Agent is not configured"
	}
	return api.AgentChatSignal{
		Conversations:        conversations,
		ActiveConversationID: activeID,
		Transcript:           transcript,
		Status: api.AgentChatStatus{
			Enabled: enabled,
			Running: running,
			Error:   statusErr,
		},
		Composer: api.AgentComposerSignal{
			Value:       "",
			Disabled:    !enabled || running,
			Placeholder: chatPlaceholder(enabled, running),
		},
	}
}

func appendServerUserTranscript(transcript []api.AgentChatTranscriptItem, conversationID, input string) []api.AgentChatTranscriptItem {
	if strings.TrimSpace(input) == "" {
		return transcript
	}
	next := append([]api.AgentChatTranscriptItem{}, transcript...)
	next = append(next, api.AgentChatTranscriptItem{
		ID:             "live:user",
		Kind:           "user",
		Text:           input,
		ConversationID: conversationID,
	})
	return next
}

func applyLiveTranscriptEvent(transcript []api.AgentChatTranscriptItem, conversationID string, event api.AgentEventEnvelope) []api.AgentChatTranscriptItem {
	next := append([]api.AgentChatTranscriptItem{}, transcript...)
	switch event.Type {
	case string(agent.EventTypeMessageDelta):
		delta := stringPayload(event.Payload, "delta")
		if delta == "" {
			return next
		}
		for i := len(next) - 1; i >= 0; i-- {
			if next[i].Kind == "assistant" && next[i].Status == "streaming" && next[i].RunID == event.RunID {
				next[i].Markdown += delta
				return next
			}
		}
		return append(next, api.AgentChatTranscriptItem{
			ID:             "live:assistant:" + event.RunID,
			Kind:           "assistant",
			Markdown:       delta,
			Status:         "streaming",
			ConversationID: conversationID,
			RunID:          event.RunID,
			CreatedAt:      event.CreatedAt,
		})
	case string(agent.EventTypeToolStart):
		callID := stringPayload(event.Payload, "tool_call_id")
		name := stringPayload(event.Payload, "tool_name")
		if callID == "" {
			return next
		}
		if idx := transcriptToolIndex(next, callID); idx >= 0 {
			next[idx].Status = "running"
			return next
		}
		return append(next, api.AgentChatTranscriptItem{
			ID:             "live:tool:" + callID,
			Kind:           "tool",
			ToolCallID:     callID,
			Name:           name,
			Title:          liveToolTitle(name),
			Status:         "running",
			ConversationID: conversationID,
			RunID:          event.RunID,
			CreatedAt:      event.CreatedAt,
		})
	case string(agent.EventTypeToolEnd):
		callID := stringPayload(event.Payload, "tool_call_id")
		if callID == "" {
			return next
		}
		idx := transcriptToolIndex(next, callID)
		if idx < 0 {
			name := stringPayload(event.Payload, "tool_name")
			next = append(next, api.AgentChatTranscriptItem{
				ID:             "live:tool:" + callID,
				Kind:           "tool",
				ToolCallID:     callID,
				Name:           name,
				Title:          liveToolTitle(name),
				ConversationID: conversationID,
				RunID:          event.RunID,
				CreatedAt:      event.CreatedAt,
			})
			idx = len(next) - 1
		}
		if event.Severity == string(agent.SeverityError) || event.Severity == string(agent.SeverityWarn) {
			next[idx].Status = "error"
			next[idx].Error = "Tool failed"
			return next
		}
		next[idx].Status = "complete"
		return next
	default:
		return next
	}
}

func transcriptToolIndex(transcript []api.AgentChatTranscriptItem, callID string) int {
	for i := range transcript {
		if transcript[i].Kind == "tool" && transcript[i].ToolCallID == callID {
			return i
		}
	}
	return -1
}

func stringPayload(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, _ := payload[key].(string)
	return value
}

func liveToolTitle(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Tool"
	}
	parts := strings.Fields(strings.ReplaceAll(name, "_", " "))
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func chatPlaceholder(enabled, running bool) string {
	if !enabled {
		return "Agent is not configured"
	}
	if running {
		return "Waiting for the current answer..."
	}
	return "Ask about dashboards, metrics, or models..."
}
