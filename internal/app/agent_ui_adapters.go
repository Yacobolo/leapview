package app

import (
	"net/http"

	agentmodule "github.com/flidai/leapview/internal/agent/module"
	"github.com/flidai/leapview/internal/app/brand"
	appshell "github.com/flidai/leapview/internal/app/shell"
	dashboardmodule "github.com/flidai/leapview/internal/dashboard/module"
	webpage "github.com/flidai/leapview/internal/platform/web/page"
	"github.com/flidai/leapview/internal/platform/web/staticasset"
)

func applicationLayout(routes *capabilityRoutes, assets staticasset.Resolver, r *http.Request) webpage.Provider {
	config := appshell.Config{
		Presentation: webpage.Presentation{ProductName: brand.Name, FaviconPath: brand.FaviconPath},
		Assets:       assets,
	}
	if routes != nil && routes.accessModule != nil {
		config.RoleLabel = routes.accessModule.CurrentRoleLabel(r)
	}
	if routes == nil || routes.agentModule == nil {
		return appshell.Provider(config)
	}
	state := routes.agentModule.ChromeSignal(r)
	config.ActiveConversationID = state.ActiveConversationID
	config.Conversations = make([]appshell.Conversation, 0, len(state.Conversations))
	for _, conversation := range state.Conversations {
		config.Conversations = append(config.Conversations, appshell.Conversation{
			ID: conversation.ID, Title: conversation.Title, TitlePending: conversation.TitlePending,
		})
	}
	return appshell.Provider(config)
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
