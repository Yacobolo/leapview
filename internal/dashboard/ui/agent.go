package ui

import (
	"net/url"

	uisignals "github.com/Yacobolo/leapview/internal/dashboard/ui/signals"
	visualizationir "github.com/Yacobolo/leapview/internal/dashboard/visualization/ir"
)

// AgentConversation is the dashboard shell's narrow view of an agent
// conversation. App composition adapts the agent-owned contract into it.
type AgentConversation struct {
	ID           string
	Title        string
	TitlePending *bool
}

// AgentChrome is the dashboard-owned consumer contract for agent navigation.
type AgentChrome struct {
	ActiveConversationID string
	Conversations        []AgentConversation
}

// AgentBootstrap contains the agent-owned state projected into dashboard
// browser contracts by app composition.
type AgentBootstrap struct {
	Agent   uisignals.ChatSignal
	Visuals map[string]visualizationir.VisualizationEnvelope
}

func AgentChromeDecorator(state AgentChrome) ChromeDecorator {
	return func(chrome *uisignals.ChromeSignal) {
		items := make([]uisignals.SidebarHistoryItemSignal, 0, len(state.Conversations))
		for _, conversation := range state.Conversations {
			title := conversation.Title
			if title == "" {
				title = "Conversation"
			}
			items = append(items, uisignals.SidebarHistoryItemSignal{
				ID:      conversation.ID,
				Title:   title,
				Href:    "/chats/" + url.PathEscape(conversation.ID),
				Active:  conversation.ID == state.ActiveConversationID,
				Pending: conversation.TitlePending,
			})
		}
		chrome.Sidebar.PrimaryAction = &uisignals.SidebarActionSignal{
			Label: "New chat",
			Href:  "/chats/new",
			Icon:  "plus",
		}
		chrome.Sidebar.History = &uisignals.SidebarHistorySignal{
			Label:     "Chats",
			EmptyText: uisignals.Optional("No chats yet."),
			Items:     items,
		}
	}
}
