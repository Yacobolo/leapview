package app

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/Yacobolo/libredash/internal/access"
	"github.com/Yacobolo/libredash/internal/agent"
	agenthttp "github.com/Yacobolo/libredash/internal/agent/http"
	"github.com/Yacobolo/libredash/internal/api"
	uisignals "github.com/Yacobolo/libredash/internal/ui/signals"
)

func (s *Server) agentHTTPHandler() *agenthttp.Handler {
	var settings agenthttp.Settings
	if s.store != nil {
		settings = s.store
	}
	return agenthttp.NewHandler(agenthttp.Options{
		Service:  s.agent,
		Settings: settings,
		Broker:   s.broker,
		CSRFToken: func(r *http.Request) string {
			return csrfToken(r, s.auth)
		},
		CurrentRoleLabel:       s.currentRoleLabel,
		ChatSignal:             s.chatSignal,
		ChatSignalWith:         s.chatSignalWith,
		SearchReferences:       s.searchAgentReferences,
		ResolveTurnContext:     s.resolveAgentTurnContext,
		QueueMissingTitle:      s.queueMissingChatTitle,
		ExecuteStartedChatTurn: s.executeStartedChatTurn,
		EnqueueRun: func(ctx context.Context, scope agent.Scope, started *agent.StartedPrompt) error {
			if err := s.appendAsyncEvent(ctx, "agent_run", started.RunID, "agent_run.queued", map[string]any{"runId": started.RunID, "conversationId": started.ConversationID, "status": "running"}); err != nil {
				return err
			}
			return s.enqueueAsyncJobPayload(ctx, "agent:"+started.RunID+":run", apiJobAgentRun, "agent_run", started.RunID, agentRunJob{Scope: scope, Conversation: started.ConversationID, Run: started.RunID, CorrelationID: started.CorrelationID})
		},
		CancelQueuedRun: func(ctx context.Context, scope agent.Scope, conversationID, runID string) (bool, error) {
			cancelled, err := s.cancelQueuedAsyncJob(ctx, "agent:"+runID+":run")
			if err != nil || !cancelled {
				return cancelled, err
			}
			if err := s.agent.CancelPersistedRun(ctx, scope, conversationID, runID); err != nil {
				return false, err
			}
			_ = s.appendAsyncEvent(ctx, "agent_run", runID, "agent_run.cancelled", map[string]any{"runId": runID, "conversationId": conversationID})
			return true, nil
		},
		CurrentPrincipal: func(r *http.Request) (agenthttp.Principal, bool) {
			if s.auth == nil {
				return agenthttp.Principal{}, false
			}
			principal, ok := s.auth.Principal(r)
			return agenthttp.Principal{ID: principal.ID, DevAuthBypass: principal.DevBypass}, ok
		},
		CurrentCredential: func(r *http.Request) (access.APICredential, bool) {
			if s.auth == nil {
				return access.APICredential{}, false
			}
			return s.auth.APICredential(r)
		},
	})
}

func (s *Server) searchAgentReferences(r *http.Request, workspaceID, query, dashboardID, pageID string) ([]uisignals.AgentReferenceSignal, error) {
	handler := s.workspaceHTTPHandler()
	workspaceIDs := []string{strings.TrimSpace(workspaceID)}
	global := workspaceIDs[0] == ""
	if global {
		var err error
		workspaceIDs, err = handler.VisibleWorkspaceIDs(r)
		if err != nil {
			return nil, err
		}
	}
	groups := make([][]uisignals.AgentReferenceSignal, 0, len(workspaceIDs))
	for _, currentWorkspaceID := range workspaceIDs {
		rows, err := handler.SearchResults(r, currentWorkspaceID, query, nil)
		if err != nil {
			return nil, err
		}
		sort.SliceStable(rows, func(i, j int) bool {
			return agentReferenceScopeRank(rows[i], dashboardID, pageID) < agentReferenceScopeRank(rows[j], dashboardID, pageID)
		})
		group := make([]uisignals.AgentReferenceSignal, 0, len(rows))
		for _, row := range rows {
			reference := agentReferenceSignal(currentWorkspaceID, row)
			if global {
				description := currentWorkspaceID
				if strings.TrimSpace(row.Description) != "" {
					description += " · " + row.Description
				}
				reference.Description = uisignals.Optional(description)
			}
			group = append(group, reference)
		}
		groups = append(groups, group)
	}
	out := make([]uisignals.AgentReferenceSignal, 0)
	for index := 0; ; index++ {
		added := false
		for _, group := range groups {
			if index < len(group) {
				out = append(out, group[index])
				added = true
			}
		}
		if !added {
			break
		}
	}
	return out, nil
}

func agentReferenceScopeRank(row api.SearchResult, dashboardID, pageID string) int {
	if pageID != "" && row.DashboardID == dashboardID && row.PageID == pageID {
		return 0
	}
	if dashboardID != "" && row.DashboardID == dashboardID {
		return 1
	}
	return 2
}

func agentReferenceSignal(workspaceID string, row api.SearchResult) uisignals.AgentReferenceSignal {
	return uisignals.AgentReferenceSignal{
		Kind:        row.Type,
		ID:          row.ID,
		Title:       row.Name,
		Description: uisignals.Optional(row.Description),
		WorkspaceID: workspaceID,
		ComponentID: uisignals.Optional(row.ComponentID),
		DashboardID: uisignals.Optional(row.DashboardID),
		PageID:      uisignals.Optional(row.PageID),
		VisualID:    uisignals.Optional(row.VisualID),
		TableID:     uisignals.Optional(row.TableID),
		FilterID:    uisignals.Optional(row.FilterID),
		ModelID:     uisignals.Optional(row.ModelID),
		DatasetID:   uisignals.Optional(row.DatasetID),
		FieldID:     uisignals.Optional(row.FieldID),
		AssetID:     uisignals.Optional(row.AssetID),
	}
}

func (s *Server) resolveAgentTurnContext(r *http.Request, scope agent.Scope, candidate agent.TurnContext) (agent.TurnContext, error) {
	switch strings.ToLower(strings.TrimSpace(candidate.Surface)) {
	case "dashboard":
		return s.resolveDashboardTurnContext(r.Context(), scope, candidate)
	case "chat":
		if !agentCredentialAllowsPrivilege(scope, access.PrivilegeViewItem) {
			return agent.TurnContext{}, errors.New("credential cannot view referenced context")
		}
		defaultWorkspaceID := firstNonEmpty(candidate.WorkspaceID, s.defaultWorkspaceID)
		workspaceRows := map[string]map[string]api.SearchResult{}
		for _, reference := range candidate.References {
			workspaceID := firstNonEmpty(reference.WorkspaceID, defaultWorkspaceID)
			if workspaceID == "" {
				continue
			}
			if _, loaded := workspaceRows[workspaceID]; loaded {
				continue
			}
			rows, err := s.workspaceHTTPHandler().SearchResults(r, workspaceID, "", nil)
			if err != nil {
				return agent.TurnContext{}, err
			}
			byKey := make(map[string]api.SearchResult, len(rows))
			for _, row := range rows {
				byKey[strings.ToLower(row.Type)+":"+row.ID] = row
			}
			workspaceRows[workspaceID] = byKey
		}
		resolved := make([]agent.TurnReference, 0, min(len(candidate.References), maxDashboardTurnReferences))
		seen := map[string]struct{}{}
		resolvedWorkspaceID := ""
		for _, reference := range candidate.References {
			if len(resolved) == maxDashboardTurnReferences {
				break
			}
			workspaceID := firstNonEmpty(reference.WorkspaceID, defaultWorkspaceID)
			key := workspaceID + ":" + strings.ToLower(strings.TrimSpace(reference.Kind)) + ":" + strings.TrimSpace(reference.ID)
			row, ok := workspaceRows[workspaceID][strings.ToLower(strings.TrimSpace(reference.Kind))+":"+strings.TrimSpace(reference.ID)]
			if !ok {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			resolved = append(resolved, agent.TurnReference{
				Kind: row.Type, ID: row.ID, Title: row.Name, WorkspaceID: workspaceID, ComponentID: row.ComponentID,
				DashboardID: row.DashboardID, PageID: row.PageID, VisualID: row.VisualID, TableID: row.TableID,
				FilterID: row.FilterID, ModelID: row.ModelID, DatasetID: row.DatasetID, FieldID: row.FieldID, AssetID: row.AssetID,
			})
			if len(resolved) == 1 {
				resolvedWorkspaceID = workspaceID
			} else if resolvedWorkspaceID != workspaceID {
				resolvedWorkspaceID = ""
			}
		}
		return agent.TurnContext{Surface: "chat", WorkspaceID: resolvedWorkspaceID, References: resolved}, nil
	default:
		return agent.TurnContext{}, errors.New("unsupported agent context surface")
	}
}

func (s *Server) agentSystemPrompt(ctx context.Context) (string, error) {
	return s.agentHTTPHandler().SystemPrompt(ctx)
}
