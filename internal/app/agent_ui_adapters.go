package app

import (
	"net/http"

	agentmodule "github.com/Yacobolo/leapview/internal/agent/module"
	dashboardmodule "github.com/Yacobolo/leapview/internal/dashboard/module"
	"github.com/Yacobolo/leapview/internal/workspace/ui"
	uisignals "github.com/Yacobolo/leapview/internal/workspace/ui/signals"
)

func agentChromeOption(module *agentmodule.Module, r *http.Request) ui.ChromeOption {
	return ui.WithChatSidebar(workspaceChatSignal(module.ChromeSignal(r)))
}

func workspaceChatViewState(state agentmodule.ChatViewState) ui.ChatViewState {
	return ui.ChatViewState{Agent: workspaceChatSignal(state.Agent), Visuals: state.Visuals}
}

func workspaceChatSignal(state agentmodule.ChatSignal) ui.ChatSignal {
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

func dashboardAgentChrome(state agentmodule.ChatSignal) dashboardmodule.AgentChrome {
	conversations := make([]dashboardmodule.AgentConversation, 0, len(state.Conversations))
	for _, conversation := range state.Conversations {
		conversations = append(conversations, dashboardmodule.AgentConversation{
			ID:           conversation.ID,
			Title:        conversation.Title,
			TitlePending: conversation.TitlePending,
		})
	}
	return dashboardmodule.AgentChrome{
		ActiveConversationID: state.ActiveConversationID,
		Conversations:        conversations,
	}
}

func dashboardAgentBootstrap(state agentmodule.ChatViewState) dashboardmodule.AgentBootstrap {
	return dashboardmodule.AgentBootstrap{
		Agent:   dashboardChatSignal(state.Agent),
		Visuals: state.Visuals,
	}
}

func dashboardChatSignal(state agentmodule.ChatSignal) dashboardmodule.ChatSignal {
	conversations := make([]dashboardmodule.ChatConversationSummary, 0, len(state.Conversations))
	for _, conversation := range state.Conversations {
		conversations = append(conversations, dashboardmodule.ChatConversationSummary{
			ArchivedAt: conversation.ArchivedAt, CreatedAt: conversation.CreatedAt, ID: conversation.ID,
			LastMessageText: conversation.LastMessageText, MessageCount: conversation.MessageCount,
			PrincipalID: conversation.PrincipalID, Status: conversation.Status, Title: conversation.Title,
			TitlePending: conversation.TitlePending, UpdatedAt: conversation.UpdatedAt,
		})
	}
	transcript := make([]dashboardmodule.ChatTranscriptItemSignal, 0, len(state.Transcript))
	for _, item := range state.Transcript {
		var artifact *dashboardmodule.ChatArtifactSignal
		if item.Artifact != nil {
			artifact = &dashboardmodule.ChatArtifactSignal{
				ID: item.Artifact.ID, Type: item.Artifact.Type, Summary: item.Artifact.Summary,
			}
		}
		var references *[]dashboardmodule.AgentReferenceSignal
		if item.References != nil {
			converted := make([]dashboardmodule.AgentReferenceSignal, 0, len(*item.References))
			for _, reference := range *item.References {
				locations := make([]dashboardmodule.AgentReferenceLocationSignal, 0, len(reference.Locations))
				for _, location := range reference.Locations {
					locations = append(locations, dashboardmodule.AgentReferenceLocationSignal{
						DashboardID: location.DashboardID, DashboardName: location.DashboardName,
						PageID: location.PageID, PageName: location.PageName, Href: location.Href,
					})
				}
				converted = append(converted, dashboardmodule.AgentReferenceSignal{
					Reference: dashboardmodule.AgentReferenceKeySignal{
						WorkspaceID: reference.Reference.WorkspaceID, Type: reference.Reference.Type, ID: reference.Reference.ID,
					},
					Name: reference.Name, Description: reference.Description, VisualType: reference.VisualType,
					Workspace: dashboardmodule.AgentReferenceWorkspaceSignal{
						ID: reference.Workspace.ID, Name: reference.Workspace.Name,
					},
					Hierarchy: append([]string(nil), reference.Hierarchy...),
					Href:      reference.Href,
					Locations: locations,
					Context:   append([]string(nil), reference.Context...),
				})
			}
			references = &converted
		}
		transcript = append(transcript, dashboardmodule.ChatTranscriptItemSignal{
			ArgumentsJSON: item.ArgumentsJSON, Artifact: artifact, ConversationID: item.ConversationID,
			CreatedAt: item.CreatedAt, Error: item.Error, ID: item.ID, InputFormat: item.InputFormat,
			InputJSON: item.InputJSON, Kind: item.Kind, Markdown: item.Markdown, Name: item.Name,
			References: references, ResultFormat: item.ResultFormat, ResultJSON: item.ResultJSON,
			ResultSummary: item.ResultSummary, RunID: item.RunID, Status: item.Status, Summary: item.Summary,
			Text: item.Text, Title: item.Title, ToolCallID: item.ToolCallID,
		})
	}
	return dashboardmodule.ChatSignal{
		ActiveConversationID: state.ActiveConversationID,
		Conversations:        conversations,
		Transcript:           transcript,
		Status: dashboardmodule.ChatStatus{
			Enabled: state.Status.Enabled, Error: state.Status.Error, Running: state.Status.Running,
		},
		Composer: dashboardmodule.ComposerSignal{
			Disabled: state.Composer.Disabled, Placeholder: state.Composer.Placeholder, Value: state.Composer.Value,
		},
	}
}
