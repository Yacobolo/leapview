package module

import (
	"net/http"

	"github.com/Yacobolo/leapview/internal/agent"
	agentui "github.com/Yacobolo/leapview/internal/agent/ui"
	"github.com/Yacobolo/leapview/internal/workspace/ui"
	uisignals "github.com/Yacobolo/leapview/internal/workspace/ui/signals"
)

func (m *Module) ChromeOption(r *http.Request) ui.ChromeOption {
	return ui.WithChatSidebar(m.ChromeSignal(r))
}

func (m *Module) ChromeSignal(r *http.Request) ui.ChatSignal {
	if m == nil {
		return ui.ChatSignal{}
	}
	scope := agent.Scope{}
	if m.handler != nil {
		scope = m.handler.Scope(r)
	}
	state := m.ChatSignalWith(r.Context(), scope, "", nil, agent.ChatArtifactSignals{}, "", false).Agent
	return workspaceChatSignal(state)
}

func (m *Module) DashboardBootstrap(r *http.Request, workspaceID string) ui.ChatViewState {
	if m == nil || m.handler == nil {
		return ui.ChatViewState{}
	}
	state := m.handler.DashboardBootstrap(r, workspaceID)
	return ui.ChatViewState{
		Agent:   workspaceChatSignal(state.Agent),
		Visuals: state.Visuals,
	}
}

func workspaceChatSignal(state agentui.ChatSignal) ui.ChatSignal {
	conversations := make([]ui.ChatConversationSummary, 0, len(state.Conversations))
	for _, conversation := range state.Conversations {
		conversations = append(conversations, ui.ChatConversationSummary{
			ArchivedAt: conversation.ArchivedAt, CreatedAt: conversation.CreatedAt, ID: conversation.ID,
			LastMessageText: conversation.LastMessageText, MessageCount: conversation.MessageCount,
			PrincipalID: conversation.PrincipalID, Status: conversation.Status, Title: conversation.Title,
			TitlePending: conversation.TitlePending, UpdatedAt: conversation.UpdatedAt,
		})
	}
	transcript := make([]ui.ChatTranscriptItemSignal, 0, len(state.Transcript))
	for _, item := range state.Transcript {
		var artifact *uisignals.ChatArtifactSignal
		if item.Artifact != nil {
			artifact = &uisignals.ChatArtifactSignal{ID: item.Artifact.ID, Type: item.Artifact.Type, Summary: item.Artifact.Summary}
		}
		var references *[]uisignals.AgentReferenceSignal
		if item.References != nil {
			converted := make([]uisignals.AgentReferenceSignal, 0, len(*item.References))
			for _, reference := range *item.References {
				locations := make([]uisignals.AgentReferenceLocationSignal, 0, len(reference.Locations))
				for _, location := range reference.Locations {
					locations = append(locations, uisignals.AgentReferenceLocationSignal{
						DashboardID: location.DashboardID, DashboardName: location.DashboardName,
						PageID: location.PageID, PageName: location.PageName, Href: location.Href,
					})
				}
				converted = append(converted, uisignals.AgentReferenceSignal{
					Reference: uisignals.AgentReferenceKeySignal{
						WorkspaceID: reference.Reference.WorkspaceID, Type: reference.Reference.Type, ID: reference.Reference.ID,
					},
					Name: reference.Name, Description: reference.Description, VisualType: reference.VisualType,
					Workspace: uisignals.AgentReferenceWorkspaceSignal{ID: reference.Workspace.ID, Name: reference.Workspace.Name},
					Hierarchy: reference.Hierarchy, Href: reference.Href, Locations: locations, Context: reference.Context,
				})
			}
			references = &converted
		}
		transcript = append(transcript, ui.ChatTranscriptItemSignal{
			ArgumentsJSON: item.ArgumentsJSON, Artifact: artifact, ConversationID: item.ConversationID,
			CreatedAt: item.CreatedAt, Error: item.Error, ID: item.ID, InputFormat: item.InputFormat,
			InputJSON: item.InputJSON, Kind: item.Kind, Markdown: item.Markdown, Name: item.Name,
			References: references, ResultFormat: item.ResultFormat, ResultJSON: item.ResultJSON,
			ResultSummary: item.ResultSummary, RunID: item.RunID, Status: item.Status, Summary: item.Summary,
			Text: item.Text, Title: item.Title, ToolCallID: item.ToolCallID,
		})
	}
	return ui.ChatSignal{
		ActiveConversationID: state.ActiveConversationID,
		Conversations:        conversations,
		Transcript:           transcript,
		Status: ui.ChatStatus{
			Enabled: state.Status.Enabled, Error: state.Status.Error, Running: state.Status.Running,
		},
		Composer: ui.ComposerSignal{
			Disabled: state.Composer.Disabled, Placeholder: state.Composer.Placeholder, Value: state.Composer.Value,
		},
	}
}
